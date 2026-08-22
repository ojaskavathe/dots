package rigs

import (
	"strconv"
	"strings"
	"testing"
)

// TestScrollCommits: wheeling over a billboard split ENTERS it for real —
// the same commit a click sends, focused on the split under the pointer —
// instead of walking the sidebar list selection (the original bug). The
// billboard can't scroll faithfully (alt-screen apps like vim have no
// history), so a scroll gesture routes to the real pane, whose own mouse
// handling then has full parity with everything else.
func TestScrollCommits(t *testing.T) {
	r := New(t)

	r.D("browse", r.CL)
	sleep(1000)
	s := r.Side()
	r.SendKeys(s.Pane, "k") // scrub to w1 (stream_test's path)
	sleep(900)
	r.Chk("billboard shows w1", strings.Contains(r.Capture(s.Pane), "MARKW1"))

	// Locate the MARKW1 pane in w1's (now carved) geometry and aim the
	// wheel at its center on the canvas: canvas x = pane x + 1 (the 40-col
	// spacer occupies the sidebar's slot, so coordinates line up 1-based).
	mark, mx, my := "", 0, 0
	for _, ln := range strings.Split(r.T("list-panes", "-t", r.W1, "-F",
		"#{pane_id} #{pane_left} #{pane_top} #{pane_width} #{pane_height} #{pane_start_command}"), "\n") {
		f := strings.SplitN(ln, " ", 6)
		if len(f) == 6 && strings.Contains(f[5], "MARKW1") {
			l, _ := strconv.Atoi(f[1])
			tp, _ := strconv.Atoi(f[2])
			w, _ := strconv.Atoi(f[3])
			h, _ := strconv.Atoi(f[4])
			mark, mx, my = f[0], l+w/2+1, tp+h/2+1
		}
	}
	r.Chk("MARKW1 pane located", mark != "")

	r.Mouse(s.Pane, 64, mx, my, true) // wheel up over the split
	sleep(1200)

	side := r.Side()
	r.Chk("wheel committed into w1", side.Win == r.W1)
	r.Chk("sidebar docked after commit", side.Width == 40)
	active := ""
	for _, ln := range strings.Split(r.T("list-panes", "-t", r.W1, "-F", "#{pane_id} #{pane_active}"), "\n") {
		if strings.HasSuffix(ln, " 1") {
			active = strings.Fields(ln)[0]
		}
	}
	r.Chk("wheeled split has focus", active == mark)

	r.D("toggle", r.CL)
	sleep(1000)
	r.Chk("w1 layout restored", r.Layout(r.W1) == tail(r.LW1))
}
