package main

import "testing"

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
