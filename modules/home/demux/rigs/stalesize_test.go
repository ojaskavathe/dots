package rigs

import (
	"strings"
	"testing"
)

// TestStaleWindowSize: windows not viewed since the client changed size
// (monitor switch, other sessions) keep stale dimensions until entered — a
// 60-row window billboarded into a 49-row canvas painted top-aligned clips
// the bottom, so splits look "scrolled up" and entering visibly corrects
// them. The daemon must normalize the window to the docked size when
// billboarding it, and put window-size back to latest.
func TestStaleWindowSize(t *testing.T) {
	r := New(t)

	// ptwo grew on "another monitor": 200x60 vs the client's 200x50 view
	r.T("resize-window", "-t", "play:ptwo", "-x", "200", "-y", "60")
	r.T("set-option", "-w", "-t", "play:ptwo", "window-size", "latest")
	pp := r.T("display-message", "-p", "-t", "play:ptwo", "#{pane_id}")
	r.SendKeys(pp, "seq 100; echo BOTTOMMARK", "Enter")
	sleep(400)
	r.Chk("stale height in place", r.T("display-message", "-p", "-t", "play:ptwo", "#{window_height}") == "60")

	r.D("toggle", r.CL) // dock on beta
	sleep(800)
	sp := r.Side().Pane
	// rows: [play, p1, ptwo, work, w1, beta, gamma] — beta -> ptwo is 3x k
	r.SendKeys(sp, "k", "k", "k")
	sleep(900)
	r.Chk("zoomed", r.Side().Width == 200)

	cap := r.Capture(sp)
	r.Chk("billboard shows the window's bottom", strings.Contains(cap, "BOTTOMMARK"))
	r.Chk("window normalized to docked height", r.T("display-message", "-p", "-t", "play:ptwo", "#{window_height}") == "49")
	r.Chk("window-size back to latest", r.ShowOpt("-wv", "-t", "play:ptwo", "window-size") == "latest")

	// entering must be geometry-free: the billboard promised this exact view
	r.SendKeys(sp, "Enter")
	sleep(800)
	real := r.Capture(pp)
	r.Chk("entered view matches billboard promise", strings.Contains(real, "BOTTOMMARK"))
	r.Chk("no surprise resize on entry", r.T("display-message", "-p", "-t", "play:ptwo", "#{window_height}") == "49")
}
