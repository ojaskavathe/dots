package rigs

import (
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// TestDockScrub: dock, billboard scrub, enter exactness, escape recovery,
// M-s commit-and-dismiss.
func TestDockScrub(t *testing.T) {
	r := New(t)

	// A: toggle on (dock into beta)
	r.D("toggle", r.CL)
	sleep(800)
	s := r.Side()
	r.Chk("sidebar pane exists", s.Pane != "")
	r.Chk("sidebar in beta", s.Win == r.W2)
	r.Chk("sidebar at left edge", s.Left == 0)
	r.Chk("sidebar 40 cols", s.Width == 40)
	r.Chk("sidebar focused", s.Active == 1)
	r.Chk("@demux_docked on work", r.ShowOpt("-t", "work", "-v", "@demux_docked") == "1")
	r.Chk("status-left padded 41", len(r.ShowOpt("-t", "work", "status-left")) >= 45)
	_, err := r.TQ("has-session", "-t", "_demux")
	r.Chk("_demux alive", err == nil)
	cap := r.Capture(s.Pane)
	r.Chk("narrow list shows sessions", strings.Contains(cap, "work") && strings.Contains(cap, "play"))
	r.Chk("narrow list has no border", !strings.Contains(cap, "│"))
	sp := s.Pane
	betaMain := r.T("list-panes", "-t", r.W2, "-F", "#{pane_id} #{pane_width}")

	// B: scrub k -> billboard (zoom, nothing real moves)
	t0 := time.Now()
	r.SendKeys(sp, "k")
	lat := -1
	for i := 0; i < 200; i++ {
		if strings.Contains(r.Capture(sp), "MARKW1") {
			lat = int(time.Since(t0).Milliseconds())
			break
		}
		sleep(10)
	}
	r.Chk("billboard shows w1 content", lat >= 0)
	t.Logf("first-billboard latency: %dms", lat)
	r.Chk("client window UNCHANGED", r.ClientWin() == r.W2)
	r.Chk("beta zoomed", r.Zoomed(r.W2))
	s = r.Side()
	r.Chk("sidebar full width (zoom)", s.Width == 200)
	after := r.T("list-panes", "-t", r.W2, "-F", "#{pane_id} #{pane_width}")
	r.Chk("hidden mains kept size", sansLine(after, sp) == sansLine(betaMain, sp))
	r.Chk("wide list border painted", strings.Contains(r.Capture(sp), "│"))
	// w1 is a ~100/99 split; the marker lives in the RIGHT pane. Cropped
	// (old behavior) it starts at col ~142; scaled to the canvas, col ~122.
	// The FIRST frame is the pre-carve approximation — the carve recaptures
	// at docked geometry a beat later, and exactness is asserted on THAT.
	sleep(400)
	bbCap := r.Capture(sp)
	pos := markerCol(bbCap)
	r.Chk("split scaled to canvas", pos > 0 && pos < 135)

	// B2: Enter -> commits for real
	r.SendKeys(sp, "Enter")
	r.WaitUntil(100, func() bool { return r.ClientWin() == r.W1 })
	sleep(400)
	s = r.Side()
	r.Chk("client now on w1", r.ClientWin() == r.W1)
	r.Chk("sidebar docked in w1", s.Win == r.W1 && s.Width == 40)
	r.Chk("zoom cleared", !r.Zoomed(r.W1) && !r.Zoomed(r.W2))
	// leaving is geometry-free: beta keeps its docked shape, a spacer
	// holding the sidebar's slot (byte-exact restore at release, checked
	// in the dismiss section)
	r.Chk("beta holds spacer at left", countLeftSpacer(r.T("list-panes", "-t", r.W2, "-F", "#{pane_left} #{pane_width}")) == 1)
	r.Chk("commit focuses main", s.Active == 0)
	// billboard EXACTNESS: the marker column on the billboard equals the
	// marker pane's real on-screen column after entering (pane_left 0-based)
	mreal := 0
	for _, ln := range strings.Split(r.T("list-panes", "-t", r.W1, "-F", "#{pane_left}"), "\n") {
		if n, _ := strconv.Atoi(ln); n > 41 {
			mreal = n + 1
			break
		}
	}
	t.Logf("billboard col=%d real col=%d", pos, mreal)
	if pos == 0 || abs(pos-mreal) > 1 {
		t.Logf("billboard line: %q", firstMarkerLine(bbCap))
		for _, ln := range strings.Split(r.T("list-panes", "-t", r.W1, "-F", "#{pane_id} #{pane_left} #{pane_width}"), "\n") {
			t.Logf("real pane: %s", ln)
		}
	}
	r.Chk("billboard == docked reality", pos > 0 && abs(pos-mreal) <= 1)

	// P: escaping a billboard (vim-navigator C-l) recovers
	r.T("select-pane", "-t", sp)
	r.SendKeys(sp, "j")
	sleep(500)
	r.Chk("scrub zoomed", r.Zoomed(r.W1))
	main := ""
	for _, ln := range strings.Split(r.T("list-panes", "-t", r.W1, "-F", "#{pane_id}"), "\n") {
		if ln != sp {
			main = ln
			break
		}
	}
	r.T("select-pane", "-t", main)
	sleep(500)
	r.Chk("escape unzoomed (tmux)", !r.Zoomed(r.W1))
	r.Chk("daemon ended scrub", r.LogHas("scrub unzoomed externally"))
	r.T("select-pane", "-t", sp)
	r.SendKeys(sp, "j")
	sleep(500)
	r.Chk("j scrubs again after escape", r.Zoomed(r.W1))
	r.SendKeys(sp, "q")
	sleep(500)
	r.Chk("q settles back", !r.Zoomed(r.W1) && r.ClientWin() == r.W1)

	// C: storm kkkk + M-s commits AND dismisses
	r.T("select-pane", "-t", sp)
	r.SendKeys(sp, "k", "k", "k", "k")
	sleep(600)
	r.Chk("storm stays put (billboard)", r.ClientWin() == r.W1)
	r.D("toggle", r.CL)
	sleep(800)
	r.Chk("M-s landed on play", r.ClientSess() == "play")
	r.Chk("sidebar dismissed", r.DemuxPanes("-s", "-t", "work") == 0 && r.DemuxPanes("-s", "-t", "play") == 0)
	r.Chk("w1 layout exact after undock", r.Layout(r.W1) == tail(r.LW1))
	r.Chk("beta layout exact after undock", r.Layout(r.W2) == tail(r.LW2))
	r.Chk("no spacers remain", r.Spacers() == 0)
	r.Chk("@demux_docked cleared", r.ShowOpt("-t", "play", "-v", "@demux_docked") == "" &&
		r.ShowOpt("-t", "work", "-v", "@demux_docked") == "")
	r.Chk("status unpadded everywhere", r.ShowOpt("-t", "work", "status-left") == "" &&
		r.ShowOpt("-t", "play", "status-left") == "")
}

func tail(layout string) string {
	if i := strings.Index(layout, ","); i >= 0 {
		return layout[i+1:]
	}
	return layout
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// markerCol is the 1-based display column of MARKW1 on the first line
// containing it. Counted in RUNES, not bytes — the │ border glyphs before
// the marker are 3 bytes each and would inflate a byte offset.
func markerCol(capture string) int {
	for _, ln := range strings.Split(capture, "\n") {
		if i := strings.Index(ln, "MARKW1"); i >= 0 {
			return utf8.RuneCountInString(ln[:i]) + 1
		}
	}
	return 0
}

// sansLine drops the line starting with the given pane id.
func sansLine(list, pane string) string {
	var out []string
	for _, ln := range strings.Split(list, "\n") {
		if !strings.HasPrefix(ln, pane+" ") {
			out = append(out, ln)
		}
	}
	return strings.Join(out, "\n")
}

// countLeftSpacer counts panes at left=0 width=40 in a "left width" listing.
func countLeftSpacer(list string) int {
	n := 0
	for _, ln := range strings.Split(list, "\n") {
		if f := strings.Fields(ln); len(f) == 2 && f[0] == "0" && f[1] == "40" {
			n++
		}
	}
	return n
}

func firstMarkerLine(capture string) string {
	for _, ln := range strings.Split(capture, "\n") {
		if strings.Contains(ln, "MARKW1") {
			return ln
		}
	}
	return ""
}
