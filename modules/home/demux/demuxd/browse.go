package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// The browse surface: choose-tree mechanics, our UI. A persistent
// demux-owned window (list pane + preview canvas) lives in a hidden
// "_demux" session. M-s switches the client TO it; scrolling paints
// capture-pane frames into the canvas; Enter switches the client to the
// target; q returns to origin. User windows are never joined into, resized,
// renamed, or otherwise touched — the entire baseline/restore problem class
// of the join-based sidebar (git history: f2538c2) does not exist here, and
// scrubbing costs one capture round trip instead of a window-wide reflow.
//
// Rules carried from the spike, still load-bearing:
//   - sequences abort at the first error -> critical commands first, targets
//     validated against the live model
//   - all client switching is explicit (-c): implicit switch-client picks
//     the wrong client whenever more than one is attached (the daemon's own
//     control client always is)
//   - `display-message -p -c X` does NOT expand formats in X's context;
//     per-client truth comes from list-clients rows

const listWidth = 40

const demuxSession = "_demux"

type browseState struct {
	sess       string // $id of the _demux session
	win        string // @id of the browse window
	listPane   string
	canvasPane string

	open       bool
	client     string // browsing client (explicit switching)
	originSess string // where q returns to
	originWin  string
	target     string // currently previewed window
	lastFrame  []byte // last frame sent, replayed to late-connecting canvases
}

type daemon struct {
	tmuxSock string
	h        *hub
	br       *browseState

	// The preview stream: while browsing, the selected window is re-captured
	// on this ticker (10fps) so previews stay LIVE — scrolling logs, agent
	// output. Frames are dropped when content hasn't changed, and the ticker
	// exists only while the browse surface is open (tickC is nil otherwise,
	// so the select case simply never fires). %output can narrow this later
	// for same-session targets; cross-session panes emit no output events at
	// all, so capture is the only universal source.
	ticker *time.Ticker
	tickC  <-chan time.Time
}

func (d *daemon) startStream() {
	if d.ticker == nil {
		d.ticker = time.NewTicker(100 * time.Millisecond)
		d.tickC = d.ticker.C
	}
}

func (d *daemon) stopStream() {
	if d.ticker != nil {
		d.ticker.Stop()
		d.ticker = nil
		d.tickC = nil
	}
}

func (d *daemon) handleCmd(ctl *control, env cmdEnvelope) {
	// Coalesce a preview backlog: only the newest target matters.
	if env.msg.Cmd == "preview" {
		for {
			select {
			case next := <-d.h.cmds:
				if next.msg.Cmd == "preview" {
					d.h.send(env.sub, marshalLine(replyMsg{Type: "reply", OK: true}))
					env = next
					continue
				}
				d.runCmd(ctl, env)
				env = next
			default:
			}
			break
		}
	}
	d.runCmd(ctl, env)
}

func (d *daemon) runCmd(ctl *control, env cmdEnvelope) {
	start := time.Now()
	var err error
	switch env.msg.Cmd {
	case "toggle":
		err = d.toggle(ctl, env.msg.Client)
	case "preview":
		err = d.preview(ctl, env.msg.Window)
	case "commit":
		err = d.commit(ctl, env.msg.Window)
	case "close":
		err = d.closeBrowse(ctl)
	case "hello-canvas":
		// A canvas connected after the last frame went out; replay it.
		if d.br != nil && d.br.lastFrame != nil {
			d.h.send(env.sub, d.br.lastFrame)
		}
	case "hello-list":
		if d.br != nil && d.br.open && d.br.target != "" {
			d.h.send(env.sub, marshalLine(selectMsg{Type: "select", Window: d.br.target}))
		}
	default:
		err = fmt.Errorf("unknown cmd %q", env.msg.Cmd)
	}
	if dur := time.Since(start); dur > 25*time.Millisecond {
		log.Printf("%s took %s", env.msg.Cmd, dur)
	}
	r := replyMsg{Type: "reply", OK: err == nil}
	if err != nil {
		r.Err = err.Error()
	}
	d.h.send(env.sub, marshalLine(r))
}

// toggle: not browsing -> remember origin and switch to the browse window;
// already browsing (M-s again) -> return to origin.
func (d *daemon) toggle(ctl *control, client string) error {
	if client == "" {
		return errors.New("toggle needs a client name")
	}
	sid, wid, cw, ch, err := d.clientView(ctl, client)
	if err != nil {
		return err
	}
	if d.br != nil && d.br.open && sid == d.br.sess {
		d.br.client = client
		return d.closeBrowse(ctl)
	}
	if err := d.ensureBrowse(ctl, cw, ch); err != nil {
		return err
	}
	d.br.open = true
	d.br.client = client
	d.br.originSess, d.br.originWin = sid, wid
	// The list width is re-asserted after the client lands: entering a
	// window resizes it to the client, which rescales panes proportionally.
	if _, err := ctl.runSeq(
		"select-pane -t "+q(d.br.listPane),
		"switch-client -c "+q(client)+" -t "+q(d.br.sess),
		"resize-pane -t "+q(d.br.listPane)+" -x "+strconv.Itoa(listWidth)); err != nil {
		return err
	}
	d.startStream()
	// Land the list selection on where the user came from, and show it.
	d.h.sendRole("list", marshalLine(selectMsg{Type: "select", Window: wid}))
	return d.preview(ctl, wid)
}

// clientView: the client's current session, window, and size, from
// list-clients rows (each row expands in that client's own context).
func (d *daemon) clientView(ctl *control, client string) (sid, wid string, cw, ch int, err error) {
	lines, err := ctl.run("list-clients -F " + f("#{client_name}", "#{session_id}", "#{window_id}", "#{client_width}", "#{client_height}"))
	if err != nil {
		return "", "", 0, 0, err
	}
	for _, ln := range lines {
		p := strings.Split(ln, sep)
		if len(p) == 5 && p[0] == client {
			cw, _ = strconv.Atoi(p[3])
			ch, _ = strconv.Atoi(p[4])
			return p[1], p[2], cw, ch, nil
		}
	}
	return "", "", 0, 0, errors.New("no client " + client)
}

// ensureBrowse verifies the browse window is intact (both panes alive and
// running us), rebuilding it from scratch otherwise — which also collects
// any stale _demux session resurrect may have restored. The session is
// created at the summoning client's size: entering a differently-sized
// window makes tmux rescale it, which crushes the list pane.
func (d *daemon) ensureBrowse(ctl *control, cw, ch int) error {
	if d.br != nil {
		lines, err := ctl.run("list-panes -t " + q(d.br.win) + " -F " + f("#{pane_id}", "#{pane_current_command}"))
		if err == nil {
			alive := 0
			for _, ln := range lines {
				p := strings.Split(ln, sep)
				if len(p) != 2 {
					continue
				}
				if (p[0] == d.br.listPane || p[0] == d.br.canvasPane) && strings.Contains(p[1], "demux") {
					alive++
				}
			}
			if alive == 2 {
				return nil
			}
		}
		d.br = nil
	}
	_, _ = ctl.run("kill-session -t " + q(demuxSession)) // stale leftovers; may not exist
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if cw < listWidth+10 || ch < 5 {
		cw, ch = 200, 50
	}
	lines, err := ctl.run(fmt.Sprintf("new-session -d -s %s -x %d -y %d -P -F %s %s",
		demuxSession, cw, ch, f("#{session_id}", "#{window_id}", "#{pane_id}"), q(self+" -S "+d.tmuxSock+" tui")))
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
	br := &browseState{sess: p[0], win: p[1], listPane: p[2]}
	lines, err = ctl.runSeq(
		fmt.Sprintf("split-window -hd -t %s -P -F '#{pane_id}' %s",
			q(br.listPane), q(self+" -S "+d.tmuxSock+" canvas")),
		"resize-pane -t "+q(br.listPane)+" -x "+strconv.Itoa(listWidth),
		"set-option -wq -t "+q(br.win)+" automatic-rename off",
		"set-option -q -t "+q(br.sess)+" status off")
	if err != nil {
		return err
	}
	if len(lines) == 0 {
		return errors.New("split-window returned no pane id")
	}
	br.canvasPane = strings.TrimSpace(lines[0])
	d.br = br
	return nil
}

// preview captures the target window and ships it to the canvas as a frame.
// Geometry is queried fresh (cross-session geometry emits no events), then
// every pane is captured in one sequence with marker lines between panes:
// capture output line counts are not reliable (trailing blanks), markers are.
const frameMarker = "\x1fdemux-frame\x1f"

func (d *daemon) preview(ctl *control, wid string) error {
	if d.br == nil || !d.br.open {
		return nil
	}
	lines, err := ctl.run("list-panes -t " + q(wid) + " -F " +
		f("#{pane_id}", "#{pane_left}", "#{pane_top}", "#{pane_width}", "#{pane_height}"))
	if err != nil {
		return err
	}
	var panes []framePane
	var caps []string
	for _, ln := range lines {
		p := strings.Split(ln, sep)
		if len(p) != 5 {
			continue
		}
		left, _ := strconv.Atoi(p[1])
		top, _ := strconv.Atoi(p[2])
		width, _ := strconv.Atoi(p[3])
		height, _ := strconv.Atoi(p[4])
		panes = append(panes, framePane{Left: left, Top: top, Width: width, Height: height})
		caps = append(caps, "capture-pane -e -p -t "+q(p[0]), "display-message -p "+q(frameMarker))
	}
	if len(panes) == 0 {
		return fmt.Errorf("no panes in %s", wid)
	}
	out, err := ctl.runSeq(caps...)
	if err != nil {
		return err
	}
	idx := 0
	for _, ln := range out {
		if ln == frameMarker {
			idx++
			continue
		}
		if idx < len(panes) {
			panes[idx].Lines = append(panes[idx].Lines, ln)
		}
	}
	d.br.target = wid
	payload := marshalLine(frameMsg{Type: "frame", Window: wid, Panes: panes})
	if bytes.Equal(payload, d.br.lastFrame) {
		return nil // idle content: no repaint
	}
	d.br.lastFrame = payload
	d.h.sendRole("canvas", payload)
	return nil
}

// commit: switch the client to the chosen window for real. The browse window
// stays alive for next time.
func (d *daemon) commit(ctl *control, wid string) error {
	if d.br == nil || !d.br.open {
		return nil
	}
	sid := ""
	for _, w := range d.h.getWorld().Windows {
		if w.ID == wid {
			sid = w.SessionID
			break
		}
	}
	if sid == "" {
		return d.closeBrowse(ctl)
	}
	d.br.open = false
	d.stopStream()
	_, err := ctl.runSeq(
		"select-window -t "+q(wid),
		"switch-client -c "+q(d.br.client)+" -t "+q(sid))
	return err
}

// closeBrowse returns the client to where it was when it summoned.
func (d *daemon) closeBrowse(ctl *control) error {
	if d.br == nil || !d.br.open {
		return nil
	}
	d.br.open = false
	d.stopStream()
	_, err := ctl.runSeq(
		"select-window -t "+q(d.br.originWin),
		"switch-client -c "+q(d.br.client)+" -t "+q(d.br.originSess))
	if err != nil {
		// Origin may have died while browsing; session alone, then give up
		// gracefully (the user can switch by hand).
		_, err = ctl.run("switch-client -c " + q(d.br.client) + " -t " + q(d.br.originSess))
	}
	return err
}

// checkBrowse runs after every re-list: if the client escaped the browse
// window by other means (detach, manual switch), browsing is over; and the
// list width is re-asserted if a window rescale (client entry, monitor
// change) crushed it.
func (d *daemon) checkBrowse(ctl *control, w world) {
	if d.br == nil || !d.br.open {
		return
	}
	found := false
	for _, c := range w.Clients {
		if c.Name == d.br.client {
			found = true
			if c.SessionID != d.br.sess {
				d.br.open = false
				d.stopStream()
				return
			}
			break
		}
	}
	if !found {
		d.br.open = false // client detached
		d.stopStream()
		return
	}
	for _, p := range w.Panes {
		if p.ID == d.br.listPane && p.Width != listWidth {
			_, _ = ctl.run("resize-pane -t " + q(d.br.listPane) + " -x " + strconv.Itoa(listWidth))
			return
		}
	}
}

// q quotes an argument for tmux's command parser. Everything we pass (ids,
// client names, store paths) is shell-tame; the quotes guard spaces only.
func q(s string) string {
	return "'" + s + "'"
}
