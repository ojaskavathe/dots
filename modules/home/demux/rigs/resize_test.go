package rigs

import (
	"strings"
	"testing"
	"unsafe"
)

// ResizeClient changes the fake client's terminal size mid-test (what a
// monitor switch does to a real terminal). The kernel delivers SIGWINCH to
// the attached tmux client, which reports the new size to the server.
func (r *Rig) ResizeClient(rows, cols int) {
	r.t.Helper()
	ws := winsize{row: uint16(rows), col: uint16(cols)}
	if err := ioctl(r.ptyMaster.Fd(), tiocswinsz, uintptr(unsafe.Pointer(&ws))); err != nil {
		r.t.Fatalf("resize client: %v", err)
	}
}

// TestClientResize: a monitor switch (client resize) while docked must not
// break the dock — sidebar snaps back to 40, scrub/commit still work, undock
// leaves no debris.
func TestClientResize(t *testing.T) {
	r := New(t)

	r.D("toggle", r.CL) // dock on beta
	sleep(800)
	r.Chk("docked at 40 pre-resize", r.Side().Width == 40)

	// visit w1 so beta holds a spacer carved at the OLD size
	sp := r.Side().Pane
	r.T("select-pane", "-t", sp)
	r.SendKeys(sp, "k")
	sleep(500)
	r.SendKeys(sp, "Enter")
	r.WaitUntil(100, func() bool { return r.ClientWin() == r.W1 })
	sleep(400)
	r.Chk("on w1 pre-resize", r.ClientWin() == r.W1)

	// the monitor switch
	r.ResizeClient(30, 120)
	sleep(1500)

	t.Logf("post-resize w1 width: %s", r.T("display-message", "-p", "-t", r.W1, "#{window_width}"))
	t.Logf("post-resize sidebar: %+v", r.Side())
	s := r.Side()
	r.Chk("sidebar back at 40 after resize", s.Width == 40)
	r.Chk("sidebar still at left edge", s.Left == 0)

	// scrub still works at the new size (commits hand off to a fresh TUI
	// pane — re-fetch after the landing above)
	sp = r.Side().Pane
	r.T("select-pane", "-t", sp)
	r.SendKeys(sp, "j")
	sleep(600)
	r.Chk("scrub zooms at new size", r.Side().Width == 120)
	found := false
	for _, ln := range strings.Split(r.Capture(sp), "\n") {
		if strings.Contains(ln, "beta") {
			found = true
			break
		}
	}
	r.Chk("wide list painted", found)
	r.SendKeys(sp, "Enter")
	r.WaitUntil(100, func() bool { return r.ClientWin() == r.W2 })
	sleep(400)
	r.Chk("commit works after resize", r.ClientWin() == r.W2)
	r.Chk("sidebar 40 in beta", r.Side().Width == 40)

	// undock: no spacers, no dead panes, options clean
	r.D("toggle", r.CL)
	sleep(1200)
	r.Chk("sidebar left user windows", r.DemuxPanes("-s", "-t", "work") == 0)
	r.Chk("no spacers remain", r.Spacers() == 0)
	r.Chk("@demux_docked cleared", r.ShowOpt("-t", "work", "-v", "@demux_docked") == "")
	r.Chk("status unpadded", r.ShowOpt("-t", "work", "status-left") == "")
	t.Logf("post-undock w1 layout: %s", r.Layout(r.W1))
	t.Logf("post-undock w2 layout: %s", r.Layout(r.W2))
}
