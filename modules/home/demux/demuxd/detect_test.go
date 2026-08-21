package main

import "testing"

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

func TestClaudeScreenStates(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines []string
		state string
		skip  bool
	}{
		{"idle prompt box", screenIdle, "idle", false},
		// the working line outranks the (always-present) prompt box
		{"working beats prompt box", screenWorking, "working", false},
		{"permission prompt", screenBlocked, "blocked", false},
		{"transcript viewer freezes", screenViewer, "", true},
		{"empty screen says nothing", []string{"", ""}, "", false},
	} {
		st, _, skip := claudeScreenState(tc.lines)
		if st != tc.state || skip != tc.skip {
			t.Errorf("%s: got state=%q skip=%v want %q/%v", tc.name, st, skip, tc.state, tc.skip)
		}
	}
}

func TestTitleStates(t *testing.T) {
	for _, tc := range []struct {
		kind, title, state string
		conclusive         bool
	}{
		{"claude", "⠂ Build herdr-like tool", "working", true},
		{"claude", "◐ Reviewing changes", "working", true}, // 2.1.228 half-circle spinner
		{"claude", "✳ Design modular autopilot", "idle", false},
		{"claude", "…/some/path", "", false},
		{"grok", "Action Required - grok", "blocked", true},
		{"grok", "Build Codebase Command - grok", "idle", true},
		{"codex", "anything at all", "idle", true},
	} {
		st, _, con := titleState(tc.kind, tc.title)
		if st != tc.state || con != tc.conclusive {
			t.Errorf("%s %q: got %q/%v want %q/%v", tc.kind, tc.title, st, con, tc.state, tc.conclusive)
		}
	}
}

func TestPromptBoxRegion(t *testing.T) {
	body := promptBoxBody(screenIdle)
	if len(body) != 1 || !rePromptLine.MatchString(body[0]) {
		t.Fatalf("promptBoxBody = %q", body)
	}
	if got := afterLastHRule(screenIdle); len(got) != 1 || got[0] != "  Opus 4.8 · iv-pro-copilot-v45" {
		t.Fatalf("afterLastHRule = %q", got)
	}
}

func TestAgentKindNormalization(t *testing.T) {
	for cmd, want := range map[string]string{
		".claude-wrapped": "claude", // nix wrapper argv0, live-verified
		"claude":          "claude",
		"grok":            "grok",
		"zsh":             "",
		"node":            "",
	} {
		if got := agentKind(cmd); got != want {
			t.Errorf("agentKind(%q) = %q want %q", cmd, got, want)
		}
	}
}

// The anti-flap hold: working -> plain idle needs idleConfirms samples;
// visible idle evidence and blocked bypass it entirely.
func TestIdleHold(t *testing.T) {
	d := &daemon{}
	a := &agentInfo{kind: "claude", state: "working"}
	if d.applyAgentState("%1", a, "idle", false) {
		t.Fatal("first plain-idle sample published")
	}
	if d.applyAgentState("%1", a, "idle", false) {
		t.Fatal("second plain-idle sample published")
	}
	if !d.applyAgentState("%1", a, "idle", false) {
		t.Fatal("third plain-idle sample held")
	}
	a = &agentInfo{kind: "claude", state: "working"}
	if !d.applyAgentState("%1", a, "idle", true) {
		t.Fatal("visible idle should bypass the hold")
	}
	a = &agentInfo{kind: "claude", state: "working"}
	if !d.applyAgentState("%1", a, "blocked", true) {
		t.Fatal("blocked must publish instantly")
	}
	// an interleaved working sample clears the pending hold
	a = &agentInfo{kind: "claude", state: "working"}
	d.applyAgentState("%1", a, "idle", false)
	d.applyAgentState("%1", a, "working", true)
	if d.applyAgentState("%1", a, "idle", false) {
		t.Fatal("hold did not restart after working interrupted it")
	}
}
