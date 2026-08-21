package rigs

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestAgent: milestone 3 end to end. A fake `claude` binary (built, since
// pane_current_command is the image name and macOS kills relocated platform
// binaries) is classified from its title (tier 1) and screen (tier 2, TOML
// manifest engine), states flow as pane diffs, completions in unwatched
// windows become "done" until visited, blocked notifies other clients, the
// sidebar renders glyphs plus the agents section, and @demux_agents keeps
// the statusline counts.
func TestAgent(t *testing.T) {
	r := New(t)

	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	if err := os.WriteFile(src, []byte("package main\nimport \"time\"\nfunc main(){time.Sleep(time.Hour)}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(dir, "claude")
	if out, err := exec.Command("go", "build", "-o", fake, src).CombinedOutput(); err != nil {
		t.Fatalf("build fake claude: %v %s", err, out)
	}

	// A cross-session -d split emits NO tmux notification: only the
	// detection tick's own discovery finds this pane (up to 2s) before its
	// 3s startup grace even starts.
	ap := r.T("split-window", "-d", "-P", "-F", "#{pane_id}", "-t", r.W3, fake+" 100000")
	sleep(5600) // discovery + startup grace + a tick
	r.T("select-pane", "-T", "⠧ Cooking up a thing", "-t", ap)
	r.Chk("spinner title -> working", r.WaitUntil(300, func() bool {
		return r.LogHas("agent claude pane=.* state=.*->working")
	}))
	r.Chk("statusline counts working", strings.Contains(r.ShowOpt("-gqv", "@demux_agents"), "✻"))

	// ✳ title = idle — but the client is looking at beta, not gamma, so
	// the completion lands as DONE and sticks until gamma is visited.
	r.T("select-pane", "-T", "✳ Ready for input", "-t", ap)
	r.Chk("unwatched completion -> done", r.WaitUntil(300, func() bool {
		return r.LogHas("agent claude pane=.* state=working->done")
	}))
	r.T("select-window", "-t", r.W3)
	r.Chk("visiting clears done", r.WaitUntil(300, func() bool {
		return r.LogHas(`state=done->idle \(seen\)`)
	}))
	r.T("select-window", "-t", r.W2)

	// Sidebar: working glyph on gamma's row plus the agents section.
	r.T("select-pane", "-T", "⠧ Cooking again", "-t", ap)
	sleep(500)
	r.D("toggle", r.CL)
	sleep(900)
	s := r.Side()
	r.Chk("working glyph in list", r.WaitUntil(200, func() bool {
		return strings.Contains(r.Capture(s.Pane), "✻")
	}))
	r.Chk("agents section listed", r.WaitUntil(200, func() bool {
		cap := r.Capture(s.Pane)
		return strings.Contains(cap, "agents") && strings.Contains(cap, "Cooking again")
	}))

	// A second agent pane showing a permission prompt: screen tier says
	// blocked, blocked outranks working in the aggregate, and clients not
	// looking at gamma get notified.
	bp := r.T("split-window", "-d", "-P", "-F", "#{pane_id}", "-t", r.W3,
		"sh -c 'printf \"  Do you want to proceed?\\n❯ 1. Yes\\n  2. No, and tell Claude what to do differently (esc)\\n\"; exec "+fake+" 100000'")
	r.Chk("permission screen -> blocked", r.WaitUntil(700, func() bool {
		return r.LogHas("agent claude pane=.* state=.*->blocked")
	}))
	r.Chk("blocked notification sent", r.LogHas("notify blocked"))
	r.Chk("blocked glyph outranks working", r.WaitUntil(200, func() bool {
		return strings.Contains(r.Capture(s.Pane), "!")
	}))
	r.Chk("statusline counts blocked", strings.Contains(r.ShowOpt("-gqv", "@demux_agents"), "!"))

	// Kill the agents: states must leave the world (glyphs gone).
	r.D("toggle", r.CL)
	sleep(600)
	r.TQ("kill-pane", "-t", ap)
	r.TQ("kill-pane", "-t", bp)
	sleep(800)
	r.Chk("gamma layout intact", r.Layout(r.W3) == tail(r.LW3))
	r.Chk("statusline cleared", r.WaitUntil(200, func() bool {
		return r.ShowOpt("-gqv", "@demux_agents") == ""
	}))
}
