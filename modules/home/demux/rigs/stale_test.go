package rigs

import (
	"os"
	"bytes"
	"strings"
	"testing"
)

// TestStaleBillboard: mid-scrub, moving the selection paints the target's
// CACHED frame instantly (wide mode), fresh capture following ~50ms later.
// Neighbor caches come from prefetch at arrival — sit on one billboard for a
// few seconds while a neighbor's content changes, and that neighbor's cache
// is stale: scrubbing onto it flashes the old screen before the fresh frame
// lands (the "random flicker when switching windows in the sidebar"). Stale
// caches must not paint; the canvas waits for the fresh frame.
func TestStaleBillboard(t *testing.T) {
	r := New(t)

	// a plain pane in w1 gets distinctive content (w1 also runs MARKW1)
	shell := ""
	for _, ln := range strings.Split(r.T("list-panes", "-t", r.W1, "-F", "#{pane_id} #{pane_start_command}"), "\n") {
		f := strings.SplitN(ln, " ", 2)
		if len(f) == 2 && !strings.Contains(f[1], "MARKW1") {
			shell = f[0]
		}
	}
	if shell == "" {
		t.Fatal("no plain shell pane in w1")
	}
	r.SendKeys(shell, "echo OLDMARK; echo OLDMARK", "Enter")
	sleep(300)

	// dock on play:p1, scrub to ptwo — prefetch caches w1 (the work header's
	// target) with OLDMARK on screen
	r.T("select-window", "-t", "work:0")
	r.T("switch-client", "-c", r.CL, "-t", "play", ";", "select-window", "-t", r.P1)
	sleep(400)
	r.D("toggle", r.CL)
	sleep(800)
	sp := r.Side().Pane
	r.SendKeys(sp, "j") // p1 -> ptwo: zoom
	sleep(700)
	r.Chk("zoomed on ptwo", r.Side().Width == 200)
	r.SendKeys(sp, "j") // -> work header (w1): caches w1's OLDMARK frame
	sleep(600)
	r.SendKeys(sp, "k") // back to ptwo
	sleep(500)

	// Sit well past frameTTL (3s) while w1's content changes under the
	// cache. Generous margin: under full-suite parallel load the prefetch
	// that stamps the cache can land seconds late, and a cache younger than
	// the TTL painting old content is accepted behavior, not the bug.
	r.SendKeys(shell, "clear; while :; do echo FRESHMARK; sleep 2; done", "Enter")
	sleep(7000)

	r.StartRecord()
	r.SendKeys(sp, "j") // -> work header row, target w1: stale cache moment
	sleep(900)
	rec := r.StopRecord()

	t.Logf("recorded %d bytes; OLDMARK=%d FRESHMARK=%d",
		len(rec), bytes.Count(rec, []byte("OLDMARK")), bytes.Count(rec, []byte("FRESHMARK")))
	if i := bytes.Index(rec, []byte("OLDMARK")); i >= 0 {
		lo := max(0, i-200)
		t.Logf("first OLDMARK context: %q", rec[lo:min(len(rec), i+120)])
	}
	r.Chk("stale cache never painted", !bytes.Contains(rec, []byte("OLDMARK")))
	r.Chk("fresh content painted", bytes.Contains(rec, []byte("FRESHMARK")))

	if b, err := os.ReadFile(r.Sock + ".tui-bench.log"); err == nil {
		lines := strings.Split(strings.TrimSpace(string(b)), "\n")
		for _, ln := range lines {
			if strings.Contains(ln, "key sel") || strings.Contains(ln, "frame win") ||
				strings.Contains(ln, "paint_frame") || strings.Contains(ln, "stale skip") {
				t.Logf("bench: %s", ln)
			}
		}
	}
}
