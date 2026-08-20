package rigs

import "testing"

// TestTreeModeDock: docking into a window with choose-tree open must not
// kill the tmux server. tmux 3.7b segfaults (window_tree_build NULL deref)
// when a join-pane destroys its source window while resizing a tree-mode
// pane — exactly the shape the old join-based dockOpen had (crash-verified
// live, then reproduced isolated). dockOpen now splits the TUI in-place and
// destroys no window, so the crash shape is structurally impossible; this
// test stays as the server-survival regression.
func TestTreeModeDock(t *testing.T) {
	r := New(t)
	bp := r.T("display-message", "-p", "-t", r.W2, "#{pane_id}")
	r.T("choose-tree", "-t", bp)
	sleep(300)
	r.Chk("tree open", r.T("display-message", "-p", "-t", bp, "#{pane_mode}") == "tree-mode")

	r.D("toggle", r.CL)
	sleep(800)

	_, err := r.TQ("list-sessions")
	r.Chk("server survived the dock", err == nil)
	s := r.Side()
	r.Chk("sidebar docked", s.Win == r.W2 && s.Width == 40)
	r.Chk("tree survives the dock", r.T("display-message", "-p", "-t", bp, "#{pane_mode}") == "tree-mode")
	r.Chk("no stray spacers", r.Spacers() == 0)
	r.SendKeys(bp, "q") // close the tree before the undock choreography
	sleep(300)

	// copy-mode must be left alone: it's a different (safe) code path and
	// users park scrollback there
	r.T("copy-mode", "-t", bp)
	sleep(200)
	r.D("toggle", r.CL) // undock
	sleep(600)
	r.D("toggle", r.CL) // re-dock with copy-mode open
	sleep(800)
	r.Chk("copy-mode untouched", r.T("display-message", "-p", "-t", bp, "#{pane_mode}") == "copy-mode")
	r.Chk("re-dock fine", r.Side().Width == 40)
}
