package rigs

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestAgent: milestone 3 slice 1 — agent state detection end to end. A fake
// `claude` binary (a renamed sleep, so pane_current_command matches) gets
// classified from its pane title (tier 1) and screen content (tier 2), the
// states flow into the world as pane diffs, and the docked sidebar renders
// the window's worst-state glyph.
func TestAgent(t *testing.T) {
	r := New(t)

	// pane_current_command is the process image name, so the fake agent must
	// BE a binary named claude — a copied /bin/sleep gets SIGKILLed by macOS
	// (relocated platform binary), symlinks report the target's name, and
	// scripts report their interpreter. Probe-verified; build our own.
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	if err := os.WriteFile(src, []byte("package main\nimport \"time\"\nfunc main(){time.Sleep(time.Hour)}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(dir, "claude")
	if out, err := exec.Command("go", "build", "-o", fake, src).CombinedOutput(); err != nil {
		t.Fatalf("build fake claude: %v %s", err, out)
	}

	// An "agent" pane in gamma: after the 3s startup grace, a spinner title
	// must read as working — instantly, no debounce.
	// A cross-session -d split emits NO tmux notification: only the
	// detection tick's own discovery finds this pane (up to 2s) before its
	// 3s startup grace even starts.
	ap := r.T("split-window", "-d", "-P", "-F", "#{pane_id}", "-t", r.W3, fake+" 100000")
	sleep(5600) // discovery + startup grace + a tick
	r.T("select-pane", "-T", "⠧ Cooking up a thing", "-t", ap)
	r.Chk("spinner title -> working", r.WaitUntil(300, func() bool {
		return r.LogHas("agent claude pane=.* state=.*->working")
	}))

	// ✳ title is visible idle: bypasses the working->idle hold.
	r.T("select-pane", "-T", "✳ Ready for input", "-t", ap)
	r.Chk("✳ title -> idle", r.WaitUntil(150, func() bool {
		return r.LogHas("agent claude pane=.* state=working->idle")
	}))

	// Sidebar glyph: dock and find the working marker on gamma's row.
	r.T("select-pane", "-T", "⠧ Cooking again", "-t", ap)
	sleep(500)
	r.D("toggle", r.CL)
	sleep(900)
	s := r.Side()
	r.Chk("working glyph in list", r.WaitUntil(200, func() bool {
		return strings.Contains(r.Capture(s.Pane), "✻")
	}))

	// A second agent pane showing a permission prompt: screen tier says
	// blocked, and blocked outranks working in the window aggregate.
	bp := r.T("split-window", "-d", "-P", "-F", "#{pane_id}", "-t", r.W3,
		"sh -c 'printf \"  Do you want to proceed?\\n❯ 1. Yes\\n  2. No, and tell Claude what to do differently (esc)\\n\"; exec "+fake+" 100000'")
	r.Chk("permission screen -> blocked", r.WaitUntil(700, func() bool {
		return r.LogHas("agent claude pane=.* state=.*->blocked")
	}))
	r.Chk("blocked glyph outranks working", r.WaitUntil(200, func() bool {
		return strings.Contains(r.Capture(s.Pane), "!")
	}))

	// Kill the agents: states must leave the world (glyphs gone).
	r.D("toggle", r.CL)
	sleep(600)
	r.TQ("kill-pane", "-t", ap)
	r.TQ("kill-pane", "-t", bp)
	sleep(800)
	r.Chk("gamma layout intact", r.Layout(r.W3) == tail(r.LW3))
}
