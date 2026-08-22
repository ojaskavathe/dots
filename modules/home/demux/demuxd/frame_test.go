package main

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

// Fixed geometry: deltas only exist between same-shape frames (a height
// change alters the rect and forces a full frame); line counts still vary
// within it because capture trims trailing blanks.
func fp(id string, lines ...string) framePane {
	return framePane{ID: id, Width: 10, Height: 5, Lines: lines}
}

// Delta shipped by the daemon, applied by the TUI, must reproduce the
// daemon's new frame exactly — including rows the new capture lost.
func TestDeltaRoundTrip(t *testing.T) {
	old := []framePane{fp("%1", "a", "b", "c"), fp("%2", "x", "y")}
	cur := []framePane{fp("%1", "a", "B", "c", "d"), fp("%2", "x", "y")}
	delta, rows := deltaPanes(old, cur)
	if rows != 2 || len(delta) != 1 || delta[0].ID != "%1" {
		t.Fatalf("delta = %+v rows=%d", delta, rows)
	}
	cache := append([]framePane(nil), old...)
	if !applyDelta(&cache, delta) {
		t.Fatal("applyDelta refused a valid delta")
	}
	if !framesEqual(cache, cur) {
		t.Fatalf("patched cache != current: %+v vs %+v", cache, cur)
	}
	// The original cache slices must be untouched (painted state shares them).
	if old[0].Lines[1] != "b" {
		t.Fatal("applyDelta mutated the old lines in place")
	}
}

func TestDeltaIdentical(t *testing.T) {
	a := []framePane{fp("%1", "a", "b")}
	b := []framePane{fp("%1", "a", "b")}
	if delta, rows := deltaPanes(a, b); delta != nil || rows != 0 {
		t.Fatalf("identical frames produced delta %+v", delta)
	}
}

// A shrinking capture (fewer trailing lines) deltas the lost rows to "" so
// the TUI blanks them instead of keeping stale content.
func TestDeltaShrink(t *testing.T) {
	old := []framePane{fp("%1", "a", "b", "c")}
	cur := []framePane{fp("%1", "a")}
	delta, rows := deltaPanes(old, cur)
	if rows != 2 || len(delta) != 1 {
		t.Fatalf("delta = %+v rows=%d", delta, rows)
	}
	cache := append([]framePane(nil), old...)
	if !applyDelta(&cache, delta) {
		t.Fatal("applyDelta refused")
	}
	for _, r := range []int{1, 2} {
		if cache[0].Lines[r] != "" {
			t.Fatalf("row %d not blanked: %q", r, cache[0].Lines[r])
		}
	}
}

// A pane id swap behind identical rects is a different grid: never delta.
func TestFrameShapeIdSwap(t *testing.T) {
	a := []framePane{fp("%1", "a")}
	b := []framePane{fp("%9", "a")}
	if sameFrameShape(a, b) {
		t.Fatal("id swap treated as same shape")
	}
}

// A delta naming a pane the cache doesn't hold must refuse, not corrupt.
func TestApplyDeltaUnknownPane(t *testing.T) {
	cache := []framePane{fp("%1", "a")}
	delta := []framePane{{ID: "%9", Rows: []int{0}, Lines: []string{"z"}}}
	if applyDelta(&cache, delta) {
		t.Fatal("applied a delta for an unknown pane")
	}
}

// Benchmarks: the stream tick's daemon-side tail, old world vs new. The old
// path marshaled the full frame every tick just to bytes.Equal it; the new
// path diffs strings first and marshals only what changed.
func benchFrame() []framePane {
	line := strings.Repeat("\x1b[38;2;180;190;254mlorem ipsum dolor \x1b[0m", 4)
	panes := make([]framePane, 4)
	for i := range panes {
		lines := make([]string, 50)
		for j := range lines {
			lines[j] = line
		}
		panes[i] = framePane{ID: "%" + strconv.Itoa(i), Width: 100, Height: 50, Lines: lines}
	}
	return panes
}

func BenchmarkOldTickUnchanged(b *testing.B) { // marshal full + compare
	panes := benchFrame()
	prev := marshalLine(frameMsg{Type: "frame", Window: "@1", Panes: panes})
	b.ReportMetric(float64(len(prev)), "payload_bytes")
	for b.Loop() {
		p := marshalLine(frameMsg{Type: "frame", Window: "@1", Panes: panes})
		if !bytes.Equal(p, prev) {
			b.Fatal("unequal")
		}
	}
}

func BenchmarkNewTickUnchanged(b *testing.B) { // diff only, no marshal
	old, cur := benchFrame(), benchFrame()
	for b.Loop() {
		if d, _ := deltaPanes(old, cur); d != nil {
			b.Fatal("unexpected delta")
		}
	}
}

func BenchmarkNewTickBusy(b *testing.B) { // diff + marshal a 3-row delta
	old, cur := benchFrame(), benchFrame()
	cur[1].Lines[47], cur[1].Lines[48], cur[1].Lines[49] = "a", "b", "c"
	d, _ := deltaPanes(old, cur)
	b.ReportMetric(float64(len(marshalLine(frameMsg{Type: "frame", Window: "@1", Panes: d, Delta: true}))), "payload_bytes")
	for b.Loop() {
		d, _ := deltaPanes(old, cur)
		marshalLine(frameMsg{Type: "frame", Window: "@1", Panes: d, Delta: true})
	}
}
