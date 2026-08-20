package rigs

import "testing"

// TestRedockAdoption: undocking defers spacer releases; docking again before
// the drain fires adopts them back — a quick M-s round trip re-uses every
// carve instead of paying release + re-carve.
func TestRedockAdoption(t *testing.T) {
	r := New(t)

	r.D("toggle", r.CL) // dock on beta
	sleep(800)
	sp := r.Side().Pane
	r.SendKeys(sp, "k") // billboard w1: carves it
	sleep(500)
	r.SendKeys(sp, "Enter") // enter w1: swap — beta now holds the spacer
	r.WaitUntil(100, func() bool { return r.ClientWin() == r.W1 })
	sleep(300)
	r.Chk("beta spacer-held after enter", countLeftSpacer(r.T("list-panes", "-t", r.W2, "-F", "#{pane_left} #{pane_width}")) == 1)

	// undock and IMMEDIATELY redock — inside the release-settle window
	r.D("toggle", r.CL)
	r.D("toggle", r.CL)
	sleep(600)
	s := r.Side()
	sp = s.Pane // the TUI pane is per-dock: the redock spawned a fresh one
	r.Chk("re-docked", s.Win == r.ClientWin() && s.Width == 40)
	r.Chk("beta carve adopted, not released", countLeftSpacer(r.T("list-panes", "-t", r.W2, "-F", "#{pane_left} #{pane_width}")) == 1)

	// the adopted carve still works: entering beta is a swap
	r.T("select-pane", "-t", sp)
	r.SendKeys(sp, "j") // from w1 row down to beta
	sleep(400)
	r.SendKeys(sp, "Enter")
	r.WaitUntil(100, func() bool { return r.ClientWin() == r.W2 })
	sleep(300)
	r.Chk("entered adopted window", r.ClientWin() == r.W2)

	// final undock: everything drains, layouts byte-exact
	r.D("toggle", r.CL)
	sleep(1000)
	r.Chk("all spacers drained", r.Spacers() == 0)
	r.Chk("w1 layout exact", r.Layout(r.W1) == tail(r.LW1))
	r.Chk("beta layout exact", r.Layout(r.W2) == tail(r.LW2))
}
