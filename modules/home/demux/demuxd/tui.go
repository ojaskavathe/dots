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

	"golang.org/x/term"
)

// The sidebar TUI: runs inside the sidebar pane, subscribes to the daemon,
// renders the session/window tree, and turns j/k into live previews.
// Rendering is fully ours — ANSI into the pane, no tmux UI. The daemon owns
// every tmux mutation; this process never spawns anything.

type store struct {
	sessions map[string]session
	windows  map[string]window
	panes    map[string]pane
}

type wireMsg struct {
	Type     string    `json:"type"`
	Sessions []session `json:"sessions"`
	Windows  []window  `json:"windows"`
	Panes    []pane    `json:"panes"`
	Ops      []wireOp  `json:"ops"`
	Window   string    `json:"window"`
	OK       *bool     `json:"ok"`
	Err      string    `json:"err"`
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

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err == nil {
		defer term.Restore(int(os.Stdin.Fd()), oldState)
	}
	fmt.Print("\033[?25l")       // hide cursor
	defer fmt.Print("\033[?25h") // best effort; pane usually dies with us

	msgs := make(chan wireMsg, 64)
	go func() {
		sc := bufio.NewScanner(conn)
		sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
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

	target := func() string {
		if sel >= 0 && sel < len(rows) {
			return rows[sel].window
		}
		return ""
	}
	repaint := func() {
		rows = st.rows()
		if sel >= len(rows) {
			sel = len(rows) - 1
		}
		if sel < 0 {
			sel = 0
		}
		paint(rows, sel)
	}
	// No debounce: a preview is one capture round trip painted into the
	// canvas — nothing in the user's windows moves, so scrubbing is cheap.
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
		paint(rows, sel)
		if w := target(); w != "" {
			send(cmdMsg{Cmd: "preview", Window: w})
		}
	}

	// The list process persists across browse sessions (the browse window is
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
				repaint()
			case "select":
				for i, r := range rows {
					if r.window == m.Window && !r.session {
						sel = i
						break
					}
				}
				repaint()
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
			repaint()
		}
	}
}

func paint(rows []row, sel int) {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		width, height = listWidth, 40
	}
	top := 0
	if len(rows) > height && sel > height/2 {
		top = sel - height/2
		if top > len(rows)-height {
			top = len(rows) - height
		}
	}
	var b strings.Builder
	b.WriteString("\033[H")
	for i := top; i < len(rows) && i-top < height; i++ {
		r := rows[i]
		label := r.label
		if len(label) > width {
			label = label[:width]
		}
		switch {
		case i == sel:
			b.WriteString("\033[7m" + label + "\033[27m")
		case r.session:
			b.WriteString("\033[1m" + label + "\033[22m")
		default:
			b.WriteString("\033[2m" + label + "\033[22m")
		}
		b.WriteString("\033[K\r\n")
	}
	b.WriteString("\033[0J")
	os.Stdout.WriteString(b.String())
}
