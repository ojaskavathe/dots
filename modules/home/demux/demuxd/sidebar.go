package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Daemon-side sidebar mechanics. The UX contract comes from the sh spike
// (M-s summon/focus/close, j/k live preview, Enter commits, q closes, a real
// pane that makes room — never an overlay), rebuilt on the rules it earned:
//
//   - select-layout is BANNED (positional: corrupts user splits). Width
//     give-back is resize-pane only, targeting pane ids.
//   - width baselines are queried over the control connection IMMEDIATELY
//     before each join. tmux serializes commands on the socket, so the
//     baseline cannot be stale — the whiplash/clobber class of the sh spike
//     is unrepresentable here.
//   - critical commands first in a sequence (join/kill/switch before
//     restores), restore targets filtered against the live model: sequences
//     abort at the first error and a dead pane id must not eat the rest.
//   - all client switching is explicit (-c): implicit switch-client picks the
//     wrong client whenever more than one is attached (the daemon's own
//     control client always is).

const sidebarWidth = 40

type sidebarState struct {
	pane     string         // %id of the pane running `demuxd tui`
	client   string         // owning tmux client (explicit switching)
	window   string         // current host window @id
	session  string         // host window's session $id
	baseline map[string]int // host-window pane widths captured at join
}

type daemon struct {
	tmuxSock string
	h        *hub
	sb       *sidebarState
}

func (d *daemon) handleCmd(ctl *control, env cmdEnvelope) {
	var err error
	switch env.msg.Cmd {
	case "toggle":
		err = d.toggle(ctl, env.msg.Client)
	case "preview":
		err = d.preview(ctl, env.msg.Window)
	case "commit", "close":
		err = d.closeSidebar(ctl)
	default:
		err = fmt.Errorf("unknown cmd %q", env.msg.Cmd)
	}
	r := replyMsg{Type: "reply", OK: err == nil}
	if err != nil {
		r.Err = err.Error()
	}
	d.h.send(env.sub, marshalLine(r))
}

// toggle is the M-s cycle: absent -> summon; elsewhere -> pull here; present
// but unfocused -> focus; focused -> close.
func (d *daemon) toggle(ctl *control, client string) error {
	if client == "" {
		return errors.New("toggle needs a client name")
	}
	sid, wid, pid, err := d.clientView(ctl, client)
	if err != nil {
		return err
	}
	if d.sb == nil {
		return d.summon(ctl, client, sid, wid, pid)
	}
	d.sb.client = client
	if d.sb.window != wid {
		return d.join(ctl, sid, wid, joinFocus)
	}
	if pid != d.sb.pane {
		_, err := ctl.run("select-pane -t " + q(d.sb.pane))
		return err
	}
	return d.closeSidebar(ctl)
}

// clientView asks tmux (not the model) where the client is right now: the
// model's geometry for sessions the daemon isn't attached to can lag, and a
// summon must land exactly where the user is looking. list-clients, not
// display-message: `display-message -p -c X fmt` expands the format in the
// ISSUING client's context (verified — -c only picks where the message would
// be shown), while each list-clients line is expanded in that client's own
// context.
func (d *daemon) clientView(ctl *control, client string) (sid, wid, pid string, err error) {
	lines, err := ctl.run("list-clients -F " + f("#{client_name}", "#{session_id}", "#{window_id}", "#{pane_id}"))
	if err != nil {
		return "", "", "", err
	}
	for _, ln := range lines {
		p := strings.Split(ln, sep)
		if len(p) == 4 && p[0] == client {
			return p[1], p[2], p[3], nil
		}
	}
	return "", "", "", errors.New("no client " + client)
}

func (d *daemon) summon(ctl *control, client, sid, wid, pid string) error {
	base, err := d.freshBaseline(ctl, wid)
	if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	lines, err := ctl.run(fmt.Sprintf("split-window -hbf -l %d -t %s -P -F '#{pane_id}' %s",
		sidebarWidth, q(pid), q(self+" -S "+d.tmuxSock+" tui")))
	if err != nil {
		return err
	}
	if len(lines) == 0 {
		return errors.New("split-window returned no pane id")
	}
	sb := strings.TrimSpace(lines[0])
	// @demux_sidebar keeps tmux-equalize-nvim refusing to touch this window;
	// focus lands on the sidebar so j/k work immediately after M-s.
	_, _ = ctl.runSeq(
		"set-option -p -t "+q(sb)+" @demux_sidebar 1",
		"select-pane -t "+q(sb))
	d.sb = &sidebarState{pane: sb, client: client, window: wid, session: sid, baseline: base}
	return nil
}

type joinMode int

const (
	joinFocus  joinMode = iota // pull: sidebar takes focus, client stays put
	joinSwitch                 // preview: client switches, sidebar keeps focus
	joinFollow                 // ride along: no switch, no focus theft
)

// join moves the sidebar pane to another window, restoring the widths of the
// window it leaves. One command sequence: critical first, restores last.
func (d *daemon) join(ctl *control, sid, wid string, mode joinMode) error {
	if d.sb == nil {
		return nil
	}
	base, err := d.freshBaseline(ctl, wid)
	if err != nil {
		return err
	}
	oldWin, oldBase := d.sb.window, d.sb.baseline
	cmds := []string{fmt.Sprintf("join-pane -hbdf -l %d -s %s -t %s", sidebarWidth, q(d.sb.pane), q(wid))}
	switch mode {
	case joinSwitch:
		cmds = append(cmds,
			"select-window -t "+q(wid),
			"switch-client -c "+q(d.sb.client)+" -t "+q(sid),
			"select-pane -t "+q(d.sb.pane))
	case joinFocus:
		cmds = append(cmds, "select-pane -t "+q(d.sb.pane))
	}
	cmds = append(cmds, d.restoreCmds(oldWin, oldBase)...)
	_, err = ctl.runSeq(cmds...)
	if err != nil {
		// The sequence aborted somewhere; re-derive where the sidebar
		// actually is instead of guessing.
		d.resync(ctl)
		return err
	}
	d.sb.window, d.sb.session, d.sb.baseline = wid, sid, base
	return nil
}

func (d *daemon) preview(ctl *control, wid string) error {
	if d.sb == nil {
		return errors.New("no sidebar")
	}
	if wid == d.sb.window {
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
		return fmt.Errorf("unknown window %s", wid)
	}
	return d.join(ctl, sid, wid, joinSwitch)
}

// closeSidebar kills the sidebar pane and gives the freed width back in the
// same sequence — one reflow, not two (spike rule).
func (d *daemon) closeSidebar(ctl *control) error {
	if d.sb == nil {
		return nil
	}
	cmds := append([]string{"kill-pane -t " + q(d.sb.pane)}, d.restoreCmds(d.sb.window, d.sb.baseline)...)
	_, err := ctl.runSeq(cmds...)
	d.sb = nil
	return err
}

// freshBaseline records the target window's pane widths right before a join.
// Live by construction: this query and the join that follows are serialized
// on the same connection.
func (d *daemon) freshBaseline(ctl *control, wid string) (map[string]int, error) {
	lines, err := ctl.run("list-panes -t " + q(wid) + " -F " + f("#{pane_id}", "#{pane_width}"))
	if err != nil {
		return nil, err
	}
	base := map[string]int{}
	for _, ln := range lines {
		p := strings.Split(ln, sep)
		if len(p) != 2 {
			continue
		}
		if d.sb != nil && p[0] == d.sb.pane {
			continue
		}
		if w, err := strconv.Atoi(p[1]); err == nil {
			base[p[0]] = w
		}
	}
	return base, nil
}

// restoreCmds builds resize-pane commands for panes that are still alive and
// still in the window they were recorded in — a dead target would abort the
// tail of the sequence.
func (d *daemon) restoreCmds(wid string, base map[string]int) []string {
	if len(base) == 0 {
		return nil
	}
	live := map[string]string{}
	for _, p := range d.h.getWorld().Panes {
		live[p.ID] = p.WindowID
	}
	var cmds []string
	for id, width := range base {
		if d.sb != nil && id == d.sb.pane {
			continue
		}
		if live[id] != wid {
			continue
		}
		cmds = append(cmds, fmt.Sprintf("resize-pane -t %s -x %d", q(id), width))
	}
	return cmds
}

// resync re-derives sidebar location from tmux after a failed sequence.
func (d *daemon) resync(ctl *control) {
	if d.sb == nil {
		return
	}
	lines, err := ctl.run("display-message -p -t " + q(d.sb.pane) + " " + f("#{session_id}", "#{window_id}"))
	if err != nil || len(lines) == 0 {
		d.sb = nil
		return
	}
	p := strings.Split(lines[0], sep)
	if len(p) != 2 {
		d.sb = nil
		return
	}
	d.sb.session, d.sb.window = p[0], p[1]
}

// checkSidebar runs after every re-list: clears state if the sidebar pane
// died externally, and makes the sidebar ride along when its client moved by
// native means (M-h/M-l, choose-tree, whatever) — the daemon sees
// %session-window-changed / %client-session-changed for every session, so no
// tmux hooks are needed. The daemon's own moves update state synchronously
// first, so they arrive here already consistent: no self-suppression dance.
func (d *daemon) checkSidebar(ctl *control, w world) {
	if d.sb == nil {
		return
	}
	alive := false
	for _, p := range w.Panes {
		if p.ID == d.sb.pane {
			alive = true
			break
		}
	}
	if !alive {
		if cmds := d.restoreCmds(d.sb.window, d.sb.baseline); len(cmds) > 0 {
			_, _ = ctl.runSeq(cmds...)
		}
		d.sb = nil
		return
	}
	csid := ""
	for _, c := range w.Clients {
		if c.Name == d.sb.client {
			csid = c.SessionID
			break
		}
	}
	if csid == "" {
		return // client detached; leave the sidebar where it is
	}
	for _, win := range w.Windows {
		if win.SessionID == csid && win.Active && win.ID != d.sb.window {
			_ = d.join(ctl, csid, win.ID, joinFollow)
			return
		}
	}
}

// q quotes an argument for tmux's command parser. Everything we pass (pane /
// window / session ids, client names, store paths) is shell-tame; the quotes
// guard spaces only.
func q(s string) string {
	return "'" + s + "'"
}
