package main

import (
	"errors"
	"fmt"
	"log"
	"strings"
)

// Pinned mode: the herdr layout on tmux. The TUI pane docks as a real 40-col
// pane at the left edge of the client's current window; the main area is the
// user's actual panes — live, focusable, typable. Scrubbing the list moves
// the MAIN AREA for real (join sidebar into the target window + select it in
// one sequence), so what you see while scrolling is the window itself, not a
// capture. M-s toggles the dock; Enter drops focus into the main pane; q
// undocks and returns to the origin window.
//
// Rig-verified (tmux 3.7b, scratchpad/rig-pin*.zsh):
//   - `join-pane -hb -f -l 40` lands at the window's left edge no matter
//     which pane is targeted, and the joined pane takes focus (no -d)
//   - undocking hands the sidebar's 40 cols to the ADJACENT pane, not back
//     proportionally — layout restore from a saved #{window_layout} string is
//     mandatory, and restores exactly
//   - select-layout with a stale string (user split while pinned) fails
//     cleanly; restores therefore run after the critical commands
//   - moving the only pane out of _demux kills the session — a placeholder
//     window keeps it alive as the undock target
//
// The old full-screen billboard browse (list + capture-pane canvas) is still
// in browse.go, reachable via `demuxd browse`, just no longer on M-s.

// winSnap is everything to put a window back the way it was before the
// sidebar entered it. Queried at point-of-use, never from the cached world:
// cross-session geometry only refreshes on re-list.
type winSnap struct {
	layout     string
	activePane string
	autoRename string // effective automatic-rename, frozen while docked
	name       string
}

type pinState struct {
	client     string
	win        string // window currently hosting the sidebar
	sess       string // session of win (tracks @demux_pinned + status pad)
	originSess string // where q returns to
	originWin  string
	snap       winSnap // pre-dock snapshot of win

	statusPrev    string // raw `show-options` lines to replay on restore,
	statusLenPrev string // empty = option was not set at session level
}

// statusPad shifts the status line's content past the sidebar column:
// 40 cols of pane + 1 col of pane border.
const statusPad = listWidth + 1

func (d *daemon) winSnapshot(ctl *control, wid string) (winSnap, error) {
	lines, err := ctl.run("display-message -p -t " + q(wid) + " -F " +
		f("#{window_layout}", "#{pane_id}", "#{automatic-rename}", "#{window_name}"))
	if err != nil {
		return winSnap{}, err
	}
	if len(lines) == 0 {
		return winSnap{}, fmt.Errorf("no snapshot for %s", wid)
	}
	p := strings.Split(lines[0], sep)
	if len(p) != 4 {
		return winSnap{}, fmt.Errorf("bad snapshot for %s", wid)
	}
	return winSnap{layout: p[0], activePane: p[1], autoRename: p[2], name: p[3]}, nil
}

// sessionOf resolves a window's session from the world model.
func (d *daemon) sessionOf(wid string) string {
	for _, w := range d.h.getWorld().Windows {
		if w.ID == wid {
			return w.SessionID
		}
	}
	return ""
}

// pinOpen docks the sidebar into the client's current window.
func (d *daemon) pinOpen(ctl *control, client string) error {
	if client == "" {
		return errors.New("pin needs a client name")
	}
	sid, wid, cw, ch, err := d.clientView(ctl, client)
	if err != nil {
		return err
	}
	if err := d.ensureSurface(ctl, cw, ch); err != nil {
		return err
	}
	snap, err := d.winSnapshot(ctl, wid)
	if err != nil {
		return err
	}
	p := &pinState{client: client, win: wid, sess: sid, originSess: sid, originWin: wid, snap: snap}
	d.savedStatus(ctl, p, sid)
	// Freeze rename BEFORE the join: the sidebar takes focus on join, and an
	// automatic-rename window would flip its name to "demuxd" (the sh-era bug).
	seq := []string{
		"set-option -w -t " + q(wid) + " automatic-rename off",
		fmt.Sprintf("join-pane -hb -f -l %d -s %s -t %s", listWidth, q(d.br.pane), q(wid)),
	}
	seq = append(seq, pinSessionCmds(sid)...)
	if _, err := ctl.runSeq(seq...); err != nil {
		d.pin = nil
		return err
	}
	d.pin = p
	n := d.h.sendRole("list", marshalLine(selectMsg{Type: "select", Window: wid}))
	log.Printf("pin open client=%s win=%s/%s size=%dx%d select_receivers=%d", client, sid, wid, cw, ch, n)
	return nil
}

// pinScrub moves the main area to wid: the sidebar rides along in the same
// sequence, so the target window is never visible without it. focusMain puts
// the keyboard in the window's own pane (nav, follow); scrubbing from the
// list keeps focus in the sidebar so held-j keeps working.
func (d *daemon) pinScrub(ctl *control, wid string, focusMain bool) error {
	p := d.pin
	if p == nil || wid == "" || wid == p.win {
		return nil
	}
	sidN := d.sessionOf(wid)
	if sidN == "" {
		return fmt.Errorf("scrub target %s unknown", wid)
	}
	snapN, err := d.winSnapshot(ctl, wid)
	if err != nil {
		return err
	}
	seq := []string{
		"set-option -w -t " + q(wid) + " automatic-rename off",
		fmt.Sprintf("join-pane -hb -f -l %d -s %s -t %s", listWidth, q(d.br.pane), q(wid)),
		"select-window -t " + q(wid),
	}
	if sidN != p.sess {
		seq = append(seq, "switch-client -c "+q(p.client)+" -t "+q(sidN))
	}
	if focusMain {
		seq = append(seq, "select-pane -t "+q(snapN.activePane))
	}
	if _, err := ctl.runSeq(seq...); err != nil {
		return err
	}
	// Session bookkeeping (option + status pad) and restores for the window
	// we just left: best-effort, each on its own so one failure (stale
	// layout after a user split) doesn't abort the rest.
	if sidN != p.sess {
		d.restoreStatus(ctl, p) // old session, before p.sess moves on
		_, _ = ctl.run("set-option -u -t " + q(p.sess) + " @demux_pinned")
		d.savedStatus(ctl, p, sidN) // MUST precede the pad, or we save our own pad
		for _, c := range pinSessionCmds(sidN) {
			_, _ = ctl.run(c)
		}
	}
	d.restoreWin(ctl, p.win, p.snap)
	prev := p.win
	p.win, p.sess, p.snap = wid, sidN, snapN
	if bench {
		log.Printf("bench scrub %s -> %s focus_main=%v", prev, wid, focusMain)
	}
	return nil
}

// pinNav is the routed M-h / M-l: previous/next window of the current
// session with the sidebar riding along atomically.
func (d *daemon) pinNav(ctl *control, dir string) error {
	p := d.pin
	if p == nil {
		return errors.New("nav while not pinned")
	}
	var wins []window
	for _, w := range d.h.getWorld().Windows {
		if w.SessionID == p.sess {
			wins = append(wins, w)
		}
	}
	if len(wins) < 2 {
		return nil
	}
	cur := -1
	for i, w := range wins {
		if w.ID == p.win {
			cur = i
		}
	}
	if cur < 0 {
		return nil
	}
	step := 1
	if dir == "prev" {
		step = -1
	}
	target := wins[(cur+step+len(wins))%len(wins)].ID
	if err := d.pinScrub(ctl, target, true); err != nil {
		return err
	}
	d.h.sendRole("list", marshalLine(selectMsg{Type: "select", Window: target}))
	return nil
}

// pinCommit is Enter in the docked list: keep the sidebar, put the keyboard
// in the main area. The origin resets — q now returns here.
func (d *daemon) pinCommit(ctl *control) error {
	p := d.pin
	if p == nil {
		return nil
	}
	target := p.snap.activePane
	alive := false
	for _, pn := range d.h.getWorld().Panes {
		if pn.ID == target && pn.WindowID == p.win {
			alive = true
			break
		}
	}
	if !alive {
		target = ""
		for _, pn := range d.h.getWorld().Panes {
			if pn.WindowID == p.win && pn.ID != d.br.pane {
				target = pn.ID
				break
			}
		}
	}
	if target == "" {
		return errors.New("no main pane to focus")
	}
	p.originSess, p.originWin = p.sess, p.win
	log.Printf("pin commit focus=%s win=%s", target, p.win)
	_, err := ctl.run("select-pane -t " + q(target))
	return err
}

// pinClose undocks. toOrigin (q) returns the client to where it was pinned
// or last committed; otherwise (M-s) it stays put and the layout snaps back.
func (d *daemon) pinClose(ctl *control, toOrigin bool) error {
	p := d.pin
	if p == nil {
		return nil
	}
	d.pin = nil
	log.Printf("pin close client=%s win=%s to_origin=%v", p.client, p.win, toOrigin)
	if toOrigin && p.originWin != p.win {
		_, err := ctl.runSeq(
			"select-window -t "+q(p.originWin),
			"switch-client -c "+q(p.client)+" -t "+q(p.originSess))
		if err != nil {
			_, _ = ctl.run("switch-client -c " + q(p.client) + " -t " + q(p.originSess))
		}
	}
	d.undock(ctl)
	d.restoreWin(ctl, p.win, p.snap)
	_, _ = ctl.run("set-option -u -t " + q(p.sess) + " @demux_pinned")
	d.restoreStatus(ctl, p)
	return nil
}

// undock sends the TUI pane home to _demux (break-pane makes a fresh window
// there; the placeholder window kept the session alive meanwhile).
func (d *daemon) undock(ctl *control) {
	lines, err := ctl.run("break-pane -d -P -F " + f("#{session_id}", "#{window_id}") +
		" -s " + q(d.br.pane) + " -t " + q(demuxSession+":"))
	if err != nil {
		log.Printf("undock: %v", err)
		return
	}
	if len(lines) == 1 {
		if parts := strings.Split(lines[0], sep); len(parts) == 2 {
			d.br.sess, d.br.win = parts[0], parts[1]
			_, _ = ctl.run("set-option -wq -t " + q(d.br.win) + " automatic-rename off")
		}
	}
}

// restoreWin puts a window back the way winSnapshot found it. Best-effort:
// a stale layout (user split while docked) fails cleanly and is logged.
func (d *daemon) restoreWin(ctl *control, wid string, s winSnap) {
	if _, err := ctl.run("select-layout -t " + q(wid) + " " + q(s.layout)); err != nil {
		log.Printf("restore layout %s: %v (panes changed while docked?)", wid, err)
	}
	_, _ = ctl.run("select-pane -t " + q(s.activePane))
	_, _ = ctl.run("set-option -w -t " + q(wid) + " automatic-rename " + s.autoRename)
}

// pinSessionCmds marks a session as pinned: the bind-routing flag M-h/M-l
// check, plus the status pad that shifts the status line past the sidebar.
func pinSessionCmds(sid string) []string {
	pad := strings.Repeat(" ", statusPad)
	return []string{
		"set-option -t " + q(sid) + " @demux_pinned 1",
		"set-option -t " + q(sid) + " status-left " + q(pad),
		fmt.Sprintf("set-option -t %s status-left-length %d", q(sid), statusPad),
	}
}

// savedStatus records the session-level status-left options before the pad
// clobbers them. show-options output replays verbatim through set-option, so
// the raw line is the restore command; no line means session-level unset.
func (d *daemon) savedStatus(ctl *control, p *pinState, sid string) {
	p.statusPrev, p.statusLenPrev = "", ""
	if lines, err := ctl.run("show-options -t " + q(sid) + " status-left"); err == nil && len(lines) > 0 {
		p.statusPrev = lines[0]
	}
	if lines, err := ctl.run("show-options -t " + q(sid) + " status-left-length"); err == nil && len(lines) > 0 {
		p.statusLenPrev = lines[0]
	}
}

func (d *daemon) restoreStatus(ctl *control, p *pinState) {
	if p.statusPrev != "" {
		_, _ = ctl.run("set-option -t " + q(p.sess) + " " + p.statusPrev)
	} else {
		_, _ = ctl.run("set-option -u -t " + q(p.sess) + " status-left")
	}
	if p.statusLenPrev != "" {
		_, _ = ctl.run("set-option -t " + q(p.sess) + " " + p.statusLenPrev)
	} else {
		_, _ = ctl.run("set-option -u -t " + q(p.sess) + " status-left-length")
	}
}

// checkPin runs after every re-list: follow the client when it switched
// windows by a path we don't route (choose-tree, prefix keys), clean up when
// the client detached or the sidebar pane died, and keep the sidebar at its
// fixed width when a client resize rescaled it.
func (d *daemon) checkPin(ctl *control, w world) {
	p := d.pin
	if p == nil {
		return
	}
	if d.br == nil || !paneAlive(w, d.br.pane) {
		// Sidebar pane died (kill-window on the host, kill-pane): the dock is
		// gone, only session state needs cleaning. Layout restore would fight
		// whatever the user just did — skip it.
		log.Printf("pin: sidebar pane gone, cleaning up")
		d.pin = nil
		d.br = nil
		_, _ = ctl.run("set-option -u -t " + q(p.sess) + " @demux_pinned")
		d.restoreStatus(ctl, p)
		return
	}
	var cl *tclient
	for i := range w.Clients {
		if w.Clients[i].Name == p.client {
			cl = &w.Clients[i]
			break
		}
	}
	if cl == nil {
		log.Printf("pin: client %s detached, undocking", p.client)
		_ = d.pinClose(ctl, false)
		return
	}
	for _, s := range w.Sessions {
		if s.ID == cl.SessionID && s.Name == demuxSession {
			return // client is on the browse surface itself; not ours to follow
		}
	}
	cur := ""
	for _, win := range w.Windows {
		if win.SessionID == cl.SessionID && win.Active {
			cur = win.ID
			break
		}
	}
	if cur != "" && cur != p.win {
		log.Printf("pin: follow %s -> %s (unrouted switch)", p.win, cur)
		if err := d.pinScrub(ctl, cur, true); err != nil {
			log.Printf("pin follow: %v", err)
			return
		}
		d.h.sendRole("list", marshalLine(selectMsg{Type: "select", Window: cur}))
		return
	}
	for _, pn := range w.Panes {
		if pn.ID == d.br.pane && pn.WindowID == p.win && pn.Width != listWidth {
			_, _ = ctl.run(fmt.Sprintf("resize-pane -t %s -x %d", q(d.br.pane), listWidth))
			break
		}
	}
}

func paneAlive(w world, pid string) bool {
	for _, p := range w.Panes {
		if p.ID == pid {
			return true
		}
	}
	return false
}
