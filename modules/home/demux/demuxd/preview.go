package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

// The preview engine: capture a window, ship it to the TUI as a billboard
// frame, and keep the current target live on a stream ticker. Docked scrub
// zoom is the only consumer (`demuxd browse` is that too, pre-zoomed).

// previewState is the engine's state on the daemon.
type previewState struct {
	target string // currently previewed window

	// Delta stream state: the target's last shipped grid. Stream ticks diff
	// against it and ship only the changed rows (a claude spinner tick is
	// 1-3 rows, not a 100KB frame); hello replays marshal it whole.
	lastPanes []framePane
	frameGen  int

	// Per-pane scrollback offsets (lines back from live) for the current
	// target: wheel-up on a billboard split scrolls its history without
	// touching the real pane. Ephemeral — cleared on target change and reset.
	scroll map[string]int

	// Capture gate: geometry fingerprint and #{window_activity} of the last
	// capture. A stream tick whose target shows the same rects and activity
	// — with the last capture a full second past the activity stamp
	// (activity has 1s resolution, so a same-second capture may predate
	// trailing output) — skips capture, selfContain, and marshal wholesale.
	lastRects    string
	lastActivity int64
	lastCapture  time.Time
	gated        bool // gate currently holding (logs the idle EDGE, not every tick)

	// The live stream: while billboards are showing, the target is
	// re-captured on this ticker (10fps) so previews stay live — scrolling
	// logs, agent output. Frames are dropped when content hasn't changed,
	// and the ticker exists only while billboards are up (tickC is nil
	// otherwise, so the consume-loop case simply never fires). %output can
	// narrow this later for same-session targets; cross-session panes emit
	// no output events at all, so capture is the only universal source.
	ticker *time.Ticker
	tickC  <-chan time.Time
}

// reset forgets the delta and gate state, so the next preview captures and
// ships a full frame unconditionally (scrub start, target teardown).
func (pv *previewState) reset() {
	pv.lastPanes, pv.lastRects, pv.lastActivity = nil, "", 0
	pv.scroll = nil
}

// frameBytes marshals the cached target grid as a full frame, for replaying
// to a late-connecting TUI.
func (pv *previewState) frameBytes() []byte {
	return marshalLine(frameMsg{Type: "frame", Window: pv.target, Panes: pv.lastPanes, Gen: pv.frameGen})
}

func (d *daemon) startStream() {
	if d.pv.ticker == nil {
		d.pv.ticker = time.NewTicker(100 * time.Millisecond)
		d.pv.tickC = d.pv.ticker.C
	}
}

func (d *daemon) stopStream() {
	if d.pv.ticker != nil {
		d.pv.ticker.Stop()
		d.pv.ticker = nil
		d.pv.tickC = nil
	}
}

// selfContain rewrites captured lines so each stands alone. capture-pane -e
// emits per-line sequences RELATIVE to the previous line's ending state
// (probe-verified: a bg opened on one row and never reset paints following
// rows with no sequence at all). The billboard painter positions and resets
// per line — carried state would be lost, washing panel backgrounds off
// rows seemingly at random — so each line gets its carry-in prepended.
func selfContain(lines []string) {
	var st sgrState
	for i, ln := range lines {
		if pre := st.seq(); pre != "" {
			lines[i] = pre + ln
		}
		st.fold(ln)
	}
}

// sgrState models the attributes an SGR stream leaves active: one fg, one
// bg, one underline color, and a small flag set. A real state model — not
// a parameter log — so compound params (38;2;r;g;b) can never be split and
// the carry stays bounded no matter how color-dense the frame is (an
// append-log capped by dropping led params cut compounds mid-way, which
// corrupted exactly the BOTTOM rows of dense frames, where the most state
// had accumulated).
type sgrState struct {
	fg, bg, ul string
	flags      []string
}

// seq renders the state as one SGR sequence, empty when default.
func (s *sgrState) seq() string {
	parts := make([]string, 0, len(s.flags)+3)
	parts = append(parts, s.flags...)
	for _, c := range []string{s.fg, s.bg, s.ul} {
		if c != "" {
			parts = append(parts, c)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "\x1b[" + strings.Join(parts, ";") + "m"
}

func (s *sgrState) setFlag(f string) {
	for _, have := range s.flags {
		if have == f {
			return
		}
	}
	if len(s.flags) < 12 {
		s.flags = append(s.flags, f)
	}
}

func (s *sgrState) dropFlags(match func(string) bool) {
	out := s.flags[:0]
	for _, f := range s.flags {
		if !match(f) {
			out = append(out, f)
		}
	}
	s.flags = out
}

// fold applies every SGR sequence in a captured line to the state.
func (s *sgrState) fold(ln string) {
	for i := 0; i < len(ln); {
		j := strings.Index(ln[i:], "\x1b[")
		if j < 0 {
			break
		}
		i += j + 2
		k := i
		for k < len(ln) && !(ln[k] >= 0x40 && ln[k] <= 0x7e) {
			k++
		}
		if k >= len(ln) {
			break
		}
		if ln[k] == 'm' {
			s.apply(strings.Split(ln[i:k], ";"))
		}
		i = k + 1
	}
}

func (s *sgrState) apply(tokens []string) {
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		// colon compounds (38:2:r:g:b, 4:3) are self-contained tokens
		if c := strings.IndexByte(t, ':'); c >= 0 {
			switch t[:c] {
			case "38":
				s.fg = t
			case "48":
				s.bg = t
			case "58":
				s.ul = t
			case "4":
				s.setFlag(t)
			}
			continue
		}
		switch t {
		case "", "0":
			*s = sgrState{flags: s.flags[:0]}
		case "38", "48", "58":
			var span int
			if i+1 < len(tokens) && tokens[i+1] == "2" && i+4 < len(tokens) {
				span = 5
			} else if i+1 < len(tokens) && tokens[i+1] == "5" && i+2 < len(tokens) {
				span = 3
			} else {
				continue // malformed; skip the introducer
			}
			val := strings.Join(tokens[i:i+span], ";")
			switch t {
			case "38":
				s.fg = val
			case "48":
				s.bg = val
			default:
				s.ul = val
			}
			i += span - 1
		case "39":
			s.fg = ""
		case "49":
			s.bg = ""
		case "59":
			s.ul = ""
		case "22":
			s.dropFlags(func(f string) bool { return f == "1" || f == "2" })
		case "23":
			s.dropFlags(func(f string) bool { return f == "3" })
		case "24":
			s.dropFlags(func(f string) bool { return f == "4" || strings.HasPrefix(f, "4:") })
		case "25":
			s.dropFlags(func(f string) bool { return f == "5" || f == "6" })
		case "27":
			s.dropFlags(func(f string) bool { return f == "7" })
		case "28":
			s.dropFlags(func(f string) bool { return f == "8" })
		case "29":
			s.dropFlags(func(f string) bool { return f == "9" })
		default:
			n, err := strconv.Atoi(t)
			switch {
			case err != nil:
			case (n >= 30 && n <= 37) || (n >= 90 && n <= 97):
				s.fg = t
			case (n >= 40 && n <= 47) || (n >= 100 && n <= 107):
				s.bg = t
			case n >= 1 && n <= 9:
				s.setFlag(t)
			}
		}
	}
}

// parseDims splits a "W<sep>H" display-message line.
func parseDims(line string) (int, int) {
	p := strings.Split(line, sep)
	if len(p) != 2 {
		return 0, 0
	}
	w, _ := strconv.Atoi(p[0])
	h, _ := strconv.Atoi(p[1])
	return w, h
}

// Geometry is queried fresh (cross-session geometry emits no events), then
// every pane is captured in one sequence with marker lines between panes:
// capture output line counts are not reliable (trailing blanks), markers are.
const frameMarker = "\x1fdemux-frame\x1f"

// preview captures wid and ships it as a frame. A prefetch warms the TUI's
// cache for adjacent rows without becoming the streamed target. The sidebar
// pane itself is never billboarded (a docked window's frame is its mains,
// shifted to the canvas origin — near-pixel-parity with entering it).
//
// stream marks the ticker's re-captures of the current target: only those
// may skip the capture (activity gate) or ship row deltas. Command-driven
// previews always capture and always ship FULL frames — they double as the
// resync path when a TUI drops a delta it can't apply.
func (d *daemon) preview(ctl *control, wid string, prefetch, stream bool) error {
	if d.dock == nil {
		return nil
	}
	// Docked scrub targets get their spacer carved on first billboard: a
	// hidden 40-col pane occupies the sidebar's slot (the dock's own split,
	// so the carve arithmetic is identical), and the billboard then IS the
	// docked reality — same wraps, same borders. Entering later is a
	// geometry-free swap into that slot. Once per window while docked;
	// skipped when another client is attached to that window's session
	// (carving would visibly resize it under them), and skipped for windows
	// carrying huge scrollback — tmux reflows a pane's ENTIRE history
	// synchronously on width change (~250ms at 660k lines, live-measured),
	// which stalls the server mid-scrub for a billboard the user is only
	// glancing at. Those windows billboard as scaled approximations and pay
	// their carve if and when actually entered.
	canCarve := wid != d.dock.win
	skipPane := ""
	if canCarve {
		if t := d.dock.carved[wid]; t != nil {
			skipPane = t.spacer
		}
	}
	var panes []framePane
	var caps []string
	var rects string
	var activity int64
	for attempt := 0; ; attempt++ {
		query := []string{
			"list-panes -t " + q(wid) + " -F " +
				f("#{pane_id}", "#{pane_left}", "#{pane_top}", "#{pane_width}", "#{pane_height}", "#{pane_active}", "#{history_size}"),
			"display-message -p -t " + q(wid) + " -F " + f("#{window_width}", "#{window_height}", "#{window_activity}"),
			"display-message -p -t " + q(d.dock.win) + " -F " + f("#{window_width}", "#{window_height}"),
		}
		lines, err := ctl.runSeq(query...)
		if err != nil {
			return err
		}
		tgtW, tgtH, dockW, dockH := 0, 0, 0, 0
		if len(lines) >= 2 {
			if p := strings.Split(lines[len(lines)-2], sep); len(p) == 3 {
				tgtW, _ = strconv.Atoi(p[0])
				tgtH, _ = strconv.Atoi(p[1])
				activity, _ = strconv.ParseInt(p[2], 10, 64)
			}
			dockW, dockH = parseDims(lines[len(lines)-1])
			lines = lines[:len(lines)-2]
		}
		// A window not viewed since the client changed size (monitor switch,
		// other sessions) keeps STALE dimensions until entered: its billboard
		// paints offset (content clipped or short) and the eventual entry
		// pays a surprise resize reflow. Normalize to the docked window's
		// size while billboarding instead.
		sizeStale := dockW > 0 && (tgtW != dockW || tgtH != dockH)
		panes, caps, rects = panes[:0], caps[:0], ""
		history := 0
		for _, ln := range lines {
			p := strings.Split(ln, sep)
			if len(p) != 7 {
				continue
			}
			rects += strings.Join(p[:6], ",") + ";"
			if h, _ := strconv.Atoi(p[6]); h > 0 {
				history += h
			}
			if p[0] == d.dock.pane || (skipPane != "" && p[0] == skipPane) {
				continue
			}
			left, _ := strconv.Atoi(p[1])
			width, _ := strconv.Atoi(p[3])
			top, _ := strconv.Atoi(p[2])
			height, _ := strconv.Atoi(p[4])
			off := d.pv.scroll[p[0]]
			panes = append(panes, framePane{ID: p[0], Left: left, Top: top, Width: width, Height: height, Active: p[5] == "1", Scroll: off})
			caps = append(caps, captureCmd(p[0], height, off), "display-message -p "+q(frameMarker))
		}
		if len(panes) == 0 {
			return fmt.Errorf("no panes in %s", wid)
		}
		if canCarve && attempt == 0 && d.dock.carved[wid] == nil &&
			history <= carveHistoryMax && !d.otherClientOn(wid) {
			// The resize (when stale) rides the carve batch: the orig layout
			// is captured AFTER it, so a later release restores the window
			// full-width at its normalized size. window-size snaps back to
			// latest immediately — the size sticks (no client event), and
			// future client resizes propagate normally.
			var seq []string
			if sizeStale {
				seq = append(seq, fmt.Sprintf("resize-window -x %d -y %d -t %s", dockW, dockH, q(wid)))
			}
			seq = append(seq,
				"display-message -p -t "+q(wid)+" -F "+f("#{window_layout}", "#{automatic-rename}"),
				"set-option -w -t "+q(wid)+" automatic-rename off",
				fmt.Sprintf("split-window -d -hb -f -l %d -P -F '#{pane_id}' -t %s %s",
					listWidth, q(wid), q(spacerCmd)))
			if sizeStale {
				seq = append(seq, "set-option -w -t "+q(wid)+" window-size latest")
			}
			out, err := ctl.runSeq(seq...)
			if err == nil && len(out) >= 2 {
				t := &carveState{spacer: out[1]}
				if parts := strings.Split(out[0], sep); len(parts) == 2 {
					t.orig, t.autoRename = parts[0], parts[1]
				}
				d.dock.carved[wid] = t
				skipPane = t.spacer
				d.lastScrub = time.Now()
				if bench {
					log.Printf("bench carve win=%s spacer=%s", wid, t.spacer)
				}
				continue // recapture at the docked geometry
			}
		}
		// Carve-skipped windows (huge scrollback) still get the CHEAP half:
		// height-only normalization never rewraps history — only width
		// changes pay the reflow — so their billboards align vertically for
		// free. A stale WIDTH on such a window stays stale (the reflow is
		// the very thing being avoided); its billboard remains an X-scaled
		// approximation.
		if attempt == 0 && wid != d.dock.win && d.dock.carved[wid] == nil &&
			sizeStale && tgtW == dockW && !d.otherClientOn(wid) {
			if _, err := ctl.runSeq(
				fmt.Sprintf("resize-window -y %d -t %s", dockH, q(wid)),
				"set-option -w -t "+q(wid)+" window-size latest"); err == nil {
				continue // recapture at the corrected height
			}
		}
		break
	}
	// Activity gate: a streamed target whose pane rects and last-activity
	// stamp both match the previous capture — with that capture at least a
	// full second newer than the stamp, so no same-second trailing output
	// can postdate it — has nothing new to show. Skip the capture batch
	// entirely; the tick already paid only three format expansions.
	if stream && wid == d.pv.target && d.pv.lastPanes != nil &&
		rects == d.pv.lastRects && activity == d.pv.lastActivity &&
		d.pv.lastCapture.Unix() >= activity+1 {
		if bench && !d.pv.gated {
			log.Printf("bench gate idle win=%s", wid)
		}
		d.pv.gated = true
		return nil
	}
	minLeft := panes[0].Left
	for _, p := range panes {
		if p.Left < minLeft {
			minLeft = p.Left
		}
	}
	if minLeft > 0 {
		for i := range panes {
			panes[i].Left -= minLeft
		}
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
	for i := range panes {
		selfContain(panes[i].Lines)
	}
	if bench {
		rects := make([]string, len(panes))
		for i, p := range panes {
			rects[i] = fmt.Sprintf("%d,%d %dx%d", p.Left, p.Top, p.Width, p.Height)
		}
		log.Printf("bench frame win=%s prefetch=%v rects=%v", wid, prefetch, rects)
	}
	if prefetch {
		d.h.sendRole("list", marshalLine(frameMsg{Type: "frame", Window: wid, Panes: panes}))
		return nil
	}
	if d.pv.target != wid {
		d.pv.scroll = nil // scroll offsets are a per-target affordance
	}
	d.pv.target = wid
	stamp := func() {
		d.pv.lastRects, d.pv.lastActivity, d.pv.lastCapture = rects, activity, time.Now()
		d.pv.gated = false
	}
	if stream && sameFrameShape(d.pv.lastPanes, panes) {
		delta, nrows := deltaPanes(d.pv.lastPanes, panes)
		if delta == nil {
			stamp()
			return nil // idle content: no repaint
		}
		base := d.pv.frameGen
		d.pv.frameGen++
		d.pv.lastPanes = panes
		stamp()
		if bench {
			log.Printf("bench frame delta win=%s panes=%d rows=%d", wid, len(delta), nrows)
		}
		d.h.sendRole("list", marshalLine(frameMsg{
			Type: "frame", Window: wid, Panes: delta, Delta: true, Base: base, Gen: d.pv.frameGen}))
		return nil
	}
	d.pv.frameGen++
	d.pv.lastPanes = panes
	stamp()
	d.h.sendRole("list", marshalLine(frameMsg{Type: "frame", Window: wid, Panes: panes, Gen: d.pv.frameGen}))
	return nil
}

// captureCmd builds one pane's capture: the visible screen, or — scrolled
// off lines back into history — the window [-off, height-1-off] (capture
// line 0 is the top of the visible screen, negatives reach into history).
func captureCmd(id string, height, off int) string {
	if off <= 0 {
		return "capture-pane -e -p -t " + q(id)
	}
	return fmt.Sprintf("capture-pane -e -p -t %s -S %d -E %d", q(id), -off, height-1-off)
}

// scrollPreview adjusts one billboard split's scrollback offset (wheel over
// the canvas) and re-ships the frame. The real pane is untouched — no
// copy-mode, no interference with whatever runs there; this is a read-only
// view into its history, clamped to what actually exists.
func (d *daemon) scrollPreview(ctl *control, pane string, delta int) error {
	if d.dock == nil || !d.dock.scrubbing || d.pv.target == "" || pane == "" || delta == 0 {
		return nil
	}
	out, err := ctl.run("display-message -p -t " + q(pane) + " -F '#{history_size}'")
	if err != nil || len(out) == 0 {
		return err
	}
	hist, _ := strconv.Atoi(out[0])
	off := d.pv.scroll[pane] + delta
	if off < 0 {
		off = 0
	}
	if off > hist {
		off = hist
	}
	if d.pv.scroll[pane] == off {
		return nil
	}
	if d.pv.scroll == nil {
		d.pv.scroll = map[string]int{}
	}
	d.pv.scroll[pane] = off
	// Content moved without any pane output: the activity gate would hold
	// the stale view, so bust it for this one recapture.
	d.pv.lastRects = ""
	return d.preview(ctl, d.pv.target, false, true)
}

// sameFrameShape reports an identical pane set — ids, rects, active — the
// precondition for shipping row deltas. An id change behind the same rect
// (respawn) means different content history; ship full instead.
func sameFrameShape(a, b []framePane) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Left != b[i].Left || a[i].Top != b[i].Top ||
			a[i].Width != b[i].Width || a[i].Height != b[i].Height || a[i].Active != b[i].Active {
			return false
		}
	}
	return true
}

// deltaPanes diffs same-shape frames row-wise: per pane, the changed rows
// (Rows holds indices, Lines the new content; rows the new capture lost
// come through as ""). nil means identical content.
func deltaPanes(old, cur []framePane) ([]framePane, int) {
	var out []framePane
	total := 0
	for i := range cur {
		o, n := old[i].Lines, cur[i].Lines
		rows := len(n)
		if len(o) > rows {
			rows = len(o)
		}
		var idx []int
		var lns []string
		for r := 0; r < rows; r++ {
			ol, nl := "", ""
			if r < len(o) {
				ol = o[r]
			}
			if r < len(n) {
				nl = n[r]
			}
			if ol != nl {
				idx = append(idx, r)
				lns = append(lns, nl)
			}
		}
		if idx != nil {
			p := cur[i]
			p.Lines, p.Rows = lns, idx
			out = append(out, p)
			total += len(idx)
		}
	}
	return out, total
}
