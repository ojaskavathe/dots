package main

import (
	"bufio"
	"encoding/json"
	"net"
	"sync"
	"time"
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
						h.cmds <- cmdEnvelope{msg: cmdMsg{Type: "cmd", Cmd: "hello-" + m.Role}, sub: s, recv: time.Now()}
					case "cmd":
						h.cmds <- cmdEnvelope{msg: m, sub: s, recv: time.Now()}
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
