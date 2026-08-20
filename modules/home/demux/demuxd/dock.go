package main

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

// Docked mode: the herdr layout on tmux. The TUI pane docks as a real 40-col
// pane at the left edge of the client's current window; the main area is the
// user's actual panes — live, focusable, typable. Scrubbing the list moves
// the MAIN AREA for real (join sidebar into the target window + select it in
// one sequence), so what you see while scrolling is the window itself, not a
// capture. M-s toggles the dock; Enter drops focus into the main pane; q
// undocks and returns to the origin window.
//
// Rig-verified (tmux 3.7b, rigs/):
//   - `join-pane -hb -f -l 40` lands at the window's left edge no matter
//     which pane is targeted, and the joined pane takes focus (no -d)
//   - undocking hands the sidebar's 40 cols to the ADJACENT pane, not back
//     proportionally — layout restore from a saved #{window_layout} string is
//     mandatory, and restores exactly
//   - select-layout with a stale string (user split while docked) fails
//     cleanly; restores therefore run after the critical commands
//   - moving the only pane out of _demux kills the session — a placeholder
//     window keeps it alive as the undock target
//
// Sequencing rules that keep transitions invisible:
//   - everything the arriving window needs (join, select-window, status pad,
//     @demux_docked) rides ONE control sequence BEFORE switch-client — the
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

type dockState struct {
	client     string
	win        string // window currently hosting the sidebar
	sess       string // session of win (tracks @demux_docked + status pad)
	originSess string // where q returns to
	originWin  string
	snap       winSnap    // pre-dock snapshot of win
	status     statusSave // pre-pad status-left of sess

	// scrubbing: the sidebar pane is ZOOMED and the main area shows live
	// billboards of the selection instead of real windows. Zoom leaves the
	// hidden panes untouched (rig-verified: sizes byte-exact, zero app
	// reflows) — scrubbing costs captures, not window mutations. The real
	// window materializes on commit (swap-from-zoomed auto-unzooms), and
	// landing back on the docked window is a free unzoom.
	scrubbing bool

	// carved: every window that holds the dock geometry while docked. A
	// 40-col spacer pane occupies the sidebar's slot whenever the sidebar
	// itself is elsewhere, so a window's mains NEVER resize between visits:
	// entering is a geometry-free swap-pane (sidebar and spacer trade
	// places, ~7ms even on 700k-line-history panes), and billboards are the
	// docked reality by construction — the spacer is carved by the same
	// split the dock itself would make. The alternative (resizing windows
	// to the main-area width and back) reflows every pane's entire
	// scrollback history in tmux, which stalls the server ~200ms per big
	// agent pane and flushes intermediate frames (the enter flicker).
	carved map[string]*carveState
}

// carveState is one spacer-held window's restore info.
type carveState struct {
	spacer     string // spacer pane id; empty while the sidebar is in this window
	orig       string // pre-carve full-width layout, replayed at release
	autoRename string // effective automatic-rename, frozen at carve, restored at release
}

// spacerCmd runs inside spacer panes. One tick off the _demux placeholder so
// a startup sweep can identify spacers leaked by a crashed daemon exactly.
const spacerCmd = "sleep 100000001"

// carveHistoryMax caps how much scrollback a window may carry and still get
// pre-carved during scrubbing. tmux reflows a pane's entire history
// synchronously on width change — ~250ms at 660k lines (live-measured) —
// and a carve + its later release would pay that stall TWICE per dock
// session just for billboarding past the window. Above the cap the
// billboard is a scaled approximation and the carve happens only on entry.
const carveHistoryMax = 100_000

// Releases (kill spacer + replay layout) stall the tmux server for the same
// history-reflow reason, so they never run inline with an undock: the
// transition paints first (releaseSettle), then pending windows drain one
// per tick, yielding to any queued user input.
const (
	releaseSettle = 120 * time.Millisecond
	releaseTick   = 50 * time.Millisecond
)

// releaseItem is one spacer-held window awaiting restore after undock.
type releaseItem struct {
	wid string
	t   *carveState
}

// deferReleases queues every spacer-held window for restore and arms the
// drain timer. Re-docking before the drain finishes ADOPTS the queue back
// instead (dockOpen) — the carves are still valid, so a quick M-s round
// trip costs nothing.
func (d *daemon) deferReleases(p *dockState) {
	for wid, t := range p.carved {
		delete(p.carved, wid)
		if t.spacer == "" {
			continue
		}
		d.pendingRelease = append(d.pendingRelease, releaseItem{wid: wid, t: t})
	}
	if len(d.pendingRelease) > 0 {
		d.armRelease(releaseSettle)
	}
}

func (d *daemon) armRelease(after time.Duration) {
	if d.releaseT != nil {
		d.releaseT.Stop()
	}
	d.releaseT = time.NewTimer(after)
	d.releaseC = d.releaseT.C
}

// releaseOne puts one spacer-held window back exactly as it was: kill the
// spacer and replay the original layout in one batch (the two reflows
// coalesce; letting tmux expand into the hole would drift borders ±1).
func (d *daemon) releaseOne(ctl *control, it releaseItem) {
	lay, dirty := d.leaveInfo(ctl, it.wid)
	seq := []string{"kill-pane -t " + q(it.t.spacer)}
	if rl := d.leaveLayout(it.wid, it.t.orig, lay, dirty, it.t.spacer); rl != "" {
		seq = append(seq, "select-layout -t "+q(it.wid)+" "+q(rl))
	}
	seq = append(seq,
		"set-option -w -t "+q(it.wid)+" automatic-rename "+it.t.autoRename,
		"set-option -w -uq -t "+q(it.wid)+" @demux_layout_dirty")
	if _, err := ctl.runSeq(seq...); err != nil {
		log.Printf("release %s: %v", it.wid, err)
	} else if bench {
		log.Printf("bench release win=%s", it.wid)
	}
}

// sweepSpacers kills spacer panes a previous daemon left behind (it died or
// was killed while docked) — matched by their distinctive start command.
func (d *daemon) sweepSpacers(ctl *control) {
	lines, err := ctl.run("list-panes -a -F " + f("#{pane_id}", "#{pane_start_command}"))
	if err != nil {
		return
	}
	for _, ln := range lines {
		p := strings.Split(ln, sep)
		if len(p) != 2 {
			continue
		}
		// tmux wraps #{pane_start_command} in double quotes (3.7).
		if strings.Trim(p[1], `"`) == spacerCmd {
			_, _ = ctl.run("kill-pane -t " + q(p[0]))
			log.Printf("swept leaked spacer %s", p[0])
		}
	}
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

// otherClientOn reports whether any client besides the docked one is
// attached to the window's session — pre-rendering a window such a client
// might be viewing would visibly resize it under them.
func (d *daemon) otherClientOn(wid string) bool {
	w := d.h.getWorld()
	sid := ""
	for _, win := range w.Windows {
		if win.ID == wid {
			sid = win.SessionID
			break
		}
	}
	if sid == "" {
		return true // unknown window: don't touch it
	}
	for _, c := range w.Clients {
		if c.SessionID == sid && (d.dock == nil || c.Name != d.dock.client) {
			return true
		}
	}
	return false
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

// dockOpen docks the sidebar into the client's current window.
func (d *daemon) dockOpen(ctl *control, client string) error {
	if client == "" {
		return errors.New("dock needs a client name")
	}
	sid, wid, cw, ch, err := d.clientView(ctl, client)
	if err != nil {
		return err
	}
	if err := d.ensureSurface(ctl, cw, ch); err != nil {
		return err
	}
	// Adopt releases still pending from the previous dock session: their
	// carves are valid, so a quick M-s round trip re-uses them for free.
	// The window being docked INTO can't keep a spacer (the sidebar joins
	// beside it) — that one is released inline, before its snapshot.
	adopted := map[string]*carveState{}
	if len(d.pendingRelease) > 0 {
		for _, it := range d.pendingRelease {
			if it.wid == wid {
				d.releaseOne(ctl, it)
				continue
			}
			adopted[it.wid] = it.t
		}
		d.pendingRelease = nil
	}
	snap, err := d.winSnapshot(ctl, wid)
	if err != nil {
		return err
	}
	p := &dockState{client: client, win: wid, sess: sid, originSess: sid, originWin: wid, snap: snap,
		carved: adopted}
	p.status = d.savedStatus(ctl, sid)
	// Freeze rename BEFORE the join: the sidebar takes focus on join, and an
	// automatic-rename window would flip its name to "demuxd" (the sh-era bug).
	seq := []string{
		"set-option -w -t " + q(wid) + " automatic-rename off",
		"set-option -p -t " + q(d.sur.pane) + " @demux_sidebar 1",
		fmt.Sprintf("join-pane -hb -f -l %d -s %s -t %s", listWidth, q(d.sur.pane), q(wid)),
	}
	seq = append(seq, dockSessionCmds(sid)...)
	if _, err := ctl.runSeq(seq...); err != nil {
		d.dock = nil
		return err
	}
	d.dock = p
	n := d.h.sendRole("list", marshalLine(selectMsg{Type: "select", Window: wid}))
	log.Printf("dock open client=%s win=%s/%s size=%dx%d select_receivers=%d", client, sid, wid, cw, ch, n)
	return nil
}

// scrubStart begins billboard scrubbing: capture the target FIRST (the TUI
// caches the frame before its WINCH repaint, so the canvas fills instantly),
// then zoom the sidebar to full window. The client never leaves the docked
// window — status line, layouts, and every hidden pane stay untouched.
func (d *daemon) scrubStart(ctl *control, wid string) error {
	p := d.dock
	if p == nil || p.scrubbing {
		return nil
	}
	if err := d.preview(ctl, wid, false); err != nil {
		return err
	}
	if _, err := ctl.runSeq(
		"resize-pane -Z -t "+q(d.sur.pane),
		"select-pane -t "+q(d.sur.pane)); err != nil {
		return err
	}
	p.scrubbing = true
	d.startStream()
	log.Printf("scrub start from=%s target=%s", p.win, wid)
	return nil
}

// scrubEnd stops billboard scrubbing. unzoom=true is the landed-back-home
// path (Enter/q on the docked window): the real panes reappear exactly as
// they were. unzoom=false is for paths where a join/break is about to move
// the zoomed sidebar anyway (tmux auto-unzooms on both — rig-verified).
func (d *daemon) scrubEnd(ctl *control, unzoom bool) {
	p := d.dock
	if p == nil || !p.scrubbing {
		return
	}
	p.scrubbing = false
	d.stopStream()
	if unzoom {
		_, _ = ctl.run("resize-pane -Z -t " + q(d.sur.pane))
	}
	log.Printf("scrub end win=%s unzoom=%v", p.win, unzoom)
}

// dockMove moves the main area to wid FOR REAL. On a spacer-held window this
// is a geometry-free swap-pane — the sidebar and the spacer trade places, no
// pane in either window changes size, so there is nothing to reflow, no
// intermediate frame to flush, and nothing to restore on the way out. A
// first-visit window gets its spacer slot carved (the dock's own split) and
// the sidebar swapped into it, all in one batch. Either way the OLD window
// inherits the spacer and keeps its docked geometry until release. Used by
// commit-from-scrub, routed nav, and unrouted-switch follow. Everything the
// arriving session needs (status pad, @demux_docked) is in the sequence
// BEFORE switch-client — after it, the pad lands a frame late and the status
// line visibly flickers. focusMain puts the keyboard in the window's own
// pane; false keeps focus in the sidebar.
func (d *daemon) dockMove(ctl *control, wid string, focusMain bool) error {
	p := d.dock
	if p == nil || wid == "" || wid == p.win {
		return nil
	}
	sidN := d.sessionOf(wid)
	if sidN == "" {
		return fmt.Errorf("scrub target %s unknown", wid)
	}
	// One round trip: snapshot of the window we enter, plus the CURRENT
	// active pane of the one we leave (for its focus restore).
	qlines, err := ctl.runSeq(
		snapQuery(wid),
		"display-message -p -t "+q(p.win)+" -F "+f("#{pane_id}"))
	if err != nil {
		return err
	}
	if len(qlines) < 1 {
		return errors.New("scrub query came back short")
	}
	snapN, err := parseSnap(qlines[0])
	if err != nil {
		return err
	}
	tgt := p.carved[wid]
	if tgt != nil {
		// Spacer-held: the live layout is the docked one; a later release
		// must replay the pre-carve original, and rename was frozen at carve.
		snapN.layout = tgt.orig
		snapN.autoRename = tgt.autoRename
	}
	var statusN statusSave
	if sidN != p.sess {
		statusN = d.savedStatus(ctl, sidN) // before the pad, or we save our own pad
	}
	var critical []string
	if tgt != nil && tgt.spacer != "" {
		critical = append(critical,
			"swap-pane -d -s "+q(d.sur.pane)+" -t "+q(tgt.spacer))
	} else {
		critical = append(critical,
			"set-option -w -t "+q(wid)+" automatic-rename off",
			fmt.Sprintf("split-window -d -hb -f -l %d -P -F '#{pane_id}' -t %s %s",
				listWidth, q(wid), q(spacerCmd)),
			"swap-pane -d -s "+q(d.sur.pane)+" -t "+q(wid+".{top-left}"))
	}
	critical = append(critical, "select-window -t "+q(wid))
	if sidN != p.sess {
		critical = append(critical, dockSessionCmds(sidN)...)
		critical = append(critical, "switch-client -c "+q(p.client)+" -t "+q(sidN))
	}
	if focusMain {
		critical = append(critical, "select-pane -t "+q(snapN.activePane))
	} else {
		// swap-pane, unlike join, does not hand the sidebar focus.
		critical = append(critical, "select-pane -t "+q(d.sur.pane))
	}
	// The old window keeps its docked geometry (the spacer fills the
	// sidebar's slot) — the only restore is keyboard focus, so the spacer
	// isn't the active pane if the user switches back by hand. Prefer the
	// pane the user was ACTUALLY in (they may have moved since docking);
	// the dock-time snapshot is the fallback when the sidebar held focus.
	leaveFocus := p.snap.activePane
	if len(qlines) >= 2 && qlines[1] != "" && qlines[1] != d.sur.pane {
		leaveFocus = qlines[1]
	}
	restore := []string{"select-pane -t " + q(leaveFocus)}
	if sidN != p.sess {
		restore = append(restore, statusRestoreCmds(p.status)...)
		restore = append(restore, "set-option -uq -t "+q(p.sess)+" @demux_docked")
	}
	outs, errs := ctl.runPipelined(critical, restore)
	if errs[0] != nil {
		if tgt != nil && tgt.spacer != "" {
			// The spacer died under us (user closed it): forget the entry and
			// retry once as a first visit — tgt is nil then, no recursion.
			delete(p.carved, wid)
			log.Printf("scrub swap %s failed (%v), retrying as first visit", wid, errs[0])
			return d.dockMove(ctl, wid, focusMain)
		}
		// Carve path: if the batch died after the split, don't leak the spacer.
		for _, ln := range outs[0] {
			if strings.HasPrefix(ln, "%") {
				_, _ = ctl.run("kill-pane -t " + q(ln))
			}
		}
		return errs[0]
	}
	// The spacer now filling the old window's slot: the target's, or the one
	// the carve just printed.
	spacerOld := ""
	if tgt != nil {
		spacerOld = tgt.spacer
	} else {
		for _, ln := range outs[0] {
			if strings.HasPrefix(ln, "%") {
				spacerOld = ln
				break
			}
		}
	}
	d.scrubEnd(ctl, false) // the swap already unzoomed the sidebar on its way out
	if errs[1] != nil {
		log.Printf("scrub restore %s: %v", p.win, errs[1])
	}
	if spacerOld == "" {
		log.Printf("scrub: no spacer id for %s — release will not restore it", p.win)
	}
	p.carved[p.win] = &carveState{spacer: spacerOld, orig: p.snap.layout, autoRename: p.snap.autoRename}
	delete(p.carved, wid)
	if sidN != p.sess {
		p.status = statusN
	}
	prev := p.win
	p.win, p.sess, p.snap = wid, sidN, snapN
	d.lastScrub = time.Now()
	if bench {
		log.Printf("bench scrub %s -> %s focus_main=%v swap=%v", prev, wid, focusMain, tgt != nil)
	}
	return nil
}

// commitScrub lands a billboard scrub: on the docked window itself it is a
// free unzoom; anywhere else the sidebar docks into the target for real.
// Either way the origin resets — q now returns here.
func (d *daemon) commitScrub(ctl *control, wid string) error {
	p := d.dock
	if p == nil {
		return nil
	}
	if wid == "" || wid == p.win {
		d.scrubEnd(ctl, true)
		return d.dockCommit(ctl)
	}
	if err := d.dockMove(ctl, wid, true); err != nil {
		return err
	}
	p.originSess, p.originWin = p.sess, p.win
	return nil
}

// dockNav is the routed M-h / M-l: previous/next window of the current
// session with the sidebar riding along atomically.
func (d *daemon) dockNav(ctl *control, dir string) error {
	p := d.dock
	if p == nil {
		return errors.New("nav while not docked")
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
	if err := d.dockMove(ctl, target, true); err != nil {
		return err
	}
	d.h.sendRole("list", marshalLine(selectMsg{Type: "select", Window: target}))
	return nil
}

// dockCommit is Enter in the docked list: keep the sidebar, put the keyboard
// in the main area. The origin resets — q now returns here.
func (d *daemon) dockCommit(ctl *control) error {
	p := d.dock
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
			if pn.WindowID == p.win && pn.ID != d.sur.pane {
				target = pn.ID
				break
			}
		}
	}
	if target == "" {
		return errors.New("no main pane to focus")
	}
	p.originSess, p.originWin = p.sess, p.win
	log.Printf("dock commit focus=%s win=%s", target, p.win)
	_, err := ctl.run("select-pane -t " + q(target))
	return err
}

// dockClose undocks. toOrigin (q) returns the client to where it was docked
// or last committed; otherwise (M-s) it stays put and the layout snaps back.
// Undock and restore ride ONE sequence: split across round trips, the
// window's apps see two resizes (break-pane's give-all-to-the-neighbor, then
// the restore) and visibly jitter.
func (d *daemon) dockClose(ctl *control, toOrigin bool) error {
	p := d.dock
	if p == nil {
		return nil
	}
	// State only: break-pane below yanks the sidebar even zoomed
	// (rig-verified), and an explicit early unzoom would flash the origin
	// panes before a toOrigin switch lands.
	d.scrubEnd(ctl, false)
	d.dock = nil
	log.Printf("undock client=%s win=%s to_origin=%v", p.client, p.win, toOrigin)
	oldLayout, oldDirty, curActive := "", false, ""
	if lines, err := ctl.run("display-message -p -t " + q(p.win) + " -F " +
		f("#{window_layout}", "#{@demux_layout_dirty}", "#{pane_id}")); err == nil && len(lines) > 0 {
		if lp := strings.Split(lines[0], sep); len(lp) == 3 {
			oldLayout, oldDirty, curActive = lp[0], lp[1] == "1", lp[2]
		}
	}
	restore := d.leaveLayout(p.win, p.snap.layout, oldLayout, oldDirty, d.sur.pane)
	// Focus after undock: whatever main pane the user is IN right now. Only
	// when the sidebar itself holds focus does the dock-time snapshot apply
	// — restoring the snapshot unconditionally yanked focus back to the
	// pane that happened to be active when the sidebar opened.
	focus := p.snap.activePane
	if curActive != "" && curActive != d.sur.pane {
		focus = curActive
	}

	moving := toOrigin && p.originWin != p.win
	undock := append([]string{"set-option -uq -t " + q(p.sess) + " @demux_docked"},
		statusRestoreCmds(p.status)...)
	if moving {
		// Land first, with the session bookkeeping in the SAME batch — an
		// unpad arriving a round trip after the switch flickers the status.
		var seq []string
		if t := p.carved[p.originWin]; t != nil && t.spacer != "" {
			// Landing on a spacer-held window: drop the spacer and replay the
			// original layout in the batch with the switch — one coalesced
			// reflow, and the window arrives already full width.
			oLay, oDirty := d.leaveInfo(ctl, p.originWin)
			seq = append(seq, "kill-pane -t "+q(t.spacer))
			if rl := d.leaveLayout(p.originWin, t.orig, oLay, oDirty, t.spacer); rl != "" {
				seq = append(seq, "select-layout -t "+q(p.originWin)+" "+q(rl))
			}
			seq = append(seq,
				"set-option -w -t "+q(p.originWin)+" automatic-rename "+t.autoRename,
				"set-option -w -uq -t "+q(p.originWin)+" @demux_layout_dirty")
			delete(p.carved, p.originWin)
		}
		seq = append(seq,
			"select-window -t "+q(p.originWin),
			"switch-client -c "+q(p.client)+" -t "+q(p.originSess))
		seq = append(seq, undock...)
		if _, err := ctl.runSeq(seq...); err != nil {
			// Origin may have died; session alone, then keep cleaning.
			_, _ = ctl.run("switch-client -c " + q(p.client) + " -t " + q(p.originSess))
			for _, c := range undock {
				_, _ = ctl.run(c)
			}
		}
	}
	seq := []string{
		"break-pane -d -P -F " + f("#{session_id}", "#{window_id}") +
			" -s " + q(d.sur.pane) + " -t " + q(demuxSession+":"),
		"set-option -w -t " + q(p.win) + " automatic-rename " + p.snap.autoRename,
		"set-option -w -uq -t " + q(p.win) + " @demux_layout_dirty",
	}
	if !moving {
		// Staying put: everything (undock, unpad, restore) in one batch so
		// the redraw coalesces.
		seq = append(seq, undock...)
	}
	if restore != "" {
		seq = append(seq, "select-layout -t "+q(p.win)+" "+q(restore))
	}
	seq = append(seq, "select-pane -t "+q(focus))
	lines, err := ctl.runSeq(seq...)
	if err != nil {
		log.Printf("undock: %v", err)
	}
	if len(lines) > 0 {
		if parts := strings.Split(lines[0], sep); len(parts) == 2 {
			d.sur.sess, d.sur.win = parts[0], parts[1]
			_, _ = ctl.run("set-option -wq -t " + q(d.sur.win) + " automatic-rename off")
		}
	}
	d.deferReleases(p)
	return nil
}

// leaveLayout picks what a window being released should get back: its exact
// pre-carve layout normally, or a proportional rescale of the CURRENT docked
// layout minus the given pane when it was deliberately reshaped while docked
// (@demux_layout_dirty). Empty means no restore (let tmux expand naturally).
func (d *daemon) leaveLayout(wid string, exact string, dockedLayout string, dirty bool, drop string) string {
	if !dirty {
		return exact
	}
	s, err := sansSidebar(dockedLayout, drop)
	if err != nil {
		log.Printf("give-back %s: %v", wid, err)
		return ""
	}
	return s
}

// dockSessionCmds marks a session as docked: the bind-routing flag M-h/M-l
// check, plus the status pad that shifts the status line past the sidebar.
func dockSessionCmds(sid string) []string {
	// bg=terminal (tmux >= 3.4) is the TERMINAL's default background — what
	// the sidebar paints on — so the strip above the sidebar reads as
	// sidebar, not statusline. (bg=default would NOT work: inside the
	// status line "default" means inherit status-style, i.e. the themed
	// statusline background — a no-op.)
	pad := "#[bg=terminal,fg=terminal]" + strings.Repeat(" ", statusPad) + "#[default]"
	return []string{
		"set-option -t " + q(sid) + " @demux_docked 1",
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

// checkDock runs after every re-list: follow the client when it switched
// windows by a path we don't route (choose-tree, prefix keys), clean up when
// the client detached or the sidebar pane died, and keep the sidebar at its
// fixed width when a client resize rescaled it.
func (d *daemon) checkDock(ctl *control, w world) {
	p := d.dock
	if p == nil {
		return
	}
	if d.sur == nil || !paneAlive(w, d.sur.pane) {
		// Sidebar pane died (kill-window on the host, kill-pane): the dock is
		// gone, only session state needs cleaning. Layout restore would fight
		// whatever the user just did — skip it.
		log.Printf("dock: sidebar pane gone, cleaning up")
		d.dock = nil
		d.dropSurface()
		_, _ = ctl.run("set-option -u -t " + q(p.sess) + " @demux_docked")
		d.restoreStatus(ctl, p.status)
		d.deferReleases(p)
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
		log.Printf("dock: client %s detached, undocking", p.client)
		_ = d.dockClose(ctl, false)
		return
	}
	for _, s := range w.Sessions {
		if s.ID == cl.SessionID && s.Name == demuxSession {
			return // client is on the browse surface itself; not ours to follow
		}
	}
	if p.scrubbing {
		for _, pn := range w.Panes {
			if pn.ID == d.sur.pane && pn.WindowID == p.win && pn.Width == listWidth {
				// The zoom broke externally: selecting any other pane
				// (vim-navigator C-h/C-l out of the billboard) auto-unzooms.
				// Reality is the docked window again — end the scrub state
				// and snap the list highlight back, or the next j/k would
				// stream billboards at a 40-col TUI that can't paint them.
				log.Printf("dock: scrub unzoomed externally, ending")
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
		log.Printf("dock: follow %s -> %s (unrouted switch)", p.win, cur)
		if err := d.dockMove(ctl, cur, true); err != nil {
			log.Printf("dock follow: %v", err)
			return
		}
		d.h.sendRole("list", marshalLine(selectMsg{Type: "select", Window: cur}))
		return
	}
	if p.scrubbing {
		return // zoomed: full-width by design; enforcing 40 would unzoom it
	}
	for _, pn := range w.Panes {
		if pn.ID == d.sur.pane && pn.WindowID == p.win && pn.Width != listWidth {
			_, _ = ctl.run(fmt.Sprintf("resize-pane -t %s -x %d", q(d.sur.pane), listWidth))
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
