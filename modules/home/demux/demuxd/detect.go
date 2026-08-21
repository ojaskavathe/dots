package main

import (
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Agent state detection: which panes run coding agents, and whether each is
// working, blocked, or idle. Design rules from herdr's arc (research in
// thoughts/agent-detection-research.md):
//
//   - The SCREEN is the level-triggered authority. herdr shipped Claude
//     hooks as state authority and REVERTED (v0.6.7): hooks are
//     edge-triggered and incomplete — nothing fires on interrupt or prompt
//     dismissal, and late SubagentStop events revive dead states.
//   - The OSC title is the cheap fast path: Claude keeps a braille (or
//     half-circle, >=2.1.228) spinner prefix while working and "✳ " when
//     idle, and tmux hands it to us in #{pane_title}.
//   - Idle is CONFIRMED, never assumed: working -> plain idle holds for
//     idleConfirms samples (capped) so a redraw gap can't flap the state.
//     Blocked and working publish instantly. Visible idle evidence (a real
//     prompt box on screen) bypasses the hold.
//
// This is a poll — the daemon's only one — because there is no event
// source: probe-verified (2026-08-21) that tmux emits NO control-mode
// notification for pane title changes, even same-session, and %output does
// not cross sessions. The tick is armed only while agent panes exist, costs
// one list-panes batch (~1ms), and captures a pane's screen only when its
// window's activity stamp moved since the last scan.

const (
	detectTick     = 300 * time.Millisecond
	detectFastTick = 100 * time.Millisecond // while a pending-idle hold is confirming
	detectIdleTick = 2 * time.Second        // no agent panes known: discovery only
	idleConfirms   = 3
	idleCap        = 700 * time.Millisecond
	startupGrace   = 3 * time.Second
)

type agentInfo struct {
	kind         string // claude | grok | codex | ...
	state        string // "" (unknown) | working | blocked | idle
	grace        time.Time
	pendingIdle  int       // consecutive plain-idle samples held back
	pendingAt    time.Time // when the hold started
	lastActivity int64     // window_activity at the last screen scan
}

type detectState struct {
	agents map[string]*agentInfo // by pane id
	ticker *time.Ticker
	tickC  <-chan time.Time
	period time.Duration
}

// agentKind normalizes a pane_current_command to a known agent. Nix wrappers
// run as ".claude-wrapped" — strip the dressing before matching.
func agentKind(cmd string) string {
	c := strings.TrimSuffix(strings.TrimPrefix(cmd, "."), "-wrapped")
	switch c {
	case "claude", "grok", "codex", "gemini", "opencode", "aider":
		return c
	}
	return ""
}

// armDetect ensures the detection ticker exists. It ALWAYS runs — a lazy
// discovery cadence with no agents known — because its own list-panes is
// what notices agent panes the notification matrix misses entirely (a
// cross-session `split-window -d` emits NOTHING: no pane-changed, no
// layout event — probe-verified via the rig, 2026-08-21).
func (d *daemon) armDetect(w world) {
	if d.det.ticker == nil {
		if d.det.agents == nil {
			d.det.agents = map[string]*agentInfo{}
		}
		d.det.period = detectIdleTick
		d.det.ticker = time.NewTicker(d.det.period)
		d.det.tickC = d.det.ticker.C
	}
	d.retune()
}

// retune matches the tick cadence to the moment: pending-idle holds confirm
// fast, live agents poll normal, an agentless server only pays discovery.
func (d *daemon) retune() {
	want := detectIdleTick
	for _, a := range d.det.agents {
		if a.pendingIdle > 0 {
			want = detectFastTick
			break
		}
		want = detectTick
	}
	if want != d.det.period && d.det.ticker != nil {
		d.det.period = want
		d.det.ticker.Reset(want)
	}
}

// injectAgents copies detection state onto a freshly fetched world, so world
// diffs carry agent fields without detection owning the re-list.
func (d *daemon) injectAgents(w *world) {
	for i := range w.Panes {
		if a := d.det.agents[w.Panes[i].ID]; a != nil {
			w.Panes[i].Agent = a.kind
			w.Panes[i].AgentState = a.state
		}
	}
}

// detectTickRun is one detection pass. It classifies every agent pane (title
// tier always; screen tier only when the window's activity moved), applies
// the anti-flap rules, and publishes a world diff if anything changed.
func (d *daemon) detectTickRun(ctl *control, w *world) {
	lines, err := ctl.run("list-panes -a -F " +
		f("#{pane_id}", "#{pane_current_command}", "#{pane_title}", "#{window_activity}"))
	if err != nil {
		return
	}
	now := time.Now()
	seen := map[string]bool{}
	type scanReq struct {
		id    string
		a     *agentInfo
		title string
		act   int64
	}
	var scans []scanReq
	changed := false
	for _, ln := range lines {
		p := strings.Split(ln, sep)
		if len(p) != 4 {
			continue
		}
		id, cmd, title := p[0], p[1], p[2]
		act, _ := strconv.ParseInt(p[3], 10, 64)
		kind := agentKind(cmd)
		if kind == "" {
			if a := d.det.agents[id]; a != nil {
				delete(d.det.agents, id)
				changed = changed || a.state != ""
			}
			continue
		}
		seen[id] = true
		a := d.det.agents[id]
		if a == nil || a.kind != kind {
			// New agent in this pane: grace period so startup screens
			// (splash art, resumed scrollback) can't misclassify, and no
			// stale title from the previous process gets trusted.
			a = &agentInfo{kind: kind, grace: now.Add(startupGrace)}
			d.det.agents[id] = a
			changed = true // kind is publishable immediately, state isn't
			continue
		}
		if now.Before(a.grace) {
			continue
		}
		st, visible, conclusive := titleState(kind, title)
		if conclusive {
			changed = d.applyAgentState(id, a, st, visible) || changed
			continue
		}
		// herdr's skip-scan rule, exactly: only IDLE panes with an unmoved
		// activity stamp skip the screen. Working/blocked always rescan —
		// that is what notices turn ends and dismissed prompts. And on
		// quiet idle ticks nothing is re-asserted at all: publishing the
		// weak ✳-title verdict over a kept screen state made real panes
		// flap idle<->working every tick (live, 2026-08-21).
		if kind == "claude" && (a.state != "idle" || act != a.lastActivity) {
			scans = append(scans, scanReq{id: id, a: a, title: title, act: act})
		}
	}
	for id, a := range d.det.agents {
		if !seen[id] {
			delete(d.det.agents, id)
			changed = changed || a.state != ""
		}
	}
	if len(scans) > 0 {
		caps := make([]string, 0, len(scans)*2)
		for _, s := range scans {
			caps = append(caps, "capture-pane -p -t "+q(s.id), "display-message -p "+q(frameMarker))
		}
		out, err := ctl.runSeq(caps...)
		if err == nil {
			grids := make([][]string, 1, len(scans))
			for _, ln := range out {
				if ln == frameMarker {
					grids = append(grids, nil)
					continue
				}
				grids[len(grids)-1] = append(grids[len(grids)-1], ln)
			}
			for i, s := range scans {
				if i >= len(grids) {
					break
				}
				st, visible, skip := claudeScreenState(grids[i])
				s.a.lastActivity = s.act
				if skip {
					continue // viewer overlay: freeze the previous state
				}
				if st == "" {
					// Screen said nothing: fall back to the weak title
					// verdict (✳ idle) or plain idle through the hold.
					// Scans are claude-only for now.
					if ts, tv, _ := titleState("claude", s.title); ts != "" {
						st, visible = ts, tv
					} else {
						st, visible = "idle", false
					}
				}
				changed = d.applyAgentState(s.id, s.a, st, visible) || changed
			}
		}
	}
	if changed {
		// If detection knows a pane the world doesn't (a cross-session
		// `split-window -d` emits no notification at all), the cheap diff
		// can't carry it — re-list for real. Detection thereby doubles as
		// the self-heal for silently appearing panes.
		known := map[string]bool{}
		for i := range w.Panes {
			known[w.Panes[i].ID] = true
		}
		missing := false
		for id := range d.det.agents {
			if !known[id] {
				missing = true
				break
			}
		}
		w2 := *w
		if missing {
			if next, err := fetchWorld(ctl); err == nil {
				w2 = next
			}
		} else {
			w2.Panes = append([]pane(nil), w.Panes...)
			for i := range w2.Panes {
				w2.Panes[i].Agent, w2.Panes[i].AgentState = "", ""
			}
		}
		d.injectAgents(&w2)
		ops := diffWorlds(*w, w2)
		*w = w2
		if len(ops) > 0 {
			d.h.setWorld(*w, ops, false, d.tmuxSock)
		}
	}
	d.retune()
}

// applyAgentState runs the anti-flap policy and publishes the transition.
// Blocked and working land instantly; working -> PLAIN idle (no visible
// prompt-box/title evidence) must survive idleConfirms consecutive samples
// or idleCap of wall time first.
func (d *daemon) applyAgentState(id string, a *agentInfo, want string, visible bool) bool {
	if want == "idle" && !visible && a.state == "working" {
		if a.pendingIdle == 0 {
			a.pendingAt = time.Now()
		}
		a.pendingIdle++
		if a.pendingIdle < idleConfirms && time.Since(a.pendingAt) < idleCap {
			return false
		}
	}
	a.pendingIdle = 0
	if a.state == want {
		return false
	}
	log.Printf("agent %s pane=%s state=%s->%s", a.kind, id, orDash(a.state), want)
	a.state = want
	return true
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// titleState classifies from #{pane_title} alone — the zero-cost tier.
// conclusive means the screen never needs consulting; a weak verdict (claude
// "✳" idle) can still be beaten by on-screen blocked evidence.
var spinnerTitle = regexp.MustCompile(`^[\x{2800}-\x{28FF}\x{25D0}-\x{25D3}] `)

func titleState(kind, title string) (state string, visible, conclusive bool) {
	switch kind {
	case "claude":
		if spinnerTitle.MatchString(title) {
			return "working", true, true
		}
		if strings.HasPrefix(title, "✳") {
			return "idle", true, false
		}
	case "grok", "codex":
		if strings.Contains(title, "Action Required") {
			return "blocked", true, true
		}
		if spinnerTitle.MatchString(title) {
			return "working", true, true
		}
		if kind == "grok" && (title == "grok" || strings.HasSuffix(title, "- grok")) {
			return "idle", true, true
		}
		if kind == "codex" && strings.TrimSpace(title) != "" {
			return "idle", false, true // codex clears/retitles at idle
		}
	}
	return "", false, false
}

// claudeScreenState ports herdr's claude manifest (v2026.08.19.1) screen
// rules in priority order over a PLAIN capture (no -e; matchers never see
// ANSI). skip=true freezes the previous state (viewer overlays).
var (
	reLiveTurnPause = regexp.MustCompile(`^\s*[⏸⏵].*esc to interrupt(\s|·|$)`)
	reLiveTurnSpin  = regexp.MustCompile(`^\s*[*·✢✶✻✽]\s+\S.*…(\s+\(\d+[smh](\s|·)|\s*$)`)
	reBgShells      = regexp.MustCompile(`^\s*[⏸⏵].*·\s+[1-9]\d*\s+shells?\s+(·|$)`)
	rePromptLine    = regexp.MustCompile(`^\s*❯`)
	reBarePrompt    = regexp.MustCompile(`^\s*❯\s*$`)
	reYesOption     = regexp.MustCompile(`(?i)^\s*❯?\s*(1\.\s*)?yes\b`)
	reNoOption      = regexp.MustCompile(`(?i)^\s*[23]\.\s*no\b`)
	reBtw           = regexp.MustCompile(`^\s*/btw(\s|$)`)
	reEscClose      = regexp.MustCompile(`(?i)esc to close\s*$`)
)

func claudeScreenState(lines []string) (state string, visible, skip bool) {
	whole := strings.ToLower(strings.Join(lines, "\n"))

	// transcript viewer overlay: a VIEW of the conversation, not a state
	b3 := strings.ToLower(strings.Join(lastNonEmpty(lines, 3), "\n"))
	if strings.Contains(b3, "showing detailed transcript") &&
		(has(b3, "ctrl+o", "to toggle") || has(b3, "ctrl+e", "show all") ||
			has(b3, "ctrl+e", "collapse") || strings.Contains(b3, "↑↓ scroll") ||
			strings.Contains(b3, "? for shortcuts")) {
		return "", false, true
	}

	// live blocked form after the last horizontal rule
	form := strings.ToLower(strings.Join(afterLastHRule(lines), "\n"))
	if strings.Contains(form, "esc to cancel") &&
		(strings.Contains(form, "enter to confirm") ||
			(strings.Contains(form, "enter to select") && hasAny(form,
				"tab/arrow keys to navigate", "arrow keys to navigate",
				"arrows to navigate", "↑/↓ to navigate", "↑↓ to navigate"))) {
		return "blocked", true, false
	}

	// /btw overlay is a working turn in a drawer
	b5 := lastNonEmpty(lines, 5)
	if matchAny(b5, reBtw) && matchAny(b5, reEscClose) {
		return "working", true, false
	}

	// live turn: pause chip or the spinner status word
	b12 := lastNonEmpty(lines, 12)
	if matchAny(b12, reLiveTurnPause) || matchAny(b12, reLiveTurnSpin) {
		return "working", true, false
	}
	if matchAny(b5, reBgShells) {
		return "working", true, false
	}

	// the prompt box: an unadorned ❯ inside the input box is live idle
	body := promptBoxBody(lines)
	bodyLow := strings.ToLower(strings.Join(body, "\n"))
	if matchAny(body, rePromptLine) && !hasAny(bodyLow,
		"enter to select", "esc to cancel", "tab/arrow keys",
		"arrow keys to navigate", "↑/↓ to navigate") {
		return "idle", true, false
	}

	// model picker overlay: browsing, not blocked
	if has(whole, "select model", "enter to set as default", "esc to cancel") &&
		!strings.Contains(whole, "do you want to proceed?") &&
		!strings.Contains(whole, "enter to select") {
		return "", false, true
	}

	// permission prompts: question plus an actual yes/no option list
	if strings.Contains(whole, "do you want to proceed?") &&
		(matchAny(lines, reYesOption) || matchAny(lines, reNoOption)) {
		return "blocked", true, false
	}

	// legacy blockers, suppressed whenever a bare prompt is visible
	if !matchAny(lines, reBarePrompt) {
		if (strings.Contains(whole, "do you want to") && hasAny(whole, "yes", "❯")) ||
			(strings.Contains(whole, "would you like to") && hasAny(whole, "yes", "❯")) ||
			hasAny(whole, "waiting for permission", "tab to amend", "ctrl+e to explain") {
			return "blocked", false, false
		}
	}
	return "", false, false
}

// --- region helpers over a plain capture ---

func lastNonEmpty(lines []string, n int) []string {
	out := make([]string, 0, n)
	for i := len(lines) - 1; i >= 0 && len(out) < n; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			out = append(out, lines[i])
		}
	}
	// restore top-to-bottom order
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// isHRule: a line that IS a horizontal rule — leading ─ run of >=3
// (suffix text allowed). Claude's input box borders are exactly this
// (live-verified 2026-08-21).
func isHRule(s string) bool {
	t := strings.TrimSpace(s)
	n := 0
	for _, r := range t {
		if r != '─' {
			break
		}
		n++
	}
	return n >= 3
}

func afterLastHRule(lines []string) []string {
	for i := len(lines) - 1; i >= 0; i-- {
		if isHRule(lines[i]) {
			return lines[i+1:]
		}
	}
	return nil
}

// promptBoxBody: the text between the 2nd-from-last horizontal rule and the
// rule after it — claude's ─-bordered input box.
func promptBoxBody(lines []string) []string {
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
	if second == -1 {
		return nil
	}
	return lines[second+1 : last]
}

func has(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

func hasAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func matchAny(lines []string, re *regexp.Regexp) bool {
	for _, ln := range lines {
		if re.MatchString(ln) {
			return true
		}
	}
	return false
}
