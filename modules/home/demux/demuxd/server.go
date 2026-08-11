package main

import (
	"bufio"
	"encoding/json"
	"net"
	"sync"
)

// hub owns the published world and fans messages out to subscribers.
// Subscribers that stall get dropped (buffered channel, non-blocking send) —
// the daemon must never block on a slow client.
type hub struct {
	mu    sync.Mutex
	world world
	subs  map[*subscriber]struct{}
	cmds  chan cmdEnvelope
}

type subscriber struct {
	conn net.Conn
	ch   chan []byte
	role string // "", "list", "canvas" — set by a hello message
}

// cmdMsg is a client -> daemon request; replyMsg answers it on the same conn
// (interleaved with the world stream, so replies are type-tagged). A
// {"type":"hello","role":"..."} line tags the connection instead.
type cmdMsg struct {
	Type     string `json:"type"`
	Cmd      string `json:"cmd"`
	Client   string `json:"client,omitempty"`
	Window   string `json:"window,omitempty"`
	Role     string `json:"role,omitempty"`
	Dir      string `json:"dir,omitempty"` // nav: "next" | "prev"
	Prefetch bool   `json:"prefetch,omitempty"`
}

// selectMsg tells the list TUI to move its selection (daemon -> list).
type selectMsg struct {
	Type   string `json:"type"`
	Window string `json:"window"`
}

// frameMsg carries a captured window for the preview region (daemon ->
// list TUI). Pane lines are raw capture-pane -e output (SGR included).
type frameMsg struct {
	Type   string      `json:"type"`
	Window string      `json:"window"`
	Panes  []framePane `json:"frame"`
}

type framePane struct {
	Left   int      `json:"left"`
	Top    int      `json:"top"`
	Width  int      `json:"width"`
	Height int      `json:"height"`
	Active bool     `json:"active,omitempty"`
	Lines  []string `json:"lines"`
}

type replyMsg struct {
	Type string `json:"type"`
	OK   bool   `json:"ok"`
	Err  string `json:"err,omitempty"`
}

type cmdEnvelope struct {
	msg cmdMsg
	sub *subscriber
}

func newHub() *hub {
	return &hub{subs: map[*subscriber]struct{}{}, cmds: make(chan cmdEnvelope, 16)}
}

func (h *hub) getWorld() world {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.world
}

// send queues a message to one subscriber, dropping it if stalled.
func (h *hub) send(s *subscriber, payload []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sendLocked(s, payload)
}

func (h *hub) sendLocked(s *subscriber, payload []byte) {
	if _, ok := h.subs[s]; !ok {
		return
	}
	select {
	case s.ch <- payload:
	default:
		delete(h.subs, s)
		close(s.ch)
	}
}

// sendRole queues a message to every subscriber with the given role and
// reports how many received it — zero receivers means the message was lost,
// which the caller should surface in its logs.
func (h *hub) sendRole(role string, payload []byte) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for s := range h.subs {
		if s.role == role {
			h.sendLocked(s, payload)
			n++
		}
	}
	return n
}

func (h *hub) setRole(s *subscriber, role string) {
	h.mu.Lock()
	s.role = role
	h.mu.Unlock()
}

type snapshotMsg struct {
	V    int    `json:"v"`
	Type string `json:"type"` // snapshot
	Tmux string `json:"tmux"` // tmux server socket path
	world
}

type diffMsg struct {
	Type string `json:"type"` // diff
	Ops  []op   `json:"ops"`
}

// setWorld replaces the world and broadcasts: a diff when ops are known, or a
// fresh snapshot after a reconnect (resync=true) since diffs across a gap lie.
func (h *hub) setWorld(w world, ops []op, resync bool, tmuxSock string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.world = w
	var payload []byte
	if resync {
		payload = marshalLine(snapshotMsg{V: 1, Type: "snapshot", Tmux: tmuxSock, world: w})
	} else {
		if len(ops) == 0 {
			return
		}
		payload = marshalLine(diffMsg{Type: "diff", Ops: ops})
	}
	for s := range h.subs {
		select {
		case s.ch <- payload:
		default:
			delete(h.subs, s)
			close(s.ch)
		}
	}
}

// add registers a subscriber and queues its initial snapshot atomically with
// respect to setWorld, so no diff can slip between snapshot and subscription.
func (h *hub) add(conn net.Conn, tmuxSock string) *subscriber {
	s := &subscriber{conn: conn, ch: make(chan []byte, 256)}
	h.mu.Lock()
	s.ch <- marshalLine(snapshotMsg{V: 1, Type: "snapshot", Tmux: tmuxSock, world: h.world})
	h.subs[s] = struct{}{}
	h.mu.Unlock()
	return s
}

func (h *hub) remove(s *subscriber) {
	h.mu.Lock()
	if _, ok := h.subs[s]; ok {
		delete(h.subs, s)
		close(s.ch)
	}
	h.mu.Unlock()
}

func (h *hub) closeAll() {
	h.mu.Lock()
	for s := range h.subs {
		delete(h.subs, s)
		close(s.ch)
	}
	h.mu.Unlock()
}

func marshalLine(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{"type":"error","error":"marshal"}` + "\n")
	}
	return append(b, '\n')
}

// serve accepts subscribers; incoming NDJSON cmd lines go to the daemon's
// event loop, which serializes them with re-lists and tmux commands.
func serve(ln net.Listener, h *hub, tmuxSock string) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go func(conn net.Conn) {
			s := h.add(conn, tmuxSock)
			go func() {
				sc := bufio.NewScanner(conn)
				sc.Buffer(make([]byte, 4096), 1024*1024)
				for sc.Scan() {
					var m cmdMsg
					if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
						continue
					}
					switch m.Type {
					case "hello":
						h.setRole(s, m.Role)
						// Let the daemon replay state (frame, selection)
						// to a client that connected after it was sent.
						h.cmds <- cmdEnvelope{msg: cmdMsg{Type: "cmd", Cmd: "hello-" + m.Role}, sub: s}
					case "cmd":
						h.cmds <- cmdEnvelope{msg: m, sub: s}
					}
				}
				h.remove(s)
				conn.Close()
			}()
			for msg := range s.ch {
				if _, err := conn.Write(msg); err != nil {
					h.remove(s)
					break
				}
			}
			conn.Close()
		}(conn)
	}
}
