package rigs

import "testing"

// TestStickySelection: the sidebar selection must be anchored to the row's
// window, not its index. World churn mid-scrub (a window created in another
// session — agents do this constantly) shifts every row below it; an
// index-anchored highlight jumps to whatever slid into its slot and Enter
// then commits the wrong window.
func TestStickySelection(t *testing.T) {
	r := New(t)

	r.D("toggle", r.CL) // dock on beta
	sleep(800)
	sp := r.Side().Pane
	r.SendKeys(sp, "j") // scrub to gamma
	sleep(700)
	r.Chk("zoomed", r.Side().Width == 200)

	// a window appears in play (sorted above work): all work rows shift
	r.T("new-window", "-d", "-t", "play:", "-n", "churn")
	sleep(800) // relist + diff reach the TUI

	r.SendKeys(sp, "Enter")
	r.WaitUntil(100, func() bool { return r.ClientWin() == r.W3 })
	sleep(400)
	r.Chk("enter lands on gamma despite churn", r.ClientWin() == r.W3)
	r.Chk("sidebar rode along", r.Side().Win == r.W3)
}
