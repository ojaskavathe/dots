package rigs

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestFlicker records the fake client's terminal stream — the ground truth
// of what a user sees — through a cross-session scrub and an Enter into a
// window too heavy to pre-carve (>carveHistoryMax, so the commit itself pays
// the carve reflow). The origin window prints ORIGMARK; once the sidebar is
// zoomed (billboards showing), origin content must NEVER reach the terminal
// again: any ORIGMARK in the recording is a flicker frame — the origin
// flashing through mid-transition.
func TestFlicker(t *testing.T) {
	if testing.Short() {
		t.Skip("slow: fills 150k lines of scrollback")
	}
	r := New(t)

	// heavy window in work, beyond carveHistoryMax
	r.T("set-option", "-g", "history-limit", "1000000")
	r.T("new-window", "-d", "-t", "work:", "-n", "hv",
		"sh -c 'seq 150000; while :; do echo HEAVYMARK; sleep 2; done'")
	hv := r.T("display-message", "-p", "-t", "work:hv", "#{window_id}")
	r.WaitUntil(600, func() bool {
		h, _ := strconv.Atoi(r.T("display-message", "-p", "-t", "work:hv", "#{history_size}"))
		return h >= 120_000
	})
	h, _ := strconv.Atoi(r.T("display-message", "-p", "-t", "work:hv", "#{history_size}"))
	if h < 120_000 {
		t.Fatalf("hv not heavy: history_size=%d", h)
	}
	r.T("select-window", "-t", r.W1) // work's current window: not the origin

	// origin: play:p1, with a marker on screen
	origPane := r.T("display-message", "-p", "-t", "play:0", "#{pane_id}")
	r.SendKeys(origPane, "while :; do echo ORIGMARK; sleep 2; done", "Enter")
	r.T("switch-client", "-c", r.CL, "-t", "play", ";", "select-window", "-t", r.P1)
	sleep(500)

	r.D("toggle", r.CL)
	sleep(800)
	s := r.Side()
	r.Chk("docked on p1", s.Win == r.P1 && s.Width == 40)
	sp := s.Pane

	// first j zooms (origin content legitimately on screen until then)
	r.SendKeys(sp, "j")
	sleep(700)
	r.Chk("zoomed", r.Side().Width == 200)

	// rows: [play, p1, ptwo, work, w1, beta, gamma, hv] — five more j's to
	// hv, none of which billboards the origin
	r.StartRecord()
	for i := 0; i < 5; i++ {
		r.SendKeys(sp, "j")
		sleep(350)
	}
	sleep(800) // hv billboard (scaled approximation — no pre-carve)
	r.SendKeys(sp, "Enter")
	r.WaitUntil(300, func() bool { return r.ClientWin() == hv })
	sleep(1200) // carve reflow + settle
	rec := r.StopRecord()

	r.Chk("entered hv", r.ClientWin() == hv)
	r.Chk("sidebar docked 40 in hv", r.Side().Win == hv && r.Side().Width == 40)
	r.Chk("hv was never pre-carved", !r.LogHas("bench carve win="+regexp.QuoteMeta(hv)))

	t.Logf("recorded %d bytes; ORIGMARK=%d HEAVYMARK=%d clears=%d",
		len(rec),
		bytes.Count(rec, []byte("ORIGMARK")),
		bytes.Count(rec, []byte("HEAVYMARK")),
		bytes.Count(rec, []byte("\x1b[2J")))
	if i := bytes.Index(rec, []byte("ORIGMARK")); i >= 0 {
		lo := max(0, i-200)
		t.Logf("flicker context: %q", rec[lo:min(len(rec), i+80)])
	}
	r.Chk("origin never flashes after zoom", !bytes.Contains(rec, []byte("ORIGMARK")))

	// the final state must actually show the heavy window
	r.Chk("hv content on screen", strings.Contains(r.Capture(r.T("display-message", "-p", "-t", hv, "#{pane_id}")), "HEAVYMARK") ||
		bytes.Contains(rec, []byte("HEAVYMARK")))
}
