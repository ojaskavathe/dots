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

// The browse TUI: one full-window process rendering the session/window list
// (left, 40 cols) and the live preview (right) in the same pane. Frames are
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

type store struct {
	sessions map[string]session
	windows  map[string]window
	panes    map[string]pane
}

type wireMsg struct {
	Type     string      `json:"type"`
	Sessions []session   `json:"sessions"`
	Windows  []window    `json:"windows"`
	Panes    []pane      `json:"panes"`
	Ops      []wireOp    `json:"ops"`
	Window   string      `json:"window"`
	Frame    []framePane `json:"frame"`
	OK       *bool       `json:"ok"`
	Err      string      `json:"err"`
}

type wireOp struct {
	Op    string          `json:"op"`
	Kind  string          `json:"kind"`
	ID    string          `json:"id"`
	Value json.RawMessage `json:"value"`
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
		if s.Name == demuxSession {
			continue // never list the browse surface itself
		}
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
	}
	frames := map[string]cached{}
	gen := 0
	paintedWin, paintedGen := "", -1

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
	// rows, not 45k cells).
	paintFrameFor := func(win string) {
		c, ok := frames[win]
		if !ok || (win == paintedWin && c.gen == paintedGen) {
			return
		}
		paintFrame(c.panes)
		paintedWin, paintedGen = win, c.gen
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
	// Cached frame paints locally NOW; the daemon refreshes it (and streams
	// it live) right behind. Neighbors are prefetched so the next scrub step
	// is warm too.
	requestFrames := func() {
		cur := target()
		if cur != "" {
			send(cmdMsg{Cmd: "preview", Window: cur})
		}
		seen := map[string]bool{cur: true, "": true}
		for _, i := range []int{sel - 1, sel + 1} {
			if i >= 0 && i < len(rows) && !seen[rows[i].window] {
				seen[rows[i].window] = true
				send(cmdMsg{Cmd: "preview", Window: rows[i].window, Prefetch: true})
			}
		}
	}
	move := func(delta int) {
		if len(rows) == 0 {
			return
		}
		next := sel + delta
		if next < 0 {
			next = 0
		}
		if next >= len(rows) {
			next = len(rows) - 1
		}
		if next == sel {
			return
		}
		sel = next
		benchf("key sel=%d target=%s cached=%v", sel, target(), frames[target()].panes != nil)
		paintList(rows, sel)
		paintFrameFor(target())
		requestFrames()
	}

	// The process persists across browse sessions (the browse window is
	// never destroyed); commit/close just switch the client away.
	for {
		select {
		case m, ok := <-msgs:
			if !ok {
				return // daemon gone
			}
			switch m.Type {
			case "snapshot", "diff":
				st.apply(m)
				rows = st.rows()
				if sel >= len(rows) {
					sel = len(rows) - 1
				}
				paintList(rows, sel)
			case "select":
				for i, r := range rows {
					if r.window == m.Window && !r.session {
						sel = i
						break
					}
				}
				paintList(rows, sel)
				paintFrameFor(target())
				requestFrames()
			case "frame":
				benchf("frame win=%s current=%v", m.Window, m.Window == target())
				if prev, ok := frames[m.Window]; ok && framesEqual(prev.panes, m.Frame) {
					break // confirming frame, content unchanged: no repaint
				}
				gen++
				frames[m.Window] = cached{gen: gen, panes: m.Frame}
				if m.Window == target() {
					paintFrameFor(m.Window)
				}
			}
		case b, ok := <-keys:
			if !ok {
				return
			}
			switch {
			case esc == 1 && b == '[':
				esc = 2
			case esc == 2:
				esc = 0
				if b == 'A' {
					move(-1)
				} else if b == 'B' {
					move(1)
				}
			case b == 0x1b:
				esc = 1
			case b == 'j':
				move(1)
			case b == 'k':
				move(-1)
			case b == '\r':
				send(cmdMsg{Cmd: "commit", Window: target()})
			case b == 'q', b == 0x03: // q, ctrl-c
				send(cmdMsg{Cmd: "close"})
			default:
				esc = 0
			}
		case <-winch:
			paintAll()
		}
	}
}

func framesEqual(a, b []framePane) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Left != b[i].Left || a[i].Top != b[i].Top ||
			a[i].Width != b[i].Width || a[i].Height != b[i].Height ||
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

// paintList redraws the list column and border only. Fixed width, padded
// with spaces — no clears, so unchanged cells cost nothing downstream.
// Wrapped in synchronized output (DECSET 2026) so tmux applies it atomically.
func paintList(rows []row, sel int) {
	start := time.Now()
	_, height := surfaceSize()
	top := 0
	if len(rows) > height && sel > height/2 {
		top = sel - height/2
		if top > len(rows)-height {
			top = len(rows) - height
		}
	}
	var b strings.Builder
	b.WriteString("\033[?2026h\033[0m")
	for y := 0; y < height; y++ {
		i := top + y
		fmt.Fprintf(&b, "\033[%d;1H", y+1)
		if i < len(rows) {
			label := []rune(rows[i].label)
			if len(label) > listWidth {
				label = label[:listWidth]
			}
			pad := strings.Repeat(" ", listWidth-len(label))
			switch {
			case i == sel:
				b.WriteString("\033[7m" + string(label) + pad + "\033[27m")
			case rows[i].session:
				b.WriteString("\033[1m" + string(label) + pad + "\033[22m")
			default:
				b.WriteString("\033[2m" + string(label) + pad + "\033[22m")
			}
		} else {
			b.WriteString(strings.Repeat(" ", listWidth))
		}
		b.WriteString("\033[2m│\033[22m")
	}
	b.WriteString("\033[?2026l")
	os.Stdout.WriteString(b.String())
	benchf("paint_list dur_us=%d bytes=%d", time.Since(start).Microseconds(), b.Len())
}

// paintFrame redraws the preview region only: space-prefill (stale content
// from differently-shaped windows must not linger), then pane contents at
// their window coordinates. No screen clear — tmux diffs cells server-side
// and ships only real changes to the terminal.
func paintFrame(frame []framePane) {
	start := time.Now()
	cols, height := surfaceSize()
	const offX = listWidth + 1 // frame region starts right of the border
	avail := cols - offX
	if avail <= 0 {
		return
	}
	var b strings.Builder
	b.WriteString("\033[?2026h\033[0m")
	blank := strings.Repeat(" ", avail)
	for y := 1; y <= height; y++ {
		fmt.Fprintf(&b, "\033[%d;%dH%s", y, offX+1, blank)
	}
	for _, p := range frame {
		if offX+p.Left >= cols || p.Top >= height {
			continue
		}
		for i, ln := range p.Lines {
			if i >= p.Height || p.Top+i >= height {
				break
			}
			// 1-based addressing; SGR reset per line so pane edges don't
			// bleed attributes into each other.
			fmt.Fprintf(&b, "\033[%d;%dH%s\033[0m", p.Top+1+i, offX+p.Left+1, ln)
		}
	}
	b.WriteString("\033[?2026l")
	os.Stdout.WriteString(b.String())
	benchf("paint_frame dur_us=%d bytes=%d panes=%d", time.Since(start).Microseconds(), b.Len(), len(frame))
}
