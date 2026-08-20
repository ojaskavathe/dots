package rigs

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRecovery: hosting window killed, user splits while docked, daemon
// restart sweeping leaked spacers (restart LAST: its reattach gap would
// flake anything racing it).
func TestRecovery(t *testing.T) {
	r := New(t)

	// I: kill hosting window recovers
	r.D("toggle", r.CL)
	sleep(600)
	r.T("kill-window", "-t", r.ClientWin())
	sleep(800)
	r.Chk("dock state cleaned", r.ShowOpt("-t", "play", "-v", "@demux_docked") == "" &&
		r.ShowOpt("-t", "work", "-v", "@demux_docked") == "")
	r.D("toggle", r.CL)
	sleep(800)
	s := r.Side()
	r.Chk("sidebar re-docked", s.Left == 0 && s.Width == 40)
	r.D("toggle", r.CL)
	sleep(400)

	// J: user splits while docked; undock restore fails cleanly
	r.D("toggle", r.CL)
	sleep(600)
	cur := r.ClientWin()
	np0 := len(strings.Split(r.T("list-panes", "-t", cur, "-F", "x"), "\n")) - r.DemuxPanes("-t", cur)
	// Kill by the split's own id later — pane LIST order is an artifact
	// (split-window -b inserts first where join-pane -b appended), so
	// "last listed" is not "the pane this test created".
	userSplit := r.T("split-window", "-v", "-P", "-F", "#{pane_id}", "-t", cur)
	sleep(300)
	r.D("toggle", r.CL)
	sleep(600)
	r.Chk("user split survives undock", len(strings.Split(r.T("list-panes", "-t", cur, "-F", "x"), "\n")) == np0+1)
	r.Chk("no sidebar left behind", r.DemuxPanes("-t", cur) == 0)
	r.Chk("stale restore logged", r.LogHas("restore layout|undock:"))
	r.TQ("kill-pane", "-t", userSplit)

	// S: daemon restart sweeps leaked spacers
	r.T("split-window", "-d", "-hb", "-f", "-l", "40", "-t", r.W3, "sleep 100000001") // fake a leak
	exec.Command("pkill", "-f", filepath.Base(demuxdBin)+" -S /private/tmp/tmux-501/"+r.L+" run").Run()
	sleep(500)
	r.D("ls")
	sleep(2000)
	r.Chk("leaked spacer swept", r.Spacers() == 0)
	r.Chk("gamma layout intact", r.Layout(r.W3) == tail(r.LW3))
	err := exec.Command("pgrep", "-f", filepath.Base(demuxdBin)+" -S /private/tmp/tmux-501/"+r.L+" run").Run()
	r.Chk("daemon alive", err == nil)
}
