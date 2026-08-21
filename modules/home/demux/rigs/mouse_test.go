package rigs

import (
	"strconv"
	"strings"
	"testing"
)

// TestMouse: wheel scrubs, click selects, click-again enters, and a click
// on a billboard split enters that window with the clicked split focused.
func TestMouse(t *testing.T) {
	r := New(t)

	r.D("toggle", r.CL) // dock on beta; rows [play,p1,ptwo,work,w1,beta,gamma]
	sleep(800)
	sp := r.Side().Pane

	// wheel up = k: selection to w1, zoom + billboard
	r.Mouse(sp, 64, 5, 3, true)
	sleep(800)
	r.Chk("wheel scrubs (zoom)", r.Side().Width == 200)
	r.Chk("wheel billboards w1", strings.Contains(r.Capture(sp), "MARKW1"))

	// click the ptwo row (index 2 -> y=3): selects; second click enters
	pt := r.T("display-message", "-p", "-t", "play:ptwo", "#{window_id}")
	r.Click(sp, 5, 3)
	sleep(700)
	r.Chk("click keeps scrubbing", r.ClientWin() == r.W2)
	r.Click(sp, 5, 3)
	r.WaitUntil(150, func() bool { return r.ClientWin() == pt })
	sleep(500)
	r.Chk("click-click enters ptwo", r.ClientWin() == pt)
	r.Chk("sidebar docked in ptwo", r.Side().Win == pt && r.Side().Width == 40)

	// canvas click: wheel down to w1's billboard, click inside the RIGHT
	// split -> enters w1 with that split focused (not the last-active one)
	sp = r.Side().Pane
	r.T("select-pane", "-t", sp)
	r.Mouse(sp, 65, 5, 3, true) // -> work header
	sleep(500)
	r.Mouse(sp, 65, 5, 3, true) // -> w1
	sleep(900)
	r.Chk("billboard shows w1", strings.Contains(r.Capture(sp), "MARKW1"))
	r.Click(sp, 160, 10)
	r.WaitUntil(150, func() bool { return r.ClientWin() == r.W1 })
	sleep(500)
	r.Chk("canvas click enters w1", r.ClientWin() == r.W1)
	right := ""
	for _, ln := range strings.Split(r.T("list-panes", "-t", r.W1, "-F", "#{pane_id} #{pane_left}"), "\n") {
		f := strings.Fields(ln)
		if len(f) != 2 {
			continue
		}
		if n, _ := strconv.Atoi(f[1]); n > 100 {
			right = f[0]
		}
	}
	active := r.T("display-message", "-p", "-t", r.W1, "#{pane_id}")
	t.Logf("right=%s active=%s", right, active)
	r.Chk("canvas click focuses the clicked split", right != "" && active == right)
}
