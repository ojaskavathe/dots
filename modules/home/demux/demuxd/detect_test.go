package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Fixtures mirror REAL claude chrome captured live 2026-08-21 (plain
// capture-pane, no -e): the input box is bounded by ─ runs, the working
// status line is "✻ Churning… (7m 22s · ↓ 24.7k tokens)".

var screenIdle = []string{
	"  explicit next step?",
	"✻ Crunched for 2m 13s",
	"────────────────────────────────",
	"❯ do the spike first, add it to the doc",
	"────────────────────────────────",
	"  Opus 4.8 · iv-pro-copilot-v45",
}

var screenWorking = []string{
	"✻ Churning… (7m 22s · ↓ 24.7k tokens)",
	"────────────────────────────────",
	"❯ ",
	"────────────────────────────────",
	"  Opus 4.8 · iv-pro-copilot-v45",
}

var screenBlocked = []string{
	"  Bash command",
	"  rm -rf ./build",
	"  Do you want to proceed?",
	"❯ 1. Yes",
	"  2. No, and tell Claude what to do differently (esc)",
}

var screenViewer = []string{
	"  some transcript content",
	"  Showing detailed transcript",
	"  ctrl+o to toggle · ↑↓ scroll",
}

// The turn ended but a background shell lives: working, and steady even
// when the footer chip truncates (the still-running line carries it).
var screenBgShell = []string{
	"✻ Crunched for 3m 10s · 1 shell still running",
	"────────────────────────────────",
	"❯ prune those two local branches",
	"────────────────────────────────",
	"  Opus 4.8 · iv-pro-copilot-v45",
}

func claudeManifest(t *testing.T) *cManifest {
	t.Helper()
	m := loadManifests()["claude"]
	if m == nil {
		t.Fatal("bundled claude manifest missing")
	}
	return m
}

func TestBundledManifestsCompile(t *testing.T) {
	ms := loadManifests()
	for _, id := range []string{"claude", "codex", "gemini", "grok", "opencode", "claude-code"} {
		if ms[id] == nil {
			t.Errorf("manifest %s missing", id)
		}
	}
}

func TestClaudeScreenStates(t *testing.T) {
	m := claudeManifest(t)
	for _, tc := range []struct {
		name  string
		lines []string
		title string
		state string
		skip  bool
	}{
		{"idle prompt box", screenIdle, "", "idle", false},
		// the working line outranks the (always-present) prompt box
		{"working beats prompt box", screenWorking, "", "working", false},
		{"permission prompt", screenBlocked, "", "blocked", false},
		{"transcript viewer freezes", screenViewer, "", "", true},
		{"bg shell still working", screenBgShell, "", "working", false},
		// blocked screen evidence outranks the weak ✳-idle title
		{"blocked beats idle title", screenBlocked, "✳ some task", "blocked", false},
	} {
		v, ok := m.eval(newSnapshot(tc.lines, tc.title), false)
		if !ok && tc.state != "" {
			t.Errorf("%s: no rule matched", tc.name)
			continue
		}
		if ok && (v.state != tc.state || v.skip != tc.skip) {
			t.Errorf("%s: got state=%q skip=%v (rule %s) want %q/%v", tc.name, v.state, v.skip, v.rule, tc.state, tc.skip)
		}
	}
}

func TestTitleTier(t *testing.T) {
	m := claudeManifest(t)
	// spinner title outranks every screen rule: conclusive without capture
	v, ok := m.eval(newSnapshot(nil, "⠂ Build herdr-like tool"), true)
	if !ok || v.state != "working" || v.prio <= m.maxScreenPrio {
		t.Fatalf("spinner title not conclusive working: %+v max=%d", v, m.maxScreenPrio)
	}
	v, ok = m.eval(newSnapshot(nil, "◐ Reviewing changes"), true)
	if !ok || v.state != "working" {
		t.Fatalf("half-circle spinner (2.1.228) missed: %+v", v)
	}
	// ✳ idle is a WEAK verdict: matches, but never outranks the screen
	v, ok = m.eval(newSnapshot(nil, "✳ Design modular autopilot"), true)
	if !ok || v.state != "idle" || v.prio > m.maxScreenPrio {
		t.Fatalf("✳ title should be weak idle: %+v max=%d", v, m.maxScreenPrio)
	}
	if _, ok := m.eval(newSnapshot(nil, "…/some/path"), true); ok {
		t.Fatal("plain path title matched a claude title rule")
	}

	g := loadManifests()["grok"]
	v, ok = g.eval(newSnapshot(nil, "Action Required - grok"), true)
	if !ok || v.state != "blocked" {
		t.Fatalf("grok Action Required: %+v", v)
	}
	v, ok = g.eval(newSnapshot(nil, "Build Codebase Command - grok"), true)
	if !ok || v.state != "idle" {
		t.Fatalf("grok idle title: %+v", v)
	}
}

func TestManifestOverride(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "demux", "agents")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	ov := `
id = "claude"
version = "9999.custom"
[[rules]]
id = "everything_is_blocked"
state = "blocked"
priority = 10
regex = ['(?s).*']
`
	if err := os.WriteFile(filepath.Join(sub, "claude.toml"), []byte(ov), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)
	m := loadManifests()["claude"]
	if m == nil || m.version != "9999.custom" {
		t.Fatalf("override not loaded: %+v", m)
	}
	if v, ok := m.eval(newSnapshot([]string{"anything"}, ""), false); !ok || v.state != "blocked" {
		t.Fatalf("override rules not in effect: %+v", v)
	}
}

func TestAgentKindNormalization(t *testing.T) {
	d := &daemon{}
	d.det.manifests = loadManifests()
	for cmd, want := range map[string]string{
		".claude-wrapped": "claude", // nix wrapper argv0, live-verified
		"claude":          "claude",
		"grok":            "grok",
		"codex":           "codex",
		"zsh":             "",
		"node":            "", // wrapper: resolved via process walk, not name
	} {
		if got := d.agentKind(cmd); got != want {
			t.Errorf("agentKind(%q) = %q want %q", cmd, got, want)
		}
	}
}

func TestPromptBoxRegion(t *testing.T) {
	s := newSnapshot(screenIdle, "")
	body := s.region("prompt_box_body")
	if len(body) != 1 || body[0] != "❯ do the spike first, add it to the doc" {
		t.Fatalf("prompt_box_body = %q", body)
	}
	if got := s.region("after_last_horizontal_rule"); len(got) != 1 || got[0] != "  Opus 4.8 · iv-pro-copilot-v45" {
		t.Fatalf("after_last_horizontal_rule = %q", got)
	}
	if got := s.region("bottom_non_empty_lines(2)"); len(got) != 2 {
		t.Fatalf("bottom_non_empty_lines(2) = %q", got)
	}
}

// The anti-flap hold: working -> idle needs idleConfirms consecutive
// samples — visible evidence included (alternating screens flap through
// any bypass). Blocked publishes instantly. Completions in an unwatched
// window become "done" and stick.
func TestIdleHoldAndDone(t *testing.T) {
	d := &daemon{}
	novis := map[string]bool{}
	vis := map[string]bool{"@1": true}

	a := &agentInfo{kind: "claude", state: "working", win: "@1"}
	if d.applyAgentState("%1", a, "idle", true, vis) {
		t.Fatal("first idle sample published")
	}
	if d.applyAgentState("%1", a, "idle", true, vis) {
		t.Fatal("second idle sample published")
	}
	if !d.applyAgentState("%1", a, "idle", true, vis) {
		t.Fatal("third idle sample held")
	}
	if a.state != "idle" {
		t.Fatalf("visible-window completion should be idle, got %s", a.state)
	}

	// completion in an unwatched window -> done, and later idle samples
	// must not clear the flag
	a = &agentInfo{kind: "claude", state: "working", win: "@2"}
	d.applyAgentState("%2", a, "idle", false, novis)
	d.applyAgentState("%2", a, "idle", false, novis)
	if !d.applyAgentState("%2", a, "idle", false, novis) || a.state != "done" {
		t.Fatalf("unwatched completion should be done, got %s", a.state)
	}
	if d.applyAgentState("%2", a, "idle", false, novis) || a.state != "done" {
		t.Fatal("idle sample cleared the done flag")
	}

	// blocked publishes instantly, from anywhere
	a = &agentInfo{kind: "claude", state: "working", win: "@1"}
	if !d.applyAgentState("%3", a, "blocked", true, vis) {
		t.Fatal("blocked must publish instantly")
	}
	// blocked -> idle unwatched is also a completion -> done, no hold
	if !d.applyAgentState("%3", a, "idle", false, novis) || a.state != "done" {
		t.Fatalf("blocked completion should be done instantly, got %s", a.state)
	}

	// an interleaved working sample clears the pending hold
	a = &agentInfo{kind: "claude", state: "working", win: "@1"}
	d.applyAgentState("%4", a, "idle", false, vis)
	d.applyAgentState("%4", a, "working", true, vis)
	if d.applyAgentState("%4", a, "idle", false, vis) {
		t.Fatal("hold did not restart after working interrupted it")
	}
}

func TestAgentTaskTitle(t *testing.T) {
	for in, want := range map[string]string{
		"⠂ Build herdr-like tool": "Build herdr-like tool",
		"✳ Convert async tests":   "Convert async tests",
		"◐ Reviewing":             "Reviewing",
		"plain title":             "plain title",
	} {
		if got := agentTaskTitle(in); got != want {
			t.Errorf("agentTaskTitle(%q) = %q want %q", in, got, want)
		}
	}
}

// The agents region is PINNED: whatever the tree length and selection,
// every agent row (up to the region cap) has a screen row, and rowAt
// round-trips the geometry paint uses.
func TestListLayoutPinsAgents(t *testing.T) {
	rows := make([]row, 0, 43)
	for i := 0; i < 40; i++ {
		rows = append(rows, row{label: "tree", window: "@1"})
	}
	for i := 0; i < 3; i++ {
		rows = append(rows, row{label: "agent", window: "@2", arow: true})
	}
	for _, sel := range []int{0, 20, 39, 40, 42} {
		lay := layoutList(rows, sel, 20)
		if lay.sepY == -1 || lay.agentH != 3 {
			t.Fatalf("sel=%d: agents not pinned: %+v", sel, lay)
		}
		seen := map[int]bool{}
		for y := 0; y < 20; y++ {
			if i := lay.rowAt(y, len(rows)); i >= 0 {
				if seen[i] {
					t.Fatalf("sel=%d: row %d painted twice", sel, i)
				}
				seen[i] = true
			}
		}
		for a := 40; a < 43; a++ {
			if !seen[a] {
				t.Fatalf("sel=%d: agent row %d has no screen row", sel, a)
			}
		}
		if !seen[sel] {
			t.Fatalf("sel=%d: selected row not on screen", sel)
		}
	}
	// no agents: single region, full height
	if lay := layoutList(rows[:40], 5, 20); lay.sepY != -1 || lay.treeH != 20 {
		t.Fatalf("no-agent layout wrong: %+v", lay)
	}
}
