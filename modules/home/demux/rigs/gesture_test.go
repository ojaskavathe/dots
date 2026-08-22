package rigs

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestGestureCommits: any mouse gesture on a billboard split — wheel,
// middle, right — ENTERS it for real, focused on the split under the
// pointer, instead of walking the sidebar list (the original bug). The
// billboard can't fake these faithfully (alt-screen apps have no history,
// paste and menus belong to the real pane), so the gesture routes to
// reality and the real pane's own handling has full parity. Also: the
// billboard paints the active pane's cursor — a canvas without one reads
// as a screenshot.
func TestGestureCommits(t *testing.T) {
	r := New(t)

	// markPane locates the MARKW1 pane in w1's (carved) geometry and its
	// center on the canvas (the 40-col spacer keeps coordinates 1-based
	// aligned with the billboard).
	markPane := func() (string, int, int) {
		for _, ln := range strings.Split(r.T("list-panes", "-t", r.W1, "-F",
			"#{pane_id} #{pane_left} #{pane_top} #{pane_width} #{pane_height} #{pane_start_command}"), "\n") {
			f := strings.SplitN(ln, " ", 6)
			if len(f) == 6 && strings.Contains(f[5], "MARKW1") {
				l, _ := strconv.Atoi(f[1])
				tp, _ := strconv.Atoi(f[2])
				w, _ := strconv.Atoi(f[3])
				h, _ := strconv.Atoi(f[4])
				return f[0], l + w/2 + 1, tp + h/2 + 1
			}
		}
		return "", 0, 0
	}
	activePane := func() string {
		for _, ln := range strings.Split(r.T("list-panes", "-t", r.W1, "-F", "#{pane_id} #{pane_active}"), "\n") {
			if strings.HasSuffix(ln, " 1") {
				return strings.Fields(ln)[0]
			}
		}
		return ""
	}
	scrubToW1 := func() string {
		r.D("browse", r.CL)
		sleep(1000)
		s := r.Side()
		r.SendKeys(s.Pane, "k")
		sleep(900)
		r.Chk("billboard shows w1", strings.Contains(r.Capture(s.Pane), "MARKW1"))
		return s.Pane
	}

	// Round 1: wheel. Plus the cursor check while the billboard is up —
	// the active pane's cursor cell paints as a lone inverse char.
	sp := scrubToW1()
	// The cursor cell: reverse video around a SINGLE char (tmux may
	// interleave other SGR transitions when re-serializing the row). The
	// list's selection bar is also reverse but spans the whole row, so
	// one-char-then-escape can't match it.
	cursor := regexp.MustCompile(`\x1b\[7m(?:\x1b\[[0-9;]*m)*.\x1b\[`)
	r.Chk("billboard paints a cursor cell", r.WaitUntil(300, func() bool {
		raw, _ := r.TQ("capture-pane", "-p", "-e", "-t", sp)
		return cursor.MatchString(raw)
	}))
	mark, mx, my := markPane()
	r.Chk("MARKW1 pane located", mark != "")
	r.Mouse(sp, 64, mx, my, true) // wheel up over the split
	sleep(1200)
	r.Chk("wheel committed into w1", r.Side().Win == r.W1)
	r.Chk("sidebar docked after commit", r.Side().Width == 40)
	r.Chk("wheeled split has focus", activePane() == mark)
	r.D("toggle", r.CL)
	sleep(1000)

	// Round 2: right press commits the same way (paste/menu gestures).
	sp = scrubToW1()
	mark, mx, my = markPane()
	r.Mouse(sp, 2, mx, my, true)
	r.Mouse(sp, 2, mx, my, false)
	sleep(1200)
	r.Chk("right-click committed into w1", r.Side().Win == r.W1)
	r.Chk("right-clicked split has focus", activePane() == mark)
	r.D("toggle", r.CL)
	sleep(1000)
	r.Chk("w1 layout restored", r.Layout(r.W1) == tail(r.LW1))
}
