package rigs

import (
	"strings"
	"testing"
)

// TestScrollPreview: wheel over a billboard split scrolls THAT pane's
// preview into history (daemon-side capture offset — the real pane never
// enters copy-mode), instead of walking the sidebar list selection (the
// original bug). Scrolling back down returns to the live view.
func TestScrollPreview(t *testing.T) {
	r := New(t)

	// Fill gamma's shell with numbered scrollback, then billboard gamma
	// itself (browse docks into the client's current window).
	r.T("select-window", "-t", r.W3)
	r.SendKeys(r.W3, "for i in $(seq 1 300); do echo SCROLLMARK-$i; done", "Enter")
	sleep(800)
	r.D("browse", r.CL)
	sleep(1000)
	s := r.Side()

	r.Chk("canvas shows live tail", r.WaitUntil(600, func() bool {
		return strings.Contains(r.Capture(s.Pane), "SCROLLMARK-300")
	}))
	r.Chk("old line not visible yet", !strings.Contains(r.Capture(s.Pane), "SCROLLMARK-230"))

	// 10 wheel-up events over the canvas (x=100 > list width): 3 lines
	// each = 30 back. The listing must not scroll — gamma stays the target.
	for i := 0; i < 10; i++ {
		r.Mouse(s.Pane, 64, 100, 20, true)
	}
	r.Chk("wheel scrolls the split back", r.WaitUntil(800, func() bool {
		c := r.Capture(s.Pane)
		return strings.Contains(c, "↑30") && strings.Contains(c, "SCROLLMARK-230")
	}))
	cap := r.Capture(s.Pane)
	r.Chk("live tail scrolled away", !strings.Contains(cap, "SCROLLMARK-300"))
	r.Chk("still gamma's billboard", strings.Contains(cap, "SCROLLMARK"))

	// Wheel back down past the bottom: clamps at live, indicator gone.
	for i := 0; i < 12; i++ {
		r.Mouse(s.Pane, 65, 100, 20, true)
	}
	r.Chk("wheel down returns to live", r.WaitUntil(800, func() bool {
		c := r.Capture(s.Pane)
		return strings.Contains(c, "SCROLLMARK-300") && !strings.Contains(c, "↑")
	}))

	r.SendKeys(s.Pane, "q")
	sleep(500)
	r.D("toggle", r.CL)
	sleep(1000)
	r.Chk("gamma layout intact", r.Layout(r.W3) == tail(r.LW3))
}
