package main

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Agent state detection: which panes run coding agents, and whether each is
// working, blocked, idle, or done (finished while you weren't looking).
// Rules live in per-agent TOML manifests (manifest.go); this file owns the
// polling loop, the anti-flap policy, and publishing.
//
// Design rules from herdr's arc (thoughts/agent-detection-research.md):
//
//   - The SCREEN is the level-triggered authority. herdr shipped Claude
//     hooks as state authority and REVERTED (v0.6.7): hooks are
//     edge-triggered and incomplete — nothing fires on interrupt or prompt
//     dismissal, and late SubagentStop events revive dead states.
//   - The OSC title is the cheap fast path: a title rule that outranks
//     every screen rule (claude's spinner) classifies without a capture.
//   - Idle is CONFIRMED, never assumed: working -> idle holds for
//     idleConfirms samples. Blocked and working publish instantly. Only
//     idle panes with an unmoved activity stamp skip the rescan.
//   - "Done" is UI state, not detection: a completion transition
//     ((working|blocked) -> idle) in a window no attached client is
//     looking at becomes "done" and sticks until the user visits it.
//
// This is a poll — the daemon's only one — because there is no event
// source: probe-verified that tmux emits NO control-mode notification for
// pane title changes, and a cross-session `split-window -d` emits nothing
// at all. The tick's own list-panes doubles as the self-heal for panes the
// notification matrix misses.

const (
	detectTick     = 300 * time.Millisecond
	detectFastTick = 100 * time.Millisecond // while a pending-idle hold is confirming
	detectIdleTick = 2 * time.Second        // no agent panes known: discovery only
	idleConfirms   = 3
	idleCap        = 700 * time.Millisecond
	startupGrace   = 3 * time.Second
)

type agentInfo struct {
	kind         string // manifest id: claude | codex | grok | ...
	state        string // "" (unknown) | working | blocked | idle | done
	reason       string // blocked only: the matched rule's label ("permission prompt")
	grace        time.Time
	pendingIdle  int       // consecutive idle samples held back
	pendingAt    time.Time // when the hold started
	lastActivity int64     // window_activity at the last screen scan
	win          string    // pane's window (for done/notify bookkeeping)
}

type detectState struct {
	agents    map[string]*agentInfo // by pane id
	manifests map[string]*cManifest // normalized command -> manifest
	wrapKind  map[string]string     // pane_pid -> resolved kind for node/bun wrappers
	ticker    *time.Ticker
	tickC     <-chan time.Time
	period    time.Duration
	lastOpt   string // last @demux_agents value pushed
}

// wrapperCmds run agents under an interpreter: pane_current_command says
// "node", and only the process tree knows it's claude.
var wrapperCmds = map[string]bool{"node": true, "bun": true, "deno": true}

// agentKind normalizes a pane_current_command to a manifest id. Nix
// wrappers run as ".claude-wrapped" — strip the dressing before matching.
func (d *daemon) agentKind(cmd string) string {
	c := strings.TrimSuffix(strings.TrimPrefix(cmd, "."), "-wrapped")
	if m := d.det.manifests[c]; m != nil {
		return m.id
	}
	return ""
}

// armDetect ensures the detection ticker exists. It ALWAYS runs — a lazy
// discovery cadence with no agents known — because its own list-panes is
// what notices agent panes the notification matrix misses entirely.
func (d *daemon) armDetect(w world) {
	if d.det.ticker == nil {
		if d.det.agents == nil {
			d.det.agents = map[string]*agentInfo{}
			d.det.wrapKind = map[string]string{}
		}
		if d.det.manifests == nil {
			d.det.manifests = loadManifests()
		}
		d.det.period = detectIdleTick
		d.det.ticker = time.NewTicker(d.det.period)
		d.det.tickC = d.det.ticker.C
	}
	d.retune()
}

// retune matches the tick cadence to the moment: pending-idle holds confirm
// fast, live agents poll normal, an agentless server only pays discovery.
func (d *daemon) retune() {
	want := detectIdleTick
	for _, a := range d.det.agents {
		if a.pendingIdle > 0 {
			want = detectFastTick
			break
		}
		want = detectTick
	}
	if want != d.det.period && d.det.ticker != nil {
		d.det.period = want
		d.det.ticker.Reset(want)
	}
}

// injectAgents copies detection state onto a freshly fetched world, so world
// diffs carry agent fields without detection owning the re-list.
func (d *daemon) injectAgents(w *world) {
	for i := range w.Panes {
		if a := d.det.agents[w.Panes[i].ID]; a != nil {
			w.Panes[i].Agent = a.kind
			w.Panes[i].AgentState = a.state
			w.Panes[i].AgentReason = a.reason
		}
	}
}

// visibleWindows: the windows some attached client is currently looking at.
func visibleWindows(w *world) map[string]bool {
	cur := map[string]string{} // session -> active window
	for _, win := range w.Windows {
		if win.Active {
			cur[win.SessionID] = win.ID
		}
	}
	vis := map[string]bool{}
	for _, c := range w.Clients {
		if wid := cur[c.SessionID]; wid != "" {
			vis[wid] = true
		}
	}
	return vis
}

// checkSeen clears "done" on panes whose window the user has now visited.
// Called on every re-list — client window moves always trigger one.
func (d *daemon) checkSeen(w *world) {
	if len(d.det.agents) == 0 {
		return
	}
	vis := visibleWindows(w)
	for id, a := range d.det.agents {
		if a.state == "done" && vis[a.win] {
			a.state = "idle"
			log.Printf("agent %s pane=%s state=done->idle (seen)", a.kind, id)
		}
	}
}

// detectTickRun is one detection pass: classify every agent pane (title
// tier free; screen tier per the skip rule), apply anti-flap, publish a
// world diff, notify on blocked, refresh the statusline option.
func (d *daemon) detectTickRun(ctl *control, w *world) {
	lines, err := ctl.run("list-panes -a -F " +
		f("#{pane_id}", "#{pane_current_command}", "#{pane_title}", "#{window_activity}", "#{window_id}", "#{pane_pid}"))
	if err != nil {
		return
	}
	now := time.Now()
	vis := visibleWindows(w)
	seen := map[string]bool{}
	type scanReq struct {
		id    string
		a     *agentInfo
		title string
		act   int64
	}
	var scans []scanReq
	var blockedNew []string // pane ids that just turned blocked
	changed := false
	apply := func(id string, a *agentInfo, want string, visible bool, label string) {
		prev := a.state
		if d.applyAgentState(id, a, want, visible, vis) {
			changed = true
			if a.state == "blocked" && prev != "blocked" {
				blockedNew = append(blockedNew, id)
			}
		}
		// The reason rides the state: set while blocked (even when the
		// matching rule changes without a transition), gone otherwise.
		want = a.state
		if want != "blocked" {
			label = ""
		}
		if a.reason != label {
			a.reason = label
			changed = true
		}
	}
	for _, ln := range lines {
		p := strings.Split(ln, sep)
		if len(p) != 6 {
			continue
		}
		id, cmd, title, wid, pid := p[0], p[1], p[2], p[4], p[5]
		act, _ := strconv.ParseInt(p[3], 10, 64)
		kind := d.agentKind(cmd)
		if kind == "" && wrapperCmds[cmd] {
			kind = d.wrappedKind(pid)
		}
		if kind == "" {
			if a := d.det.agents[id]; a != nil {
				delete(d.det.agents, id)
				changed = changed || a.state != ""
			}
			continue
		}
		seen[id] = true
		a := d.det.agents[id]
		if a == nil || a.kind != kind {
			// New agent in this pane: grace period so startup screens
			// (splash art, resumed scrollback) can't misclassify, and no
			// stale title from the previous process gets trusted.
			a = &agentInfo{kind: kind, grace: now.Add(startupGrace), win: wid}
			d.det.agents[id] = a
			changed = true // kind is publishable immediately, state isn't
			continue
		}
		a.win = wid
		if now.Before(a.grace) {
			continue
		}
		m := d.det.manifests[kind]
		if m == nil {
			continue
		}
		if v, ok := m.eval(newSnapshot(nil, title), true); ok && v.prio > m.maxScreenPrio {
			// Title verdict outranks every screen rule: conclusive, free.
			if !v.skip {
				apply(id, a, v.state, v.visible, v.label)
			}
			continue
		}
		// herdr's skip-scan rule: only IDLE panes with an unmoved activity
		// stamp skip the screen; working/blocked always rescan — that is
		// what notices turn ends and dismissed prompts. "done" counts as
		// idle here (it IS idle, flagged unseen).
		quiet := (a.state == "idle" || a.state == "done") && act == a.lastActivity
		if !quiet {
			scans = append(scans, scanReq{id: id, a: a, title: title, act: act})
		}
	}
	for id, a := range d.det.agents {
		if !seen[id] {
			delete(d.det.agents, id)
			changed = changed || a.state != ""
		}
	}
	if len(scans) > 0 {
		caps := make([]string, 0, len(scans)*2)
		for _, s := range scans {
			caps = append(caps, "capture-pane -p -t "+q(s.id), "display-message -p "+q(frameMarker))
		}
		out, err := ctl.runSeq(caps...)
		if err == nil {
			grids := make([][]string, 1, len(scans))
			for _, ln := range out {
				if ln == frameMarker {
					grids = append(grids, nil)
					continue
				}
				grids[len(grids)-1] = append(grids[len(grids)-1], ln)
			}
			for i, s := range scans {
				if i >= len(grids) {
					break
				}
				s.a.lastActivity = s.act
				m := d.det.manifests[s.a.kind]
				v, ok := m.eval(newSnapshot(grids[i], s.title), false)
				switch {
				case !ok:
					apply(s.id, s.a, "idle", false, "") // known agent, silent screen
				case v.skip:
					// viewer overlay: freeze the previous state
				default:
					apply(s.id, s.a, v.state, v.visible, v.label)
				}
			}
		}
	}
	if changed {
		d.publishAgents(ctl, w)
		d.notifyBlocked(ctl, w, blockedNew)
		d.pushStatusOpt(ctl, w)
	}
	d.retune()
}

// publishAgents folds detection state into the world and diffs it out. If
// detection knows a pane the world doesn't (a cross-session split-window -d
// emits no notification at all), re-list for real — detection doubles as
// the self-heal for silently appearing panes.
func (d *daemon) publishAgents(ctl *control, w *world) {
	known := map[string]bool{}
	for i := range w.Panes {
		known[w.Panes[i].ID] = true
	}
	missing := false
	for id := range d.det.agents {
		if !known[id] {
			missing = true
			break
		}
	}
	w2 := *w
	if missing {
		if next, err := fetchWorld(ctl); err == nil {
			w2 = next
		}
	} else {
		w2.Panes = append([]pane(nil), w.Panes...)
		for i := range w2.Panes {
			w2.Panes[i].Agent, w2.Panes[i].AgentState = "", ""
		}
	}
	d.injectAgents(&w2)
	ops := diffWorlds(*w, w2)
	*w = w2
	if len(ops) > 0 {
		d.h.setWorld(*w, ops, false, d.tmuxSock)
	}
}

// notifyBlocked pings attached clients that are NOT looking at the pane
// that just blocked — the ones who can't already see it needs them.
func (d *daemon) notifyBlocked(ctl *control, w *world, ids []string) {
	if len(ids) == 0 {
		return
	}
	winName := map[string]string{}
	sessOf := map[string]string{}
	sessName := map[string]string{}
	for _, win := range w.Windows {
		winName[win.ID] = win.Name
		sessOf[win.ID] = win.SessionID
	}
	for _, s := range w.Sessions {
		sessName[s.ID] = s.Name
	}
	cur := map[string]string{}
	for _, win := range w.Windows {
		if win.Active {
			cur[win.SessionID] = win.ID
		}
	}
	var cmds []string
	for _, id := range ids {
		a := d.det.agents[id]
		if a == nil {
			continue
		}
		msg := fmt.Sprintf("demux: %s needs attention in %s:%s", a.kind, sessName[sessOf[a.win]], winName[a.win])
		for _, c := range w.Clients {
			if cur[c.SessionID] == a.win {
				continue // already looking at it
			}
			cmds = append(cmds, "display-message -c "+q(c.Name)+" "+q(msg))
		}
	}
	if len(cmds) > 0 {
		log.Printf("notify blocked panes=%d msgs=%d", len(ids), len(cmds))
		_, _ = ctl.runSeq(cmds...)
	}
}

// pushStatusOpt maintains @demux_agents, a global option the status line
// can reference for free: "!2 ✓1 ✻3" (blocked / done / working counts;
// empty when quiet). Zero-cost render — no #() and no process spawns.
func (d *daemon) pushStatusOpt(ctl *control, w *world) {
	nb, nd, nw := 0, 0, 0
	for _, a := range d.det.agents {
		switch a.state {
		case "blocked":
			nb++
		case "done":
			nd++
		case "working":
			nw++
		}
	}
	var parts []string
	if nb > 0 {
		parts = append(parts, fmt.Sprintf("#[fg=red,bold]!%d#[default]", nb))
	}
	if nd > 0 {
		parts = append(parts, fmt.Sprintf("#[fg=green]✓%d#[default]", nd))
	}
	if nw > 0 {
		parts = append(parts, fmt.Sprintf("#[fg=yellow]✻%d#[default]", nw))
	}
	opt := strings.Join(parts, " ")
	if opt == d.det.lastOpt {
		return
	}
	d.det.lastOpt = opt
	cmds := []string{"set-option -g @demux_agents " + q(opt)}
	for _, c := range w.Clients {
		cmds = append(cmds, "refresh-client -S -t "+q(c.Name))
	}
	_, _ = ctl.runSeq(cmds...)
}

// wrappedKind resolves an interpreter pane (node/bun) to an agent via the
// process tree, cached per pane_pid. One `ps` exec per UNRESOLVED pid —
// never on the steady-state path.
func (d *daemon) wrappedKind(pid string) string {
	if k, ok := d.det.wrapKind[pid]; ok {
		return k
	}
	kind := ""
	out, err := exec.Command("ps", "-axo", "pid=,ppid=,command=").Output()
	if err == nil {
		children := map[string][]string{}
		cmdOf := map[string]string{}
		for _, ln := range strings.Split(string(out), "\n") {
			fl := strings.Fields(ln)
			if len(fl) < 3 {
				continue
			}
			children[fl[1]] = append(children[fl[1]], fl[0])
			cmdOf[fl[0]] = strings.Join(fl[2:], " ")
		}
		var walk func(p string) string
		walk = func(p string) string {
			for _, c := range children[p] {
				for token := range d.det.manifests {
					cl := cmdOf[c]
					if strings.Contains(cl, "/"+token) || strings.HasPrefix(filepath.Base(strings.Fields(cl + " x")[0]), token) {
						return d.det.manifests[token].id
					}
				}
				if k := walk(c); k != "" {
					return k
				}
			}
			return ""
		}
		kind = walk(pid)
	}
	d.det.wrapKind[pid] = kind
	if kind != "" {
		log.Printf("agent %s resolved from wrapper pid=%s", kind, pid)
	}
	return kind
}

// applyAgentState runs the anti-flap policy and publishes the transition.
// Blocked and working land instantly; working -> idle must survive
// idleConfirms CONSECUTIVE samples (or idleCap of wall time) first — even
// with visible idle evidence (screens exist whose verdict alternates per
// scan; any bypass turns that into a flap). A completion transition
// ((working|blocked) -> idle) in a window nobody is looking at publishes
// as "done" and sticks until the user visits the window.
func (d *daemon) applyAgentState(id string, a *agentInfo, want string, visible bool, vis map[string]bool) bool {
	if want == "idle" && a.state == "done" {
		return false // done IS idle, flagged unseen; keep the flag
	}
	if want == "idle" && a.state == "working" {
		if a.pendingIdle == 0 {
			a.pendingAt = time.Now()
		}
		a.pendingIdle++
		if a.pendingIdle < idleConfirms && time.Since(a.pendingAt) < idleCap {
			return false
		}
	}
	a.pendingIdle = 0
	if want == "idle" && (a.state == "working" || a.state == "blocked") && !vis[a.win] {
		want = "done"
	}
	if a.state == want {
		return false
	}
	log.Printf("agent %s pane=%s state=%s->%s", a.kind, id, orDash(a.state), want)
	a.state = want
	return true
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// --- region helpers shared with manifest.go ---

func lastNonEmpty(lines []string, n int) []string {
	out := make([]string, 0, n)
	for i := len(lines) - 1; i >= 0 && len(out) < n; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			out = append(out, lines[i])
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// isHRule: a line that IS a horizontal rule — leading ─ run of >=3
// (suffix text allowed). Claude's input box borders are exactly this
// (live-verified 2026-08-21).
func isHRule(s string) bool {
	t := strings.TrimSpace(s)
	n := 0
	for _, r := range t {
		if r != '─' {
			break
		}
		n++
	}
	return n >= 3
}

func afterLastHRule(lines []string) []string {
	for i := len(lines) - 1; i >= 0; i-- {
		if isHRule(lines[i]) {
			return lines[i+1:]
		}
	}
	return nil
}

// promptBoxBody: the text between the 2nd-from-last horizontal rule and the
// rule after it — claude's ─-bordered input box.
func promptBoxBody(lines []string) []string {
	last, second := -1, -1
	for i := len(lines) - 1; i >= 0; i-- {
		if isHRule(lines[i]) {
			if last == -1 {
				last = i
			} else {
				second = i
				break
			}
		}
	}
	if second == -1 {
		return nil
	}
	return lines[second+1 : last]
}
