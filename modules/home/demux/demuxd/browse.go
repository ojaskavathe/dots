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
	sess string // $id of the _demux session
	win  string // @id of the browse window
	pane string // the single full-window pane running `demuxd tui`

	open       bool
	client     string // browsing client (explicit switching)
	originSess string // where q returns to
	originWin  string
	target     string // currently previewed window
	lastFrame  []byte // last target frame, replayed to late-connecting TUIs
}

type daemon struct {
	tmuxSock string
	h        *hub
	br       *browseState
	pin      *pinState

	// lastScrub gates re-lists: world churn within scrubQuiet of a pin
	// scrub is our own doing, and re-listing the whole server once per
	// scrub step is the daemon's main cost during a held-key scrub.
	lastScrub time.Time

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

// handleCmd drains everything already queued and executes only what still
// matters: every non-preview command in order, the NEWEST real preview, and
// the prefetches that came after it (they describe the final position).
// Stale previews and stale prefetches are acked without running — during a
// fast scrub they'd otherwise serialize ahead of the frame the user is
// actually looking at. Remaining prefetches are abandoned the moment fresher
// input arrives.
func (d *daemon) handleCmd(ctl *control, env cmdEnvelope) {
	batch := []cmdEnvelope{env}
	for {
		select {
		case next := <-d.h.cmds:
			batch = append(batch, next)
			continue
		default:
		}
		break
	}
	lastReal := -1
	for i, e := range batch {
		if e.msg.Cmd == "preview" && !e.msg.Prefetch {
			lastReal = i
		}
	}
	for i, e := range batch {
		isPreview := e.msg.Cmd == "preview"
		switch {
		case !isPreview:
			d.runCmd(ctl, e)
		case !e.msg.Prefetch && i == lastReal:
			d.runCmd(ctl, e)
		case e.msg.Prefetch && i > lastReal:
			if len(d.h.cmds) > 0 {
				// Fresher input queued: this prefetch is already history.
				d.h.send(e.sub, marshalLine(replyMsg{Type: "reply", OK: true}))
				continue
			}
			d.runCmd(ctl, e)
		default:
			d.h.send(e.sub, marshalLine(replyMsg{Type: "reply", OK: true}))
		}
	}
}

var bench = os.Getenv("DEMUX_BENCH") != ""

func (d *daemon) runCmd(ctl *control, env cmdEnvelope) {
	start := time.Now()
	browsing := d.br != nil && d.br.open
	var err error
	switch env.msg.Cmd {
	case "toggle":
		err = d.toggle(ctl, env.msg.Client)
	case "browse":
		err = d.browseOpen(ctl, env.msg.Client)
	case "nav":
		err = d.pinNav(ctl, env.msg.Dir)
	case "preview":
		if d.pin != nil && !browsing {
			// Pinned scrub: the "preview" moves the real main area. Prefetch
			// is a billboard concept — acked, never switches anything.
			if !env.msg.Prefetch {
				err = d.pinScrub(ctl, env.msg.Window, false)
			}
		} else {
			err = d.preview(ctl, env.msg.Window, env.msg.Prefetch)
		}
	case "commit":
		if d.pin != nil && !browsing {
			err = d.pinCommit(ctl)
		} else {
			err = d.commit(ctl, env.msg.Window)
		}
	case "close":
		if d.pin != nil && !browsing {
			err = d.pinClose(ctl, true)
		} else {
			err = d.closeBrowse(ctl)
		}
	case "hello-list":
		// A TUI connected after state went out; replay selection + frame.
		if d.pin != nil {
			d.h.send(env.sub, marshalLine(selectMsg{Type: "select", Window: d.pin.win}))
			log.Printf("hello-list: replay pinned select=%s", d.pin.win)
			break
		}
		replaySel := browsing && d.br.target != ""
		if replaySel {
			d.h.send(env.sub, marshalLine(selectMsg{Type: "select", Window: d.br.target}))
		}
		log.Printf("hello-list: replay select=%v target=%s frame=%v",
			replaySel, brTarget(d.br), d.br != nil && d.br.lastFrame != nil)
		if d.br != nil && d.br.lastFrame != nil {
			d.h.send(env.sub, d.br.lastFrame)
		}
	default:
		err = fmt.Errorf("unknown cmd %q", env.msg.Cmd)
	}
	if dur := time.Since(start); dur > 25*time.Millisecond {
		log.Printf("%s took %s", env.msg.Cmd, dur)
	} else if bench {
		log.Printf("bench cmd=%s prefetch=%v dur_us=%d", env.msg.Cmd, env.msg.Prefetch, dur.Microseconds())
	}
	r := replyMsg{Type: "reply", OK: err == nil}
	if err != nil {
		r.Err = err.Error()
	}
	d.h.send(env.sub, marshalLine(r))
}

// toggle is M-s: browsing (full-screen) -> commit/close as before; pinned ->
// undock in place; otherwise dock the sidebar into the current window.
func (d *daemon) toggle(ctl *control, client string) error {
	if client == "" {
		return errors.New("toggle needs a client name")
	}
	if d.br != nil && d.br.open {
		sid, _, _, _, err := d.clientView(ctl, client)
		if err == nil && sid == d.br.sess {
			d.br.client = client
			// M-s while browsing commits to the current selection, like Enter —
			// muscle memory from the join-sidebar era. q / Ctrl-C remain
			// cancel-to-origin.
			if d.br.target != "" && d.br.target != d.br.originWin {
				log.Printf("toggle-off commits to %s", d.br.target)
				return d.commit(ctl, d.br.target)
			}
			return d.closeBrowse(ctl)
		}
	}
	if d.pin != nil {
		return d.pinClose(ctl, false)
	}
	return d.pinOpen(ctl, client)
}

// browseOpen is the full-screen billboard browser (`demuxd browse`): switch
// the client to the _demux window and stream capture frames. No longer on
// M-s — pinned mode replaced it there — but fully functional.
func (d *daemon) browseOpen(ctl *control, client string) error {
	if client == "" {
		return errors.New("browse needs a client name")
	}
	if d.pin != nil {
		if err := d.pinClose(ctl, false); err != nil {
			return err
		}
	}
	sid, wid, cw, ch, err := d.clientView(ctl, client)
	if err != nil {
		return err
	}
	if d.br != nil && d.br.open && sid == d.br.sess {
		return nil // already browsing
	}
	if err := d.ensureSurface(ctl, cw, ch); err != nil {
		return err
	}
	d.br.open = true
	d.br.client = client
	d.br.originSess, d.br.originWin = sid, wid
	// Selection first: the list repaints while the browse window is still
	// hidden, so it is already on the origin row when the client lands. A
	// freshly spawned TUI isn't connected yet (n=0) — the hello-list replay
	// covers that; n=0 on a WARM browse means the select was lost.
	n := d.h.sendRole("list", marshalLine(selectMsg{Type: "select", Window: wid}))
	log.Printf("browse open client=%s origin=%s/%s size=%dx%d select_receivers=%d", client, sid, wid, cw, ch, n)
	if _, err := ctl.runSeq(
		"select-window -t "+q(d.br.win),
		"switch-client -c "+q(client)+" -t "+q(d.br.sess)); err != nil {
		return err
	}
	d.startStream()
	return d.preview(ctl, wid, false)
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
	if d.br != nil {
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
				if p[0] == d.br.pane && strings.Contains(p[1], "demux") {
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
						d.br.sess, d.br.win = p[0], p[1]
					}
				}
				_, _ = ctl.run("set-option -q -t " + q(d.br.sess) + " status off")
			case nDemuxWins == 1 && d.pin == nil:
				// TUI home is the only window: docking it away would kill the
				// session. Keep the placeholder ahead of need.
				_, _ = ctl.run("new-window -d -t " + q(demuxSession+":") + " " + q(placeholderCmd))
			}
			return nil
		}
		d.br = nil
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
	br := &browseState{sess: p[0], win: p[1], pane: p[2]}
	_, err = ctl.runSeq(
		"new-window -d -t "+q(demuxSession+":")+" "+q(placeholderCmd),
		"set-option -wq -t "+q(br.win)+" automatic-rename off",
		"set-option -q -t "+q(br.sess)+" status off")
	if err != nil {
		return err
	}
	d.br = br
	return nil
}

// preview captures the target window and ships it to the canvas as a frame.
// Geometry is queried fresh (cross-session geometry emits no events), then
// every pane is captured in one sequence with marker lines between panes:
// capture output line counts are not reliable (trailing blanks), markers are.
const frameMarker = "\x1fdemux-frame\x1f"

// preview captures wid and ships it as a frame. A prefetch warms the TUI's
// cache for adjacent rows without becoming the streamed target.
func (d *daemon) preview(ctl *control, wid string, prefetch bool) error {
	if d.br == nil || !d.br.open {
		return nil
	}
	lines, err := ctl.run("list-panes -t " + q(wid) + " -F " +
		f("#{pane_id}", "#{pane_left}", "#{pane_top}", "#{pane_width}", "#{pane_height}", "#{pane_active}"))
	if err != nil {
		return err
	}
	var panes []framePane
	var caps []string
	for _, ln := range lines {
		p := strings.Split(ln, sep)
		if len(p) != 6 {
			continue
		}
		left, _ := strconv.Atoi(p[1])
		top, _ := strconv.Atoi(p[2])
		width, _ := strconv.Atoi(p[3])
		height, _ := strconv.Atoi(p[4])
		panes = append(panes, framePane{Left: left, Top: top, Width: width, Height: height, Active: p[5] == "1"})
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
	payload := marshalLine(frameMsg{Type: "frame", Window: wid, Panes: panes})
	if prefetch {
		d.h.sendRole("list", payload)
		return nil
	}
	d.br.target = wid
	if bytes.Equal(payload, d.br.lastFrame) {
		return nil // idle content: no repaint
	}
	d.br.lastFrame = payload
	d.h.sendRole("list", payload)
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
		log.Printf("commit target=%s unknown, closing to origin", wid)
		return d.closeBrowse(ctl)
	}
	d.br.open = false
	d.stopStream()
	log.Printf("commit client=%s -> %s/%s", d.br.client, sid, wid)
	_, err := ctl.runSeq(
		"select-window -t "+q(wid),
		"switch-client -c "+q(d.br.client)+" -t "+q(sid))
	return err
}

func brTarget(br *browseState) string {
	if br == nil {
		return "<nil>"
	}
	return br.target
}

// closeBrowse returns the client to where it was when it summoned.
func (d *daemon) closeBrowse(ctl *control) error {
	if d.br == nil || !d.br.open {
		return nil
	}
	d.br.open = false
	d.stopStream()
	log.Printf("close client=%s -> origin %s/%s", d.br.client, d.br.originSess, d.br.originWin)
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
// window by other means (detach, manual switch), browsing is over.
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
	}
}

// q quotes an argument for tmux's command parser. Everything we pass (ids,
// client names, store paths) is shell-tame; the quotes guard spaces only.
func q(s string) string {
	return "'" + s + "'"
}
