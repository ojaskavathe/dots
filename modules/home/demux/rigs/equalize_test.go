package rigs

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// TestEqualize: tmux-equalize-nvim coexistence — equalize while docked,
// geometry-free leave, proportional give-back at release and at undock.
func TestEqualize(t *testing.T) {
	r := New(t)

	mains := func() [][3]string { // [pane, left, width] of w1's non-demux panes
		var out [][3]string
		for _, ln := range strings.Split(r.T("list-panes", "-t", r.W1, "-F",
			"#{pane_id} #{pane_left} #{pane_width} #{pane_current_command}"), "\n") {
			f := strings.Fields(ln)
			if len(f) == 4 && !strings.Contains(f[3], "demux") {
				out = append(out, [3]string{f[0], f[1], f[2]})
			}
		}
		return out
	}
	width := func(pane string) int {
		for _, m := range mains() {
			if m[0] == pane {
				n, _ := strconv.Atoi(m[2])
				return n
			}
		}
		return -1
	}
	left := func(pane string) int {
		for _, m := range mains() {
			if m[0] == pane {
				n, _ := strconv.Atoi(m[1])
				return n
			}
		}
		return -1
	}
	equalize := func(pane string) {
		cmd := exec.Command(equalizeBin)
		cmd.Env = append(envSansTmux(),
			"TMUX=/private/tmp/tmux-501/"+r.L+",1,0", "TMUX_PANE="+pane)
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("equalize: %v\n%s", err, b)
		}
	}
	m := mains()
	if len(m) < 2 {
		t.Fatalf("w1 needs 2 mains, got %d", len(m))
	}
	m0, m1 := m[0][0], m[1][0]

	// M: equalize while docked (main region only)
	r.D("toggle", r.CL)
	sleep(600)
	r.T("select-window", "-t", r.W1)
	r.WaitUntil(100, func() bool { return r.Side().Win == r.W1 })
	r.Chk("docked in w1", r.Side().Win == r.W1)
	r.T("resize-pane", "-t", m0, "-x", "50")
	sleep(300)
	r.Chk("mains unequal before", width(m0) == 50)
	equalize(m0)
	sleep(500)
	s := r.Side()
	r.Chk("sidebar untouched at 40", s.Left == 0 && s.Width == 40)
	r.Chk("mains equalized", abs(width(m0)-width(m1)) <= 1)
	r.Chk("pane order preserved", left(m0) < left(m1))
	r.Chk("dirty marker set", r.ShowOpt("-wv", "-t", r.W1, "@demux_layout_dirty") == "1")

	// N: commit elsewhere keeps geometry; undock gives back
	we0, we1 := width(m0), width(m1)
	sp := r.Side().Pane
	r.T("select-pane", "-t", sp)
	r.SendKeys(sp, "j")
	sleep(400)
	r.SendKeys(sp, "Enter")
	r.WaitUntil(100, func() bool { return r.ClientWin() != r.W1 })
	sleep(400)
	// leaving is geometry-free: the equalized mains DON'T move, the spacer
	// holds the slot, dirty survives until release
	r.Chk("leave keeps equalized mains", width(m0) == we0 && width(m1) == we1)
	r.Chk("spacer holds w1 slot", countLeftSpacer(r.T("list-panes", "-t", r.W1, "-F", "#{pane_left} #{pane_width}")) == 1)
	r.Chk("dirty survives leave", r.ShowOpt("-wv", "-t", r.W1, "@demux_layout_dirty") == "1")
	r.D("toggle", r.CL) // undock: release w1
	sleep(600)
	r.Chk("give-back full width", width(m0)+width(m1) == 199)
	r.Chk("give-back proportional", abs(width(m0)-width(m1)) <= 1)
	r.Chk("dirty marker cleared", r.ShowOpt("-wv", "-t", r.W1, "@demux_layout_dirty") == "")
	r.Chk("release swept w1 spacer", r.Spacers() == 0)

	// O: equalize then toggle-off give-back (dirty window IS the docked one)
	r.D("toggle", r.CL)
	sleep(600)
	r.T("select-window", "-t", r.W1)
	r.WaitUntil(100, func() bool { return r.Side().Win == r.W1 })
	r.T("resize-pane", "-t", m0, "-x", "50")
	sleep(300)
	equalize(m0)
	sleep(500)
	r.D("toggle", r.CL)
	sleep(600)
	r.Chk("undock give-back full width", width(m0)+width(m1) == 199)
	r.Chk("undock give-back equal", abs(width(m0)-width(m1)) <= 1)
	r.Chk("no sidebar in w1", r.DemuxPanes("-t", r.W1) == 0)
	r.Chk("dirty cleared on undock", r.ShowOpt("-wv", "-t", r.W1, "@demux_layout_dirty") == "")
}
