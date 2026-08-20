package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
)

// The sidebar TUI: one process rendering the session/window list (left, 40
// cols) and, when its pane is zoomed for scrubbing, the live preview canvas
// (right) in the same pane. Frames are
// cached per window, so scrubbing paints the cached frame in ~0ms — the
// herdr/choose-tree trick: state lives locally, painting is local. The fresh
// frame (and the 10fps live stream) replaces it moments later. The daemon
// owns every tmux interaction; this process spawns nothing.

// benchLog, when DEMUX_BENCH is set, records per-event timings so latency can
// be attributed (key handling vs paint cost vs frame arrival).
var benchLog *os.File

func benchf(format string, args ...any) {
	if benchLog == nil {
		return
	}
	fmt.Fprintf(benchLog, "%s tui ", time.Now().Format("15:04:05.000000"))
	fmt.Fprintf(benchLog, format+"\n", args...)
}

// tuiLog is ALWAYS on (<demux sock>.tui.log): low-volume lifecycle events —
// which build started, what select arrived and where it resolved — so field
// reports can be diagnosed from logs instead of guesses.
var tuiLog *os.File

func tlogf(format string, args ...any) {
	if tuiLog == nil {
		return
	}
	fmt.Fprintf(tuiLog, "%s ", time.Now().Format("01-02 15:04:05.000"))
	fmt.Fprintf(tuiLog, format+"\n", args...)
}

type store struct {
	sessions map[string]session
	windows  map[string]window
	panes    map[string]pane
}

func (st *store) apply(m wireMsg) {
	switch m.Type {
	case "snapshot":
		st.sessions = map[string]session{}
		st.windows = map[string]window{}
		st.panes = map[string]pane{}
		for _, s := range m.Sessions {
			st.sessions[s.ID] = s
		}
		for _, w := range m.Windows {
			st.windows[w.ID] = w
		}
		for _, p := range m.Panes {
			st.panes[p.ID] = p
		}
	case "diff":
		for _, o := range m.Ops {
			switch o.Kind {
			case "session":
				if o.Op == "del" {
					delete(st.sessions, o.ID)
				} else {
					var v session
					if json.Unmarshal(o.Value, &v) == nil {
						st.sessions[o.ID] = v
					}
				}
			case "window":
				if o.Op == "del" {
					delete(st.windows, o.ID)
				} else {
					var v window
					if json.Unmarshal(o.Value, &v) == nil {
						st.windows[o.ID] = v
					}
				}
			case "pane":
				if o.Op == "del" {
					delete(st.panes, o.ID)
				} else {
					var v pane
					if json.Unmarshal(o.Value, &v) == nil {
						st.panes[o.ID] = v
					}
				}
			}
		}
	}
}

type row struct {
	label   string
	window  string // preview target
	session bool
}

func (st *store) rows() []row {
	sessions := make([]session, 0, len(st.sessions))
	for _, s := range st.sessions {
		sessions = append(sessions, s)
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].Name < sessions[j].Name })

	var out []row
	for _, s := range sessions {
		wins := make([]window, 0, 8)
		for _, w := range st.windows {
			if w.SessionID == s.ID {
				wins = append(wins, w)
			}
		}
		sort.Slice(wins, func(i, j int) bool { return wins[i].Index < wins[j].Index })
		activeWin := ""
		for _, w := range wins {
			if w.Active {
				activeWin = w.ID
			}
		}
		att := " "
		if s.Attached {
			att = "●"
		}
		out = append(out, row{label: fmt.Sprintf("%s %s", att, s.Name), window: activeWin, session: true})
		for _, w := range wins {
			mark := " "
			if w.Active {
				mark = "*"
			}
			out = append(out, row{label: fmt.Sprintf("   %d%s %s", w.Index, mark, w.Name), window: w.ID})
		}
	}
	return out
}

func cmdTui(tmuxSock, demuxSock string) {
	conn, err := dialEnsure(tmuxSock, demuxSock)
	if err != nil {
		fmt.Fprintf(os.Stderr, "demuxd tui: %v\r\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	if os.Getenv("DEMUX_BENCH") != "" {
		benchLog, _ = os.OpenFile(demuxSock+".tui-bench.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	}
	tuiLog, _ = os.OpenFile(demuxSock+".tui.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	exe, _ := os.Executable()
	cols, height := surfaceSize()
	tlogf("start build=%s pane=%s size=%dx%d", exe, os.Getenv("TMUX_PANE"), cols, height)

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err == nil {
		defer term.Restore(int(os.Stdin.Fd()), oldState)
	}
	// Hide cursor; disable autowrap so frame lines wider than the preview
	// region clip at the right edge instead of wrapping over the layout.
	fmt.Print("\033[?25l\033[?7l")
	defer fmt.Print("\033[?25h\033[?7h")

	msgs := make(chan wireMsg, 64)
	go func() {
		sc := bufio.NewScanner(conn)
		sc.Buffer(make([]byte, 64*1024), 64*1024*1024)
		for sc.Scan() {
			var m wireMsg
			if json.Unmarshal(sc.Bytes(), &m) == nil {
				msgs <- m
			}
		}
		close(msgs)
	}()

	keys := make(chan byte, 16)
	go func() {
		buf := make([]byte, 64)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				close(keys)
				return
			}
			for _, b := range buf[:n] {
				keys <- b
			}
		}
	}()

	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	var winchT *time.Timer
	var winchC <-chan time.Time

	send := func(m cmdMsg) {
		m.Type = "cmd"
		b, _ := json.Marshal(m)
		conn.Write(append(b, '\n'))
	}
	conn.Write([]byte(`{"type":"hello","role":"list"}` + "\n"))

	st := &store{}
	sel := 0
	esc := 0 // escape-sequence state for arrow keys
	var rows []row

	// Per-window frame cache with generations, so "did I already paint
	// exactly this?" is an integer compare, never a deep one.
	type cached struct {
		gen   int
		panes []framePane
		at    time.Time
	}
	// A cache older than this never paints: it exists to make ACTIVE
	// scrubbing instant (frames milliseconds old), but a window last
	// billboarded minutes ago is ancient content — painting it flashes the
	// old screen for the ~50ms until the fresh capture lands, which reads
	// as random flicker. Better to leave the canvas and wait.
	const frameTTL = 3 * time.Second
	frames := map[string]cached{}
	gen := 0
	paintedWin, paintedGen := "", -1
	var paintedPanes []framePane

	target := func() string {
		if sel >= 0 && sel < len(rows) {
			return rows[sel].window
		}
		return ""
	}
	// Painting is split: a selection move repaints the list column only, a
	// frame repaints the preview region only. Neither clears the screen —
	// everything is positioned overwrites, so tmux's own cell diff decides
	// what actually reaches the terminal (a selection move is ~2 changed
	// rows, not 45k cells). Streaming updates of the same window diff at
	// line level: a claude spinner tick repaints 1-3 lines, not the region.
	paintFrameFor := func(win string) {
		c, ok := frames[win]
		if !ok || (win == paintedWin && c.gen == paintedGen) {
			return
		}
		if time.Since(c.at) > frameTTL {
			benchf("stale skip win=%s age_ms=%d", win, time.Since(c.at).Milliseconds())
			return // stale: the fresh frame is already on its way
		}
		cols, _ := surfaceSize()
		avail := cols - (listWidth + 1)
		if avail <= 0 {
			return // narrow (docked): no canvas; keep the cache unmarked
		}
		// Frames are cached RAW and scaled at paint time, so a resize (zoom
		// in/out) just re-scales on the next paint — no cache invalidation.
		scaled := scaleFrame(c.panes, avail)
		var prev []framePane
		if win == paintedWin && sameGeometry(paintedPanes, scaled) {
			prev = paintedPanes
		}
		paintFrame(scaled, prev)
		paintedWin, paintedGen, paintedPanes = win, c.gen, scaled
	}
	paintAll := func() {
		rows = st.rows()
		if sel >= len(rows) {
			sel = len(rows) - 1
		}
		if sel < 0 {
			sel = 0
		}
		paintList(rows, sel)
		paintedWin, paintedGen = "", -1 // size/world may have shifted regions
		paintFrameFor(target())
	}
	// shrinkExpected: a commit/close was sent from wide mode, so the pane is
	// about to shrink to 40 cols. tmux REWRAPS the grid on width change —
	// the canvas mashes into a wall of wrapped text in the 40-col strip (the
	// "blob" flicker) until the winch repaint covers it. Painting anything
	// more into the canvas meanwhile only widens that window (a 100KB frame
	// write can delay the covering repaint ~100ms), so canvas paints pause
	// until the winch lands. Pre-CLEARING instead is worse: tmux flushes the
	// clear as its own sync-wrapped frame (probe-verified), which blanks the
	// billboard visibly before the swap.
	shrinkExpected := false
	// Browse mode: cached frame paints locally NOW; the daemon refreshes it
	// (and streams it live) right behind, neighbors prefetched warm. Docked
	// (narrow) mode: the same preview cmd IS the scrub — the daemon moves
	// the real main area to the target window; prefetch means nothing.
	requestFrames := func() {
		cur := target()
		if cur != "" {
			send(cmdMsg{Cmd: "preview", Window: cur})
		}
		if narrowMode() {
			return
		}
		seen := map[string]bool{cur: true, "": true}
		for _, i := range []int{sel - 1, sel + 1} {
			if i >= 0 && i < len(rows) && !seen[rows[i].window] {
				seen[rows[i].window] = true
				send(cmdMsg{Cmd: "preview", Window: rows[i].window, Prefetch: true})
			}
		}
	}
	// moveSel clamps and applies a selection delta; painting is the caller's
	// job — the key loop coalesces a drained batch into one paint.
	moveSel := func(delta int) bool {
		if len(rows) == 0 {
			return false
		}
		next := sel + delta
		if next < 0 {
			next = 0
		}
		if next >= len(rows) {
			next = len(rows) - 1
		}
		if next == sel {
			return false
		}
		sel = next
		return true
	}

	// The process is per-dock: spawned by dockOpen, killed with its pane at
	// undock. It also exits itself when the daemon connection closes, so a
	// dead daemon never leaves a zombie sidebar pane behind.
	for {
		select {
		case m, ok := <-msgs:
			if !ok {
				tlogf("exit: daemon connection closed")
				return
			}
			switch m.Type {
			case "reply":
				// A failed commit/close means the shrink is never coming —
				// unfreeze canvas painting (the stream refills it in a tick).
				if shrinkExpected && m.OK != nil && !*m.OK {
					shrinkExpected = false
					paintFrameFor(target())
				}
			case "snapshot", "diff":
				// Selection is sticky to the ROW IDENTITY, not the index:
				// world churn (a window or session appearing/dying — agents
				// do this constantly) rebuilds rows, and an index-anchored
				// highlight visibly jumps to whatever slid into its slot.
				prevWin, prevSession := "", false
				if sel >= 0 && sel < len(rows) {
					prevWin, prevSession = rows[sel].window, rows[sel].session
				}
				st.apply(m)
				rows = st.rows()
				if prevWin != "" {
					for i, r := range rows {
						if r.window == prevWin && r.session == prevSession {
							sel = i
							break
						}
					}
				}
				if sel >= len(rows) {
					sel = len(rows) - 1
				}
				paintList(rows, sel)
			case "select":
				found := false
				for i, r := range rows {
					if r.window == m.Window && !r.session {
						sel = i
						found = true
						break
					}
				}
				tlogf("select win=%s found=%v sel=%d rows=%d", m.Window, found, sel, len(rows))
				paintList(rows, sel)
				if !shrinkExpected {
					paintFrameFor(target())
					requestFrames()
				}
			case "frame":
				benchf("frame win=%s current=%v", m.Window, m.Window == target())
				// Input outranks painting: with keys already queued, a full
				// frame paint (20-120ms in a saturated tmux) would land
				// behind the selection the user has already left. The frame
				// is cached either way — the key batch paints it if the
				// target still matches, the stream re-covers it otherwise.
				quiet := len(keys) == 0 && !shrinkExpected
				if prev, ok := frames[m.Window]; ok && framesEqual(prev.panes, m.Frame) {
					// Confirming frame, content unchanged: no repaint, but
					// the cache is proven current — restamp it, or a
					// static window's cache would age out while streaming.
					// Target-only paint: a stale-skip may have left the
					// canvas waiting for exactly this confirmation.
					prev.at = time.Now()
					frames[m.Window] = prev
					if m.Window == target() && quiet {
						paintFrameFor(m.Window)
					}
					break
				}
				gen++
				frames[m.Window] = cached{gen: gen, panes: m.Frame, at: time.Now()}
				if m.Window == target() && quiet {
					paintFrameFor(m.Window)
				}
			}
		case b, ok := <-keys:
			if !ok {
				return
			}
			// Coalesce: drain every key already queued and paint ONCE for
			// the final selection. A held j autorepeats faster than tmux
			// drains full-canvas paints (20-120ms each, log-measured on a
			// 440x95 client); painting every intermediate row shoves ~100KB
			// a step into the saturated pty and the input queue backs up —
			// the held-scrub mush. NOT a debounce: nothing waits, a lone key
			// finds the queue empty and paints immediately.
			moved := false
			for {
				switch {
				case esc == 1 && b == '[':
					esc = 2
				case esc == 2:
					esc = 0
					if b == 'A' {
						moved = moveSel(-1) || moved
					} else if b == 'B' {
						moved = moveSel(1) || moved
					}
				case b == 0x1b:
					esc = 1
				// vim-tmux-navigator hands its keys to this pane (the
				// @vim_navigator_pattern includes demuxd), so the sidebar
				// behaves like a vim split: C-l goes INTO what you're looking
				// at — the billboarded window mid-scrub, the main pane when
				// docked idle — never "escapes" back to the docked window's
				// hidden panes via a raw unzoom. C-j/C-k mirror j/k. C-h has
				// nowhere left to go and is ignored.
				case b == 'j', b == 0x0a: // j, ctrl-j
					moved = moveSel(1) || moved
				case b == 'k', b == 0x0b: // k, ctrl-k
					moved = moveSel(-1) || moved
				case b == '\r': // enter
					shrinkExpected = !narrowMode()
					send(cmdMsg{Cmd: "commit", Window: target()})
				case b == 0x0c: // ctrl-l
					if narrowMode() {
						// Docked idle: C-l is the navigator's "pane to the
						// right" — the pane NEXT to the sidebar, not the
						// window's last-active one (commit would skip splits).
						send(cmdMsg{Cmd: "focus"})
					} else {
						// Zoomed billboard / full-screen browse: C-l goes INTO
						// what you're looking at, like Enter.
						shrinkExpected = true
						send(cmdMsg{Cmd: "commit", Window: target()})
					}
				case b == 'q', b == 0x03: // q, ctrl-c
					shrinkExpected = !narrowMode()
					send(cmdMsg{Cmd: "close"})
				default:
					esc = 0
				}
				more := false
				select {
				case b, ok = <-keys:
					if !ok {
						return
					}
					more = true
				default:
				}
				if !more {
					break
				}
			}
			if moved {
				benchf("key sel=%d target=%s cached=%v", sel, target(), frames[target()].panes != nil)
				paintList(rows, sel)
				if !shrinkExpected {
					paintFrameFor(target())
					requestFrames()
				}
			}
		case <-winch:
			shrinkExpected = false // the resize this was armed for has landed
			paintAll()
			// A client resize (monitor switch) rescales the docked sidebar
			// off its fixed width, and no tmux notification crosses sessions
			// to tell the daemon — this SIGWINCH is the only signal. Report
			// it DELAYED: zoom transitions fire SIGWINCH too, and an instant
			// winch cmd races into the daemon's queue right as scrub-start
			// prefetches are dispatched, making them look superseded (they
			// get abandoned, killing the billboard cache warming). A monitor
			// switch doesn't care about a 250ms wait.
			if winchT != nil {
				winchT.Stop()
			}
			winchT = time.NewTimer(250 * time.Millisecond)
			winchC = winchT.C
		case <-winchC:
			winchC = nil
			if cols, _ := surfaceSize(); cols != listWidth {
				send(cmdMsg{Cmd: "winch", Width: cols})
			}
		}
	}
}

// scaleFrame maps a frame captured at its window's real width onto the
// canvas: pane rects scale proportionally — splits land where they will
// actually be once the window is entered and docked — and every line is
// truncated to its scaled cell so panes never bleed into a neighbor. A
// plain crop (the old behavior) put a 100/99 split's border at col 100 of
// a 159-col canvas and clipped the right pane to a sliver. Identity when
// the frame already fits (the docked window's own billboard).
func scaleFrame(frame []framePane, avail int) []framePane {
	fw := 0
	for _, p := range frame {
		if p.Left+p.Width > fw {
			fw = p.Left + p.Width
		}
	}
	if fw <= avail || fw <= 0 {
		return frame
	}
	s := float64(avail) / float64(fw)
	out := make([]framePane, len(frame))
	for i, p := range frame {
		// A pane with a left neighbor owns the column AFTER its scaled
		// border; its neighbor ends AT the border. Scaling the border
		// position (Left-1) keeps content and border from ever sharing a
		// column — plain edge rounding collapses the gap and the border
		// glyph eats the pane's first character.
		x0 := 0
		if p.Left > 0 {
			x0 = int(float64(p.Left-1)*s+0.5) + 1
		}
		x1 := int(float64(p.Left+p.Width)*s + 0.5)
		w := x1 - x0
		if w < 1 {
			w = 1
		}
		lines := make([]string, len(p.Lines))
		for j, ln := range p.Lines {
			lines[j], _ = cleanLine(ln, w)
		}
		out[i] = framePane{ID: p.ID, Left: x0, Top: p.Top, Width: w, Height: p.Height, Active: p.Active, Lines: lines}
	}
	return out
}

// cleanLine renders a captured line paint-safe: truncated at max display
// columns (max < 0 means no limit), CSI sequences passed through unconsumed
// (they take no columns), OSC sequences STRIPPED — capture emits them for
// hyperlinks (grok's TUI), their payload is zero-width so counting it skews
// the pad math, and a truncation cutting one mid-way leaves a dangling OSC
// that swallows everything painted after it. Returns the line and its
// display width; east-asian wide runes count 2 — close enough for previews
// without a width library. Trailing SGR state is deliberate: paintFrame
// pads in it (BCE bars), then resets.
func cleanLine(s string, max int) (string, int) {
	w := 0
	esc := 0 // 0 plain, 1 ESC, 2 CSI, 3 OSC, 4 ESC-in-OSC (ST?)
	var b strings.Builder
	for _, r := range s {
		switch esc {
		case 1:
			switch r {
			case '[':
				b.WriteRune(0x1b)
				b.WriteRune(r)
				esc = 2
			case ']':
				esc = 3
			default: // two-byte ESC sequence
				b.WriteRune(0x1b)
				b.WriteRune(r)
				esc = 0
			}
			continue
		case 2:
			b.WriteRune(r)
			if r >= 0x40 && r <= 0x7e {
				esc = 0
			}
			continue
		case 3:
			if r == 0x07 {
				esc = 0
			} else if r == 0x1b {
				esc = 4
			}
			continue
		case 4:
			if r == '\\' || r == 0x07 {
				esc = 0
			} else if r != 0x1b {
				esc = 3
			}
			continue
		}
		if r == 0x1b {
			esc = 1
			continue
		}
		rw := runeWidth(r)
		if max >= 0 && w+rw > max {
			break
		}
		w += rw
		b.WriteRune(r)
	}
	return b.String(), w
}

// runeWidth: east-asian wide runes count 2 — close enough for previews
// without pulling in a width library.
func runeWidth(r rune) int {
	if r >= 0x1100 && (r <= 0x115f ||
		(r >= 0x2e80 && r <= 0xa4cf) || (r >= 0xac00 && r <= 0xd7a3) ||
		(r >= 0xf900 && r <= 0xfaff) || (r >= 0xfe30 && r <= 0xfe4f) ||
		(r >= 0xff00 && r <= 0xff60) || (r >= 0xffe0 && r <= 0xffe6) ||
		(r >= 0x1f300 && r <= 0x1faff) || (r >= 0x20000 && r <= 0x3fffd)) {
		return 2
	}
	return 1
}


func framesEqual(a, b []framePane) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Left != b[i].Left || a[i].Top != b[i].Top ||
			a[i].Width != b[i].Width || a[i].Height != b[i].Height ||
			a[i].Active != b[i].Active ||
			len(a[i].Lines) != len(b[i].Lines) {
			return false
		}
		for j := range a[i].Lines {
			if a[i].Lines[j] != b[i].Lines[j] {
				return false
			}
		}
	}
	return true
}

func surfaceSize() (int, int) {
	cols, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || cols <= 0 {
		cols, height = 120, 40
	}
	return cols, height
}

// narrowMode: the surface is the docked 40-col sidebar, not the full-screen
// browser. The list takes the whole width (the tmux pane border is the
// separator) and the preview region simply doesn't exist.
func narrowMode() bool {
	cols, _ := surfaceSize()
	return cols <= listWidth+2
}

// listTop is the scroll offset paintList uses; click mapping shares it so
// a clicked y always lands on the row that was painted there.
func listTop(n, sel, height int) int {
	top := 0
	if n > height && sel > height/2 {
		top = sel - height/2
		if top > n-height {
			top = n - height
		}
	}
	return top
}

// benchListPrev remembers the last painted list content (bench mode only):
// a "list flicker" is by definition rows whose cells actually changed —
// tmux ships nothing for identical repaints — so diffing consecutive
// paints names exactly what was written.
var benchListPrev []string

// paintList redraws the list column and border only. Fixed width, padded
// with spaces — no clears, so unchanged cells cost nothing downstream.
// Wrapped in synchronized output (DECSET 2026) so tmux applies it atomically.
func paintList(rows []row, sel int) {
	start := time.Now()
	cols, height := surfaceSize()
	lw, border := listWidth, true
	if cols <= listWidth+2 {
		lw, border = cols, false
	}
	top := listTop(len(rows), sel, height)
	var cur []string
	if benchLog != nil {
		cur = make([]string, 0, height)
	}
	var b strings.Builder
	b.WriteString("\033[?2026h\033[0m")
	for y := 0; y < height; y++ {
		i := top + y
		fmt.Fprintf(&b, "\033[%d;1H", y+1)
		if i < len(rows) {
			label := []rune(rows[i].label)
			if len(label) > lw {
				label = label[:lw]
			}
			pad := strings.Repeat(" ", lw-len(label))
			switch {
			case i == sel:
				b.WriteString("\033[7m" + string(label) + pad + "\033[27m")
			case rows[i].session:
				b.WriteString("\033[1m" + string(label) + pad + "\033[22m")
			default:
				b.WriteString("\033[2m" + string(label) + pad + "\033[22m")
			}
			if benchLog != nil {
				cls := byte(' ')
				if i == sel {
					cls = '>'
				} else if rows[i].session {
					cls = 'S'
				}
				cur = append(cur, string(cls)+string(label))
			}
		} else {
			b.WriteString(strings.Repeat(" ", lw))
			if benchLog != nil {
				cur = append(cur, "")
			}
		}
		if border {
			b.WriteString("\033[2m│\033[22m")
		}
	}
	b.WriteString("\033[?2026l")
	os.Stdout.WriteString(b.String())
	if benchLog != nil {
		logged := 0
		for y := 0; y < len(cur) || y < len(benchListPrev); y++ {
			o, n := "", ""
			if y < len(benchListPrev) {
				o = benchListPrev[y]
			}
			if y < len(cur) {
				n = cur[y]
			}
			if o != n {
				if logged < 8 {
					benchf("list row %d: %q -> %q", y, o, n)
				}
				logged++
			}
		}
		if logged > 8 {
			benchf("list rows changed: %d total", logged)
		}
		benchListPrev = cur
	}
	benchf("paint_list dur_us=%d bytes=%d", time.Since(start).Microseconds(), b.Len())
}

// sameGeometry reports whether two frames tile identically with the same
// active pane — the precondition for line-level diff painting (an active
// change recolors borders, so it takes the full path).
func sameGeometry(a, b []framePane) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Left != b[i].Left || a[i].Top != b[i].Top ||
			a[i].Width != b[i].Width || a[i].Height != b[i].Height ||
			a[i].Active != b[i].Active {
			return false
		}
	}
	return true
}

// activeBorderStyle matches the real tmux pane-active-border color
// (catppuccin lavender). Theming milestone can query the live style later.
const activeBorderStyle = "\033[38;2;183;189;251m"

// paintBorders draws the gaps between panes as tmux-style borders: dim
// │ ─ ┼, with the active pane's surrounding border in the active color.
func paintBorders(b *strings.Builder, frame []framePane, cols, height, offX int) {
	W, H := 0, 0
	for _, p := range frame {
		if p.Left+p.Width > W {
			W = p.Left + p.Width
		}
		if p.Top+p.Height > H {
			H = p.Top + p.Height
		}
	}
	if W <= 0 || H <= 0 || len(frame) < 2 {
		return // single pane: no internal borders
	}
	const (
		vert  = 1
		horiz = 2
	)
	grid := make([]byte, W*H)
	mark := func(x, y int, kind byte) {
		if x >= 0 && x < W && y >= 0 && y < H {
			grid[y*W+x] |= kind
		}
	}
	for _, p := range frame {
		if x := p.Left + p.Width; x < W {
			for y := p.Top; y < p.Top+p.Height; y++ {
				mark(x, y, vert)
			}
			mark(x, p.Top+p.Height, vert|horiz)
		}
		if y := p.Top + p.Height; y < H {
			for x := p.Left; x < p.Left+p.Width; x++ {
				mark(x, y, horiz)
			}
		}
	}
	// Border cells adjacent to the active pane render green, like tmux's
	// pane-active-border-style.
	activeAt := func(x, y int) bool {
		for _, p := range frame {
			if !p.Active {
				continue
			}
			hSpan := x >= p.Left-1 && x <= p.Left+p.Width
			vSpan := y >= p.Top-1 && y <= p.Top+p.Height
			onV := (x == p.Left-1 || x == p.Left+p.Width) && vSpan
			onH := (y == p.Top-1 || y == p.Top+p.Height) && hSpan
			return onV || onH
		}
		return false
	}
	for y := 0; y < H && y < height; y++ {
		for x := 0; x < W && offX+x < cols; x++ {
			kind := grid[y*W+x]
			if kind == 0 {
				continue
			}
			ch := "│"
			switch kind {
			case horiz:
				ch = "─"
			case vert | horiz:
				ch = "┼"
			}
			style := "\033[2m"
			if activeAt(x, y) {
				style = activeBorderStyle
			}
			fmt.Fprintf(b, "\033[%d;%dH%s%s\033[0m", y+1, offX+x+1, style, ch)
		}
	}
}

// paintFrame redraws the preview region. With prev (same window, same
// geometry) only changed lines are erased-and-rewritten — a streaming pane
// repaints a few lines per frame instead of blanking the region, which is
// what made busy claude panes flicker. Without prev (new window, resize)
// the full region is prefilled so stale content from differently-shaped
// windows cannot linger. No screen clear either way — tmux diffs cells
// server-side and ships only real changes to the terminal.
func paintFrame(frame, prev []framePane) {
	start := time.Now()
	cols, height := surfaceSize()
	const offX = listWidth + 1 // frame region starts right of the border
	avail := cols - offX
	if avail <= 0 {
		return
	}
	var b strings.Builder
	b.WriteString("\033[?2026h\033[0m")
	changed := 0
	if prev == nil {
		blank := strings.Repeat(" ", avail)
		for y := 1; y <= height; y++ {
			fmt.Fprintf(&b, "\033[%d;%dH%s", y, offX+1, blank)
		}
	}
	for pi, p := range frame {
		if offX+p.Left >= cols || p.Top >= height {
			continue
		}
		width := p.Width
		if offX+p.Left+width > cols {
			width = cols - offX - p.Left
		}
		blank := strings.Repeat(" ", width)
		lines := len(p.Lines)
		if prev != nil && len(prev[pi].Lines) > lines {
			lines = len(prev[pi].Lines) // erase rows the new frame lost
		}
		for i := 0; i < lines; i++ {
			if i >= p.Height || p.Top+i >= height {
				break
			}
			ln := ""
			if i < len(p.Lines) {
				ln = p.Lines[i]
			}
			if prev != nil && i < len(prev[pi].Lines) && prev[pi].Lines[i] == ln {
				continue
			}
			changed++
			// One write: content, then spaces out to the pane's cell edge
			// BEFORE the SGR reset. TUIs paint full-width bars via BCE
			// (set bg + \033[K) and capture-pane drops the fill entirely,
			// leaving the line ending with the bar's bg still open
			// (probe-verified) — padding in that live state reconstructs
			// the bar; lines ending in default state pad blank as before.
			// Reset per line so pane edges never bleed attributes.
			ln, dw := cleanLine(ln, width)
			pad := width - dw
			if pad < 0 {
				pad = 0
			}
			fmt.Fprintf(&b, "\033[%d;%dH%s%s\033[0m",
				p.Top+1+i, offX+p.Left+1, ln, blank[:pad])
		}
	}
	if prev == nil {
		// Borders last: scaled-frame rounding can collapse a gap onto a
		// pane's first column, and the border should win that cell.
		paintBorders(&b, frame, cols, height, offX)
	}
	b.WriteString("\033[?2026l")
	os.Stdout.WriteString(b.String())
	benchf("paint_frame dur_us=%d bytes=%d panes=%d diff=%v changed_lines=%d",
		time.Since(start).Microseconds(), b.Len(), len(frame), prev != nil, changed)
}
