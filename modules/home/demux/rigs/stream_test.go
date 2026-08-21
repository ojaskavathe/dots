package rigs

import (
	"strings"
	"testing"
)

// TestStream: the billboard stream's cost levers. A busy target ships row
// DELTAS (not full frames) once its first full frame establishes lineage; a
// quiet target trips the activity gate and stops paying captures at all.
// Content assertions ride the same scrub, so a delta-path bug that corrupted
// the canvas would fail here, not just miss a bench line.
func TestStream(t *testing.T) {
	r := New(t)

	r.D("browse", r.CL)
	sleep(1000)
	s := r.Side()

	// w1 holds the MARKW1 echo loop (2s period): its scrolling output must
	// arrive as delta frames, and the canvas must keep showing the marker.
	r.SendKeys(s.Pane, "k")
	sleep(700)
	r.Chk("billboard shows w1 content", strings.Contains(r.Capture(s.Pane), "MARKW1"))
	r.Chk("busy target ships deltas", r.WaitUntil(400, func() bool { return r.LogHas("bench frame delta") }))
	r.Chk("billboard still correct after deltas", strings.Contains(r.Capture(s.Pane), "MARKW1"))
	r.Chk("TUI applied a delta", r.WaitUntil(200, func() bool { return r.TuiBenchHas("delta=true") }))
	r.Chk("no delta resyncs", !r.TuiBenchHas("delta resync"))

	// gamma is quiet (no output since setup): the stream must gate — no
	// capture batches, just the idle edge log once.
	r.SendKeys(s.Pane, "j", "j")
	sleep(800)
	r.Chk("quiet target gates captures", r.LogHas("bench gate idle"))

	// scrub back to the busy window: the gate must release (fresh full
	// frame for the new target, deltas after).
	r.SendKeys(s.Pane, "k", "k")
	sleep(700)
	r.Chk("gate releases on return", strings.Contains(r.Capture(s.Pane), "MARKW1"))

	r.SendKeys(s.Pane, "q")
	sleep(500)
	r.D("toggle", r.CL)
	sleep(1000)
}
