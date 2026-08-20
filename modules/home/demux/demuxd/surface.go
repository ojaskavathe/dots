package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// The surface: the TUI pane and the hidden "_demux" session that is its
// home. Both presentation modes share it — docked mode joins the pane into
// user windows as the 40-col sidebar, full-screen browse switches the client
// to its home window. This file owns only the pane's lifecycle; the modes
// live in dock.go and browse.go.

type surface struct {
	sess string // $id of the _demux session
	win  string // @id of the TUI's home window
	pane string // the single pane running `demuxd tui`
}

const listWidth = 40

const demuxSession = "_demux"

// placeholderCmd keeps the _demux session alive while the TUI pane is
// docked in a user window — moving a session's only pane out kills it
// (rig-verified), and the undock target must exist.
const placeholderCmd = "sleep 100000000"

// ensureSurface verifies the TUI pane is alive (wherever it currently lives
// — its home window, or docked in a user window) and that the _demux holding
// session can receive it back, rebuilding whatever is missing. The session
// is created at the summoning client's size: entering a differently-sized
// window makes tmux rescale it, which crushes the list pane.
func (d *daemon) ensureSurface(ctl *control, cw, ch int) error {
	if cw < listWidth+10 || ch < 5 {
		cw, ch = 200, 50
	}
	if d.sur != nil {
		alive := false
		nDemuxWins := 0
		lines, err := ctl.run("list-panes -a -F " + f("#{pane_id}", "#{pane_current_command}", "#{session_name}", "#{window_id}"))
		if err == nil {
			wins := map[string]bool{}
			for _, ln := range lines {
				p := strings.Split(ln, sep)
				if len(p) != 4 {
					continue
				}
				if p[0] == d.sur.pane && strings.Contains(p[1], "demux") {
					alive = true
				}
				if p[2] == demuxSession && !wins[p[3]] {
					wins[p[3]] = true
					nDemuxWins++
				}
			}
		}
		if alive {
			switch {
			case nDemuxWins == 0:
				// Holding session died with the TUI docked out; recreate it.
				out, err := ctl.run(fmt.Sprintf("new-session -d -s %s -x %d -y %d -P -F %s %s",
					demuxSession, cw, ch, f("#{session_id}", "#{window_id}"), q(placeholderCmd)))
				if err != nil {
					return err
				}
				if len(out) == 1 {
					if p := strings.Split(out[0], sep); len(p) == 2 {
						d.sur.sess, d.sur.win = p[0], p[1]
					}
				}
				_, _ = ctl.run("set-option -q -t " + q(d.sur.sess) + " status off")
			case nDemuxWins == 1 && d.dock == nil:
				// TUI home is the only window: docking it away would kill the
				// session. Keep the placeholder ahead of need.
				_, _ = ctl.run("new-window -d -t " + q(demuxSession+":") + " " + q(placeholderCmd))
			}
			return nil
		}
		d.dropSurface()
	}
	_, _ = ctl.run("kill-session -t " + q(demuxSession)) // stale leftovers; may not exist
	self, err := os.Executable()
	if err != nil {
		return err
	}
	tuiCmd := self + " -S " + d.tmuxSock + " tui"
	if bench {
		tuiCmd = "env DEMUX_BENCH=1 " + tuiCmd
	}
	lines, err := ctl.run(fmt.Sprintf("new-session -d -s %s -x %d -y %d -P -F %s %s",
		demuxSession, cw, ch, f("#{session_id}", "#{window_id}", "#{pane_id}"), q(tuiCmd)))
	if err != nil {
		return err
	}
	if len(lines) == 0 {
		return errors.New("new-session returned nothing")
	}
	p := strings.Split(lines[0], sep)
	if len(p) != 3 {
		return errors.New("bad new-session reply")
	}
	sur := &surface{sess: p[0], win: p[1], pane: p[2]}
	_, err = ctl.runSeq(
		"new-window -d -t "+q(demuxSession+":")+" "+q(placeholderCmd),
		"set-option -wq -t "+q(sur.win)+" automatic-rename off",
		"set-option -q -t "+q(sur.sess)+" status off")
	if err != nil {
		return err
	}
	d.sur = sur
	return nil
}

// dropSurface forgets a dead TUI pane and every bit of mode state that
// referenced it — a fresh surface starts from nothing.
func (d *daemon) dropSurface() {
	d.sur = nil
	d.browse = browseState{}
	d.stopStream()
	d.pv.target, d.pv.lastFrame = "", nil
}
