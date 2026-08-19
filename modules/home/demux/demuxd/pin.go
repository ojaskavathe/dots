package main

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
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
// Sequencing rules that keep transitions invisible:
//   - everything the arriving window needs (join, select-window, status pad,
//     @demux_pinned) rides ONE control sequence BEFORE switch-client — the
//     status pad landing after the switch is a visible flicker
//   - undock + layout restore ride ONE sequence — as separate round trips,
//     apps in the window (nvim) see two SIGWINCH resizes and jitter
//
// While docked, tools may reshape the window deliberately (tmux-equalize-nvim
// equalizes the main region and sets @demux_layout_dirty). On leave, a dirty
// window gets a PROPORTIONAL give-back (layout.go: drop the sidebar leaf,
// rescale) instead of the pre-dock snapshot, which would undo the change.
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

// statusSave holds a session's pre-pad status-left options: raw
// `show-options` lines that replay verbatim through set-option, empty when
// the option was not set at session level.
type statusSave struct {
	sess    string
	left    string
	leftLen string
}

type pinState struct {
	client     string
	win        string // window currently hosting the sidebar
	sess       string // session of win (tracks @demux_pinned + status pad)
	originSess string // where q returns to
	originWin  string
	snap       winSnap    // pre-dock snapshot of win
	status     statusSave // pre-pad status-left of sess

	// scrubbing: the sidebar pane is ZOOMED and the main area shows live
	// billboards of the selection instead of real windows. Zoom leaves the
	// hidden panes untouched (rig-verified: sizes byte-exact, zero app
	// reflows) — scrubbing costs captures, not window mutations. The real
	// window materializes on commit (join-from-zoomed auto-unzooms), and
	// landing back on the pinned window is a free unzoom.
	scrubbing bool
}

// statusPad shifts the status line's content past the sidebar column:
// 40 cols of pane + 1 col of pane border.
const statusPad = listWidth + 1

func snapQuery(wid string) string {
	return "display-message -p -t " + q(wid) + " -F " +
		f("#{window_layout}", "#{pane_id}", "#{automatic-rename}", "#{window_name}")
}

func parseSnap(line string) (winSnap, error) {
	p := strings.Split(line, sep)
	if len(p) != 4 {
		return winSnap{}, fmt.Errorf("bad snapshot %q", line)
	}
	return winSnap{layout: p[0], activePane: p[1], autoRename: p[2], name: p[3]}, nil
}

func (d *daemon) winSnapshot(ctl *control, wid string) (winSnap, error) {
	lines, err := ctl.run(snapQuery(wid))
	if err != nil {
		return winSnap{}, err
	}
	if len(lines) == 0 {
		return winSnap{}, fmt.Errorf("no snapshot for %s", wid)
	}
	return parseSnap(lines[0])
}

// leaveInfo queries what restoring the window being left needs: its CURRENT
// docked layout (for proportional give-back) and the dirty marker.
func (d *daemon) leaveInfo(ctl *control, wid string) (layout string, dirty bool) {
	lines, err := ctl.run("display-message -p -t " + q(wid) + " -F " +
		f("#{window_layout}", "#{@demux_layout_dirty}"))
	if err != nil || len(lines) == 0 {
		return "", false
	}
	p := strings.Split(lines[0], sep)
	if len(p) != 2 {
		return "", false
	}
	return p[0], p[1] == "1"
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
	p.status = d.savedStatus(ctl, sid)
	// Freeze rename BEFORE the join: the sidebar takes focus on join, and an
	// automatic-rename window would flip its name to "demuxd" (the sh-era bug).
	seq := []string{
		"set-option -w -t " + q(wid) + " automatic-rename off",
		"set-option -p -t " + q(d.br.pane) + " @demux_sidebar 1",
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

// scrubStart begins billboard scrubbing: capture the target FIRST (the TUI
// caches the frame before its WINCH repaint, so the canvas fills instantly),
// then zoom the sidebar to full window. The client never leaves the pinned
// window — status line, layouts, and every hidden pane stay untouched.
func (d *daemon) scrubStart(ctl *control, wid string) error {
	p := d.pin
	if p == nil || p.scrubbing {
		return nil
	}
	if err := d.preview(ctl, wid, false); err != nil {
		return err
	}
	if _, err := ctl.runSeq(
		"resize-pane -Z -t "+q(d.br.pane),
		"select-pane -t "+q(d.br.pane)); err != nil {
		return err
	}
	p.scrubbing = true
	d.startStream()
	log.Printf("scrub start from=%s target=%s", p.win, wid)
	return nil
}

// scrubEnd stops billboard scrubbing. unzoom=true is the landed-back-home
// path (Enter/q on the pinned window): the real panes reappear exactly as
// they were. unzoom=false is for paths where a join/break is about to move
// the zoomed sidebar anyway (tmux auto-unzooms on both — rig-verified).
func (d *daemon) scrubEnd(ctl *control, unzoom bool) {
	p := d.pin
	if p == nil || !p.scrubbing {
		return
	}
	p.scrubbing = false
	d.stopStream()
	if unzoom {
		_, _ = ctl.run("resize-pane -Z -t " + q(d.br.pane))
	}
	log.Printf("scrub end win=%s unzoom=%v", p.win, unzoom)
}

// pinScrub moves the main area to wid FOR REAL: the sidebar rides along in
// the same sequence, so the target window is never visible without it. Used
// by commit-from-scrub, routed nav, and unrouted-switch follow — never by
// plain list scrubbing anymore (that's billboards via scrubStart). Everything
// the arriving session needs (status pad, @demux_pinned) is in the sequence
// BEFORE switch-client — after it, the pad lands a frame late and the status
// line visibly flickers. focusMain puts the keyboard in the window's own
// pane; false keeps focus in the sidebar.
func (d *daemon) pinScrub(ctl *control, wid string, focusMain bool) error {
	p := d.pin
	if p == nil || wid == "" || wid == p.win {
		return nil
	}
	sidN := d.sessionOf(wid)
	if sidN == "" {
		return fmt.Errorf("scrub target %s unknown", wid)
	}
	// Both point-of-use queries in one round trip: snapshot of the window we
	// enter, leave-info of the one we exit.
	qlines, err := ctl.runSeq(
		snapQuery(wid),
		"display-message -p -t "+q(p.win)+" -F "+f("#{window_layout}", "#{@demux_layout_dirty}"))
	if err != nil {
		return err
	}
	if len(qlines) < 2 {
		return errors.New("scrub query came back short")
	}
	snapN, err := parseSnap(qlines[0])
	if err != nil {
		return err
	}
	oldLayout, oldDirty := "", false
	if lp := strings.Split(qlines[1], sep); len(lp) == 2 {
		oldLayout, oldDirty = lp[0], lp[1] == "1"
	}
	var statusN statusSave
	if sidN != p.sess {
		statusN = d.savedStatus(ctl, sidN) // before the pad, or we save our own pad
	}
	critical := []string{
		"set-option -w -t " + q(wid) + " automatic-rename off",
		fmt.Sprintf("join-pane -hb -f -l %d -s %s -t %s", listWidth, q(d.br.pane), q(wid)),
		"select-window -t " + q(wid),
	}
	if sidN != p.sess {
		critical = append(critical, pinSessionCmds(sidN)...)
		critical = append(critical, "switch-client -c "+q(p.client)+" -t "+q(sidN))
	}
	if focusMain {
		critical = append(critical, "select-pane -t "+q(snapN.activePane))
	}
	// The old window's restores ride the SAME socket write (pipelined): its
	// join-away resize and its layout restore reach the server in one read,
	// so its apps (nvim) coalesce the two SIGWINCHes into one reflow —
	// separate round trips here doubled every visited window's reflow work.
	// select-layout leads the fallible tail: if the critical line failed, the
	// sidebar is still in the old window and the restore fails on pane count,
	// aborting the rename re-enable that would otherwise fire while the
	// sidebar holds focus.
	restore := []string{
		"set-option -w -uq -t " + q(p.win) + " @demux_layout_dirty",
	}
	if sidN != p.sess {
		restore = append(restore, statusRestoreCmds(p.status)...)
		restore = append(restore, "set-option -uq -t "+q(p.sess)+" @demux_pinned")
	}
	if rl := d.leaveLayout(p.win, p.snap, oldLayout, oldDirty); rl != "" {
		restore = append(restore, "select-layout -t "+q(p.win)+" "+q(rl))
	}
	restore = append(restore,
		"select-pane -t "+q(p.snap.activePane),
		"set-option -w -t "+q(p.win)+" automatic-rename "+p.snap.autoRename)
	_, errs := ctl.runPipelined(critical, restore)
	if errs[0] != nil {
		return errs[0]
	}
	d.scrubEnd(ctl, false) // join already unzoomed the sidebar on its way out
	if errs[1] != nil {
		log.Printf("scrub restore %s: %v", p.win, errs[1])
	}
	if sidN != p.sess {
		p.status = statusN
	}
	prev := p.win
	p.win, p.sess, p.snap = wid, sidN, snapN
	d.lastScrub = time.Now()
	if bench {
		log.Printf("bench scrub %s -> %s focus_main=%v dirty=%v", prev, wid, focusMain, oldDirty)
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
// Undock and restore ride ONE sequence: split across round trips, the
// window's apps see two resizes (break-pane's give-all-to-the-neighbor, then
// the restore) and visibly jitter.
func (d *daemon) pinClose(ctl *control, toOrigin bool) error {
	p := d.pin
	if p == nil {
		return nil
	}
	d.scrubEnd(ctl, true) // settle any zoom before undocking
	d.pin = nil
	log.Printf("pin close client=%s win=%s to_origin=%v", p.client, p.win, toOrigin)
	if toOrigin && p.originWin != p.win {
		_, err := ctl.runSeq(
			"select-window -t "+q(p.originWin),
			"switch-client -c "+q(p.client)+" -t "+q(p.originSess))
		if err != nil {
			// Origin may have died; session alone, then stay where we are.
			_, _ = ctl.run("switch-client -c " + q(p.client) + " -t " + q(p.originSess))
		}
	}
	oldLayout, oldDirty := d.leaveInfo(ctl, p.win)
	restore := d.leaveLayout(p.win, p.snap, oldLayout, oldDirty)
	seq := []string{
		"break-pane -d -P -F " + f("#{session_id}", "#{window_id}") +
			" -s " + q(d.br.pane) + " -t " + q(demuxSession+":"),
		"set-option -w -t " + q(p.win) + " automatic-rename " + p.snap.autoRename,
		"set-option -w -uq -t " + q(p.win) + " @demux_layout_dirty",
	}
	if restore != "" {
		seq = append(seq, "select-layout -t "+q(p.win)+" "+q(restore))
	}
	seq = append(seq, "select-pane -t "+q(p.snap.activePane))
	lines, err := ctl.runSeq(seq...)
	if err != nil {
		log.Printf("pin undock: %v", err)
	}
	if len(lines) > 0 {
		if parts := strings.Split(lines[0], sep); len(parts) == 2 {
			d.br.sess, d.br.win = parts[0], parts[1]
			_, _ = ctl.run("set-option -wq -t " + q(d.br.win) + " automatic-rename off")
		}
	}
	_, _ = ctl.run("set-option -u -t " + q(p.sess) + " @demux_pinned")
	d.restoreStatus(ctl, p.status)
	return nil
}

// leaveLayout picks what a window being left should get back: the exact
// pre-dock snapshot normally, or a proportional sans-sidebar rescale of the
// CURRENT docked layout when it was deliberately reshaped while docked
// (@demux_layout_dirty). Empty means no restore (let tmux expand naturally).
func (d *daemon) leaveLayout(wid string, snap winSnap, dockedLayout string, dirty bool) string {
	if !dirty {
		return snap.layout
	}
	s, err := sansSidebar(dockedLayout, d.br.pane)
	if err != nil {
		log.Printf("give-back %s: %v", wid, err)
		return ""
	}
	return s
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
func (d *daemon) savedStatus(ctl *control, sid string) statusSave {
	s := statusSave{sess: sid}
	if lines, err := ctl.run("show-options -t " + q(sid) + " status-left"); err == nil && len(lines) > 0 {
		s.left = lines[0]
	}
	if lines, err := ctl.run("show-options -t " + q(sid) + " status-left-length"); err == nil && len(lines) > 0 {
		s.leftLen = lines[0]
	}
	return s
}

// statusRestoreCmds builds the commands that put a session's status-left
// back: replay the saved raw show-options lines, or quiet-unset down to the
// global value. All infallible, safe anywhere in a sequence.
func statusRestoreCmds(s statusSave) []string {
	if s.sess == "" {
		return nil
	}
	out := make([]string, 0, 2)
	if s.left != "" {
		out = append(out, "set-option -t "+q(s.sess)+" "+s.left)
	} else {
		out = append(out, "set-option -uq -t "+q(s.sess)+" status-left")
	}
	if s.leftLen != "" {
		out = append(out, "set-option -t "+q(s.sess)+" "+s.leftLen)
	} else {
		out = append(out, "set-option -uq -t "+q(s.sess)+" status-left-length")
	}
	return out
}

func (d *daemon) restoreStatus(ctl *control, s statusSave) {
	for _, c := range statusRestoreCmds(s) {
		_, _ = ctl.run(c)
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
		d.restoreStatus(ctl, p.status)
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
	if p.scrubbing {
		for _, pn := range w.Panes {
			if pn.ID == d.br.pane && pn.WindowID == p.win && pn.Width == listWidth {
				// The zoom broke externally: selecting any other pane
				// (vim-navigator C-h/C-l out of the billboard) auto-unzooms.
				// Reality is the pinned window again — end the scrub state
				// and snap the list highlight back, or the next j/k would
				// stream billboards at a 40-col TUI that can't paint them.
				log.Printf("pin: scrub unzoomed externally, ending")
				d.scrubEnd(ctl, false)
				d.h.sendRole("list", marshalLine(selectMsg{Type: "select", Window: p.win}))
				break
			}
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
	if p.scrubbing {
		return // zoomed: full-width by design; enforcing 40 would unzoom it
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
