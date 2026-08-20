package main

import (
	"bytes"
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
	target    string // currently previewed window
	lastFrame []byte // last target frame, replayed to late-connecting TUIs

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
	var state []string
	for i, ln := range lines {
		if len(state) > 0 {
			lines[i] = "\x1b[" + strings.Join(state, ";") + "m" + ln
		}
		state = sgrFold(state, ln)
	}
}

// sgrFold accumulates the SGR parameters a line leaves active. Parameters
// are replayed in order, so compound forms (38;2;r;g;b) survive intact; a
// bare 0 (or empty) resets. Capped so a pathological pane can't balloon
// every following line.
func sgrFold(state []string, ln string) []string {
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
			params := ln[i:k]
			if params == "" {
				params = "0"
			}
			for _, p := range strings.Split(params, ";") {
				if p == "0" || p == "" {
					state = state[:0]
				} else {
					state = append(state, p)
				}
			}
			if len(state) > 64 {
				state = append(state[:0], state[len(state)-64:]...)
			}
		}
		i = k + 1
	}
	return state
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
func (d *daemon) preview(ctl *control, wid string, prefetch bool) error {
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
	for attempt := 0; ; attempt++ {
		query := []string{
			"list-panes -t " + q(wid) + " -F " +
				f("#{pane_id}", "#{pane_left}", "#{pane_top}", "#{pane_width}", "#{pane_height}", "#{pane_active}", "#{history_size}"),
			"display-message -p -t " + q(wid) + " -F " + f("#{window_width}", "#{window_height}"),
			"display-message -p -t " + q(d.dock.win) + " -F " + f("#{window_width}", "#{window_height}"),
		}
		lines, err := ctl.runSeq(query...)
		if err != nil {
			return err
		}
		tgtW, tgtH, dockW, dockH := 0, 0, 0, 0
		if len(lines) >= 2 {
			tgtW, tgtH = parseDims(lines[len(lines)-2])
			dockW, dockH = parseDims(lines[len(lines)-1])
			lines = lines[:len(lines)-2]
		}
		// A window not viewed since the client changed size (monitor switch,
		// other sessions) keeps STALE dimensions until entered: its billboard
		// paints offset (content clipped or short) and the eventual entry
		// pays a surprise resize reflow. Normalize to the docked window's
		// size while billboarding instead.
		sizeStale := dockW > 0 && (tgtW != dockW || tgtH != dockH)
		panes, caps = panes[:0], caps[:0]
		history := 0
		for _, ln := range lines {
			p := strings.Split(ln, sep)
			if len(p) != 7 {
				continue
			}
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
			panes = append(panes, framePane{ID: p[0], Left: left, Top: top, Width: width, Height: height, Active: p[5] == "1"})
			caps = append(caps, "capture-pane -e -p -t "+q(p[0]), "display-message -p "+q(frameMarker))
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
	payload := marshalLine(frameMsg{Type: "frame", Window: wid, Panes: panes})
	if prefetch {
		d.h.sendRole("list", payload)
		return nil
	}
	d.pv.target = wid
	if bytes.Equal(payload, d.pv.lastFrame) {
		return nil // idle content: no repaint
	}
	d.pv.lastFrame = payload
	d.h.sendRole("list", payload)
	return nil
}
