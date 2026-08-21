package rigs

import (
	"strings"
	"testing"
)

// TestNavFollow: routed nav, scrub focus handoff, unrouted-switch follow,
// undock restore, q mid-scrub.
func TestNavFollow(t *testing.T) {
	r := New(t)
	r.D("toggle", r.CL) // dock on beta
	sleep(800)

	// D: nav next (routed M-l, real switch)
	navWin := r.ClientWin()
	r.D("nav", "next", r.CL)
	sleep(500)
	s := r.Side()
	r.Chk("nav changed window", r.ClientWin() != navWin)
	r.Chk("sidebar rode along", s.Win == r.ClientWin())
	r.Chk("nav focuses MAIN pane", s.Active == 0)

	// E: scrub grabs focus; Enter hands it back.
	// k, not j: nav landed on gamma, the last list row — j has nowhere to go.
	sp := r.Side().Pane
	r.T("select-pane", "-t", sp)
	r.SendKeys(sp, "k")
	sleep(500)
	s = r.Side()
	r.Chk("scrub zooms sidebar", s.Width == 200)
	r.Chk("scrub keeps sidebar focus", s.Active == 1)
	r.SendKeys(sp, "Enter")
	sleep(600)
	s = r.Side()
	r.Chk("enter docks at 40", s.Width == 40)
	r.Chk("enter focuses main", s.Active == 0)

	// F: unrouted switch -> follow
	r.T("switch-client", "-c", r.CL, "-t", "work", ";", "select-window", "-t", r.W3)
	r.WaitUntil(100, func() bool { return r.Side().Win == r.W3 })
	s = r.Side()
	r.Chk("follow docked into gamma", s.Win == r.W3)
	r.Chk("follow focuses main", s.Active == 0)

	// G: toggle off (stay, restore)
	r.D("toggle", r.CL)
	sleep(600)
	r.Chk("sidebar left user windows", r.DemuxPanes("-t", r.W3) == 0)
	r.Chk("TUI pane gone entirely", r.Side().Pane == "")
	r.Chk("gamma layout exact", r.Layout(r.W3) == tail(r.LW3))
	r.Chk("client still on gamma", r.ClientWin() == r.W3)
	r.Chk("@demux_docked off work", r.ShowOpt("-t", "work", "-v", "@demux_docked") == "")
	r.Chk("work status-left restored", r.ShowOpt("-t", "work", "status-left") == "")
	r.Chk("no spacers remain", r.Spacers() == 0)

	// H: q mid-scrub unzooms in place, still docked
	r.D("toggle", r.CL)
	sleep(600)
	sp = r.Side().Pane
	r.SendKeys(sp, "k", "k")
	sleep(500)
	r.Chk("scrubbing (zoomed)", r.Side().Width == 200)
	r.SendKeys(sp, "q")
	sleep(500)
	s = r.Side()
	r.Chk("q never moved the client", r.ClientWin() == r.W3)
	r.Chk("q unzoomed, still docked", s.Win == r.W3 && s.Width == 40)
	r.Chk("gamma unzoomed", !r.Zoomed(r.W3))
	r.Chk("home unzoom went through respawn", r.LogHas("unzoom=respawn"))
	r.Chk("fresh TUI repainted the list", strings.Contains(r.Capture(s.Pane), "work"))

	// undock keeps the pane the user is IN, not the dock-time active one:
	// dock lands focused on w1's right main, user moves to the left main,
	// M-s — focus must stay left.
	leftMain, rightMain := "", ""
	for _, ln := range strings.Split(r.T("list-panes", "-t", r.W1, "-F", "#{pane_id} #{pane_left} #{pane_current_command}"), "\n") {
		f := strings.Fields(ln)
		if len(f) != 3 || strings.Contains(f[2], "demux") {
			continue
		}
		if f[1] == "0" || f[1] == "41" {
			leftMain = f[0]
		} else {
			rightMain = f[0]
		}
	}
	r.T("select-pane", "-t", rightMain) // w1's active while hidden
	r.T("select-window", "-t", r.W1)
	r.WaitUntil(100, func() bool { return r.Side().Win == r.W1 })
	sleep(300)
	r.T("select-pane", "-t", leftMain) // the user moves
	sleep(300)
	r.D("toggle", r.CL)
	sleep(600)
	active := r.T("display-message", "-p", "-t", r.W1, "#{pane_id}")
	r.Chk("undock keeps the user's pane", active == leftMain)
}
