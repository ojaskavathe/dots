package rigs

import (
	"strings"
	"testing"
)

// TestBrowse: the full-screen browse surface, and browse-from-docked
// auto-undock.
func TestBrowse(t *testing.T) {
	r := New(t)

	// K: full-screen browse
	r.D("browse", r.CL)
	sleep(800)
	r.Chk("client on _demux", r.ClientSess() == "_demux")
	tp, tw := "", ""
	for _, ln := range strings.Split(r.T("list-panes", "-s", "-t", "_demux", "-F",
		"#{pane_id} #{pane_current_command} #{pane_width}"), "\n") {
		f := strings.Fields(ln)
		if len(f) == 3 && strings.Contains(f[1], "demux") {
			tp, tw = f[0], f[2]
			break
		}
	}
	r.Chk("tui full width", tw == "200")
	r.Chk("wide mode has border", strings.Contains(r.Capture(tp), "│"))
	r.SendKeys(tp, "q")
	sleep(600)
	r.Chk("q leaves browse", r.ClientSess() != "_demux")

	// L: browse from docked auto-undocks
	r.D("toggle", r.CL)
	sleep(500)
	pinWin := r.ClientWin()
	r.D("browse", r.CL)
	sleep(800)
	r.Chk("browse took over", r.ClientSess() == "_demux")
	r.Chk("dock auto-undocked", r.DemuxPanes("-t", pinWin) == 0)
	r.Chk("no spacers remain", r.Spacers() == 0)
	tp2 := ""
	for _, ln := range strings.Split(r.T("list-panes", "-s", "-t", "_demux", "-F",
		"#{pane_id} #{pane_current_command}"), "\n") {
		f := strings.Fields(ln)
		if len(f) == 2 && strings.Contains(f[1], "demux") {
			tp2 = f[0]
			break
		}
	}
	r.SendKeys(tp2, "q")
	sleep(600)
	r.Chk("q from browse returns", r.ClientSess() != "_demux")
}
