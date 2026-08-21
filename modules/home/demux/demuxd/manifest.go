package main

import (
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// The agent-detection rule engine: declarative per-agent TOML manifests
// (schema ported from herdr's, Apache 2.0 — see
// thoughts/agent-detection-research.md) compiled into matchers evaluated
// over a screen+title snapshot. Manifests ship as DATA: the bundled ones
// are embedded, and a user file at <config>/demux/agents/<id>.toml
// replaces the bundled manifest of the same id — when an agent's UI copy
// changes, the fix is a user-editable TOML, not a release.
//
// Rule semantics: highest priority match wins, ties go to the earlier rule
// in the file; `skip_state_update` rules freeze the previous state (viewer
// overlays are a VIEW, not a state); no match at all falls back to idle
// for a known agent. `contains` is case-insensitive substring (all must
// hit), `regex` runs against the joined region text, `line_regex` must
// match at least one line; `all`/`any`/`not` nest recursively.

//go:embed manifests/*.toml
var bundledManifests embed.FS

type manifestTOML struct {
	ID      string     `toml:"id"`
	Version string     `toml:"version"`
	Aliases []string   `toml:"aliases"`
	Rules   []ruleTOML `toml:"rules"`
}

type ruleTOML struct {
	ID             string `toml:"id"`
	State          string `toml:"state"`
	Priority       int    `toml:"priority"`
	Region         string `toml:"region"`
	VisibleIdle    bool   `toml:"visible_idle"`
	VisibleBlocker bool   `toml:"visible_blocker"`
	VisibleWorking bool   `toml:"visible_working"`
	Skip           bool   `toml:"skip_state_update"`
	gateTOML
}

type gateTOML struct {
	Contains  []string   `toml:"contains"`
	Regex     []string   `toml:"regex"`
	LineRegex []string   `toml:"line_regex"`
	All       []gateTOML `toml:"all"`
	Any       []gateTOML `toml:"any"`
	Not       []gateTOML `toml:"not"`
}

type cManifest struct {
	id      string
	version string
	rules   []cRule
	// maxScreenPrio: the highest priority among non-title rules. A title
	// verdict that outranks it is CONCLUSIVE without a capture — the cheap
	// tier never needs the screen for that pane this tick.
	maxScreenPrio int
}

type cRule struct {
	id      string
	state   string // "" = unknown
	prio    int
	region  string
	visible bool // any visible_* bit: live UI chrome actually on screen
	skip    bool
	gate    cGate
}

type cGate struct {
	contains  []string // lowercased
	regex     []*regexp.Regexp
	lineRegex []*regexp.Regexp
	all       []cGate
	any       []cGate
	not       []cGate
}

// verdict is one manifest evaluation outcome.
type verdict struct {
	state   string
	skip    bool
	visible bool
	prio    int
	rule    string
}

// loadManifests reads bundled manifests, then user overrides. Returns the
// engine map (normalized kind -> manifest) plus alias resolution.
func loadManifests() map[string]*cManifest {
	out := map[string]*cManifest{}
	byID := map[string]*cManifest{}
	entries, _ := bundledManifests.ReadDir("manifests")
	for _, e := range entries {
		b, err := bundledManifests.ReadFile("manifests/" + e.Name())
		if err != nil {
			continue
		}
		m, err := compileManifest(b)
		if err != nil {
			log.Printf("manifest %s: %v", e.Name(), err)
			continue
		}
		byID[m.id] = m
	}
	// User overrides: <config>/demux/agents/<id>.toml replaces the bundled
	// manifest wholesale. XDG_CONFIG_HOME is honored explicitly — Go's
	// UserConfigDir ignores it on darwin, and the override dir must behave
	// the same on every platform demux ships to.
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		if d, err := os.UserConfigDir(); err == nil {
			dir = d
		}
	}
	if dir != "" {
		files, _ := filepath.Glob(filepath.Join(dir, "demux", "agents", "*.toml"))
		for _, fp := range files {
			b, err := os.ReadFile(fp)
			if err != nil {
				continue
			}
			m, err := compileManifest(b)
			if err != nil {
				log.Printf("manifest override %s: %v", fp, err)
				continue
			}
			log.Printf("manifest override %s (%s %s)", m.id, fp, m.version)
			byID[m.id] = m
		}
	}
	for _, m := range byID {
		out[m.id] = m
	}
	for _, m := range byID {
		for _, a := range manifestAliases[m.id] {
			out[a] = m
		}
	}
	return out
}

// manifestAliases: normalized command names that map to a manifest id.
var manifestAliases = map[string][]string{
	"claude": {"claude-code"},
}

func compileManifest(b []byte) (*cManifest, error) {
	var mt manifestTOML
	if err := toml.Unmarshal(b, &mt); err != nil {
		return nil, err
	}
	if mt.ID == "" || len(mt.Rules) == 0 || len(mt.Rules) > 128 {
		return nil, fmt.Errorf("manifest needs an id and 1..128 rules")
	}
	m := &cManifest{id: mt.ID, version: mt.Version}
	for i := range mt.Rules {
		rt := &mt.Rules[i]
		g, err := compileGate(&rt.gateTOML, 0)
		if err != nil {
			return nil, fmt.Errorf("rule %s: %w", rt.ID, err)
		}
		switch rt.State {
		case "", "idle", "working", "blocked", "unknown":
		default:
			return nil, fmt.Errorf("rule %s: bad state %q", rt.ID, rt.State)
		}
		st := rt.State
		if st == "unknown" {
			st = ""
		}
		region := rt.Region
		if region == "" {
			region = "whole_recent"
		}
		if err := checkRegion(region); err != nil {
			return nil, fmt.Errorf("rule %s: %w", rt.ID, err)
		}
		m.rules = append(m.rules, cRule{
			id: rt.ID, state: st, prio: rt.Priority, region: region,
			visible: rt.VisibleIdle || rt.VisibleBlocker || rt.VisibleWorking,
			skip:    rt.Skip, gate: g,
		})
		if region != "osc_title" && rt.Priority > m.maxScreenPrio {
			m.maxScreenPrio = rt.Priority
		}
	}
	return m, nil
}

func compileGate(gt *gateTOML, depth int) (cGate, error) {
	var g cGate
	if depth > 8 {
		return g, fmt.Errorf("gate nesting too deep")
	}
	for _, c := range gt.Contains {
		g.contains = append(g.contains, strings.ToLower(c))
	}
	for _, r := range gt.Regex {
		re, err := regexp.Compile(r)
		if err != nil {
			return g, err
		}
		g.regex = append(g.regex, re)
	}
	for _, r := range gt.LineRegex {
		re, err := regexp.Compile(r)
		if err != nil {
			return g, err
		}
		g.lineRegex = append(g.lineRegex, re)
	}
	for i := range gt.All {
		c, err := compileGate(&gt.All[i], depth+1)
		if err != nil {
			return g, err
		}
		g.all = append(g.all, c)
	}
	for i := range gt.Any {
		c, err := compileGate(&gt.Any[i], depth+1)
		if err != nil {
			return g, err
		}
		g.any = append(g.any, c)
	}
	for i := range gt.Not {
		c, err := compileGate(&gt.Not[i], depth+1)
		if err != nil {
			return g, err
		}
		g.not = append(g.not, c)
	}
	return g, nil
}

// snapshot is one pane's detection input: the visible screen (plain text,
// no ANSI) and the retained title. Region slices are memoized — several
// rules share a region within one evaluation.
type snapshot struct {
	screen []string
	title  string

	regions map[string][]string // region name -> lines
	joined  map[string]string   // region name -> joined text
	lowered map[string]string   // region name -> lowercased joined text
}

func newSnapshot(screen []string, title string) *snapshot {
	return &snapshot{screen: screen, title: title,
		regions: map[string][]string{}, joined: map[string]string{}, lowered: map[string]string{}}
}

var regionArg = regexp.MustCompile(`^([a-z_]+)\((\d+)\)$`)

func checkRegion(name string) error {
	base := name
	if m := regionArg.FindStringSubmatch(name); m != nil {
		base = m[1] + "(n)"
	}
	switch base {
	case "whole_recent", "osc_title", "after_last_horizontal_rule",
		"prompt_box_body", "above_prompt_box", "last_non_empty_above_prompt_box",
		"bottom_lines(n)", "bottom_non_empty_lines(n)", "top_non_empty_lines(n)":
		return nil
	}
	return fmt.Errorf("unknown region %q", name)
}

func (s *snapshot) region(name string) []string {
	if r, ok := s.regions[name]; ok {
		return r
	}
	var out []string
	base, n := name, 0
	if m := regionArg.FindStringSubmatch(name); m != nil {
		base = m[1]
		n, _ = strconv.Atoi(m[2])
	}
	switch base {
	case "osc_title":
		out = []string{s.title}
	case "whole_recent":
		out = s.screen
	case "bottom_lines":
		if len(s.screen) > n {
			out = s.screen[len(s.screen)-n:]
		} else {
			out = s.screen
		}
	case "bottom_non_empty_lines":
		out = lastNonEmpty(s.screen, n)
	case "top_non_empty_lines":
		// start through the Nth non-empty line
		seen := 0
		for i, ln := range s.screen {
			if strings.TrimSpace(ln) != "" {
				seen++
				if seen == n {
					out = s.screen[:i+1]
					break
				}
			}
		}
		if out == nil {
			out = s.screen
		}
	case "after_last_horizontal_rule":
		out = afterLastHRule(s.screen)
	case "prompt_box_body":
		out = promptBoxBody(s.screen)
	case "above_prompt_box":
		out = abovePromptBox(s.screen)
	case "last_non_empty_above_promptbox", "last_non_empty_above_prompt_box":
		out = lastNonEmpty(abovePromptBox(s.screen), 1)
	}
	s.regions[name] = out
	return out
}

func (s *snapshot) text(name string) string {
	if t, ok := s.joined[name]; ok {
		return t
	}
	t := strings.Join(s.region(name), "\n")
	s.joined[name] = t
	return t
}

func (s *snapshot) lower(name string) string {
	if t, ok := s.lowered[name]; ok {
		return t
	}
	t := strings.ToLower(s.text(name))
	s.lowered[name] = t
	return t
}

// eval runs every rule over the snapshot; highest priority match wins,
// ties keep the earlier rule. titleOnly restricts to osc_title rules (the
// zero-cost tier — no screen captured yet).
func (m *cManifest) eval(s *snapshot, titleOnly bool) (verdict, bool) {
	best := verdict{prio: -1 << 30}
	found := false
	for i := range m.rules {
		r := &m.rules[i]
		if titleOnly && r.region != "osc_title" {
			continue
		}
		if r.prio <= best.prio && found {
			continue
		}
		if !gateMatches(&r.gate, s, r.region) {
			continue
		}
		best = verdict{state: r.state, skip: r.skip, visible: r.visible, prio: r.prio, rule: r.id}
		found = true
	}
	return best, found
}

func gateMatches(g *cGate, s *snapshot, region string) bool {
	lower := s.lower(region)
	for _, c := range g.contains {
		if !strings.Contains(lower, c) {
			return false
		}
	}
	if len(g.regex) > 0 {
		text := s.text(region)
		for _, re := range g.regex {
			if !re.MatchString(text) {
				return false
			}
		}
	}
	if len(g.lineRegex) > 0 {
		lines := s.region(region)
		for _, re := range g.lineRegex {
			ok := false
			for _, ln := range lines {
				if re.MatchString(ln) {
					ok = true
					break
				}
			}
			if !ok {
				return false
			}
		}
	}
	for i := range g.all {
		if !gateMatches(&g.all[i], s, region) {
			return false
		}
	}
	if len(g.any) > 0 {
		ok := false
		for i := range g.any {
			if gateMatches(&g.any[i], s, region) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	for i := range g.not {
		if gateMatches(&g.not[i], s, region) {
			return false
		}
	}
	return true
}

// abovePromptBox: everything above claude's ─-bordered input box.
func abovePromptBox(lines []string) []string {
	last, second := -1, -1
	for i := len(lines) - 1; i >= 0; i-- {
		if isHRule(lines[i]) {
			if last == -1 {
				last = i
			} else {
				second = i
				break
			}
		}
	}
	if second <= 0 {
		return nil
	}
	return lines[:second]
}
