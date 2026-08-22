package main

import (
	"encoding/json"
	"time"
)

// The demux wire protocol: newline-delimited JSON over the unix socket, both
// directions. This file is the complete vocabulary — anything a client can
// say or hear is defined here.
//
// A connection begins with the daemon pushing one snapshot; diffs follow as
// the world changes. Clients may send:
//   {"type":"hello","role":"list"}      tag the connection (role-addressed pushes)
//   {"type":"cmd","cmd":...,...}        commands; each gets one reply line
// Everything the daemon pushes is type-tagged ("snapshot", "diff", "select",
// "frame", "reply"), so replies interleave safely with the world stream.
//
// Compatibility: snapshotMsg carries a protocol version (v). Additive fields
// are the upgrade path — clients must ignore keys they don't know.

// cmdMsg is a client -> daemon request.
type cmdMsg struct {
	Type     string `json:"type"`
	Cmd      string `json:"cmd"`
	Client   string `json:"client,omitempty"`
	Window   string `json:"window,omitempty"`
	Role     string `json:"role,omitempty"`
	Dir      string `json:"dir,omitempty"`   // nav: "next" | "prev"
	Width    int    `json:"width,omitempty"` // winch: the TUI's new cols
	Pane     string `json:"pane,omitempty"`  // commit: focus this pane (billboard click)
	Prefetch bool   `json:"prefetch,omitempty"`
}

// replyMsg answers one cmdMsg on the same connection.
type replyMsg struct {
	Type string `json:"type"`
	OK   bool   `json:"ok"`
	Err  string `json:"err,omitempty"`
}

// snapshotMsg is the full world, pushed on connect and after control-mode
// reconnects (diffs across a gap could lie).
type snapshotMsg struct {
	V    int    `json:"v"`
	Type string `json:"type"` // snapshot
	Tmux string `json:"tmux"` // tmux server socket path
	world
}

// diffMsg carries whole-entity put/del ops since the previous world.
type diffMsg struct {
	Type string `json:"type"` // diff
	Ops  []op   `json:"ops"`
}

// selectMsg tells the list TUI to move its selection (daemon -> list).
type selectMsg struct {
	Type   string `json:"type"`
	Window string `json:"window"`
}

// frameMsg carries a captured window for the preview region (daemon ->
// list TUI). Pane lines are raw capture-pane -e output (SGR included).
// Full frames carry every pane's whole grid plus a generation; stream
// ticks of an unchanged-shape target ship a delta instead: only changed
// rows per pane, valid solely against the client's cache at Base. Safe
// because hub sends are in-order-or-disconnect — a client that missed a
// message is gone, and a fresh one starts from a full hello replay.
type frameMsg struct {
	Type   string      `json:"type"`
	Window string      `json:"window"`
	Panes  []framePane `json:"frame"`
	Gen    int         `json:"gen,omitempty"`   // generation of this frame
	Delta  bool        `json:"delta,omitempty"` // Panes carry only changed rows
	Base   int         `json:"base,omitempty"`  // delta applies to the cache at this gen
}

type framePane struct {
	ID     string   `json:"id,omitempty"` // real pane id — billboard clicks target it
	Left   int      `json:"left"`
	Top    int      `json:"top"`
	Width  int      `json:"width"`
	Height int      `json:"height"`
	Active bool     `json:"active,omitempty"`
	Lines  []string `json:"lines"`
	Rows   []int    `json:"rows,omitempty"` // delta only: row indices for Lines
	// The real pane's cursor, billboarded like everything else (a canvas
	// with no cursor is the quickest visual tell). Only the window's active
	// pane with a visible cursor sets Cursor.
	Cursor  bool `json:"cursor,omitempty"`
	CursorX int  `json:"curx,omitempty"`
	CursorY int  `json:"cury,omitempty"`
}

// wireMsg is the client-side decode target: one struct covering every
// daemon push, discriminated by Type.
type wireMsg struct {
	Type     string      `json:"type"`
	Sessions []session   `json:"sessions"`
	Windows  []window    `json:"windows"`
	Panes    []pane      `json:"panes"`
	Ops      []wireOp    `json:"ops"`
	Window   string      `json:"window"`
	Frame    []framePane `json:"frame"`
	Gen      int         `json:"gen"`
	Delta    bool        `json:"delta"`
	Base     int         `json:"base"`
	OK       *bool       `json:"ok"`
	Err      string      `json:"err"`
}

// wireOp defers entity decoding until Kind is known.
type wireOp struct {
	Op    string          `json:"op"`
	Kind  string          `json:"kind"`
	ID    string          `json:"id"`
	Value json.RawMessage `json:"value"`
}

// cmdEnvelope wraps a received cmdMsg with its sender and enqueue time
// (runCmd logs how long a command sat queued).
type cmdEnvelope struct {
	msg  cmdMsg
	sub  *subscriber
	recv time.Time
}

func marshalLine(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{"type":"error","error":"marshal"}` + "\n")
	}
	return append(b, '\n')
}
