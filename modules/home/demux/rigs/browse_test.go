package rigs

import (
	"strings"
	"testing"
)

// TestBrowse: `demuxd browse` is dock + zoom — the sidebar docks into the
// client's current window and opens already scrubbing, full width. No parked
// session exists anywhere (the _demux holdout is gone).
func TestBrowse(t *testing.T) {
	r := New(t)

	// K: browse from scratch: docked into the current window, zoomed
	origin := r.ClientWin()
	r.D("browse", r.CL)
	sleep(1000)
	_, err := r.TQ("has-session", "-t", "_demux")
	r.Chk("no _demux session", err != nil)
	s := r.Side()
	r.Chk("sidebar docked in the current window", s.Win == origin)
	r.Chk("client never moved", r.ClientWin() == origin)
	r.Chk("browse opens zoomed", s.Width == 200 && r.Zoomed(origin))
	r.Chk("wide mode has border", strings.Contains(r.Capture(s.Pane), "│"))

	// scrub to w1; Enter commits for real and stays docked
	r.SendKeys(s.Pane, "k")
	sleep(700)
	r.Chk("billboard shows w1 content", strings.Contains(r.Capture(s.Pane), "MARKW1"))
	r.SendKeys(s.Pane, "Enter")
	r.WaitUntil(100, func() bool { return r.ClientWin() == r.W1 })
	sleep(400)
	s = r.Side()
	r.Chk("enter commits for real", r.ClientWin() == r.W1)
	r.Chk("sidebar stays docked at 40", s.Win == r.W1 && s.Width == 40)

	// L: browse while already docked just zooms in place
	r.D("browse", r.CL)
	sleep(600)
	s = r.Side()
	r.Chk("browse from docked zooms", s.Width == 200 && r.Zoomed(r.W1))
	r.SendKeys(s.Pane, "q")
	sleep(500)
	s = r.Side()
	r.Chk("q unzooms, still docked", s.Win == r.W1 && s.Width == 40)

	// undock: the TUI pane dies, everything restores
	r.D("toggle", r.CL)
	sleep(1200)
	r.Chk("TUI pane gone", r.Side().Pane == "")
	r.Chk("no spacers remain", r.Spacers() == 0)
	r.Chk("w1 layout exact", r.Layout(r.W1) == tail(r.LW1))
}
