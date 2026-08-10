# demux architecture

herdr-class agent awareness (status, navigation, previews, notifications) built
natively on tmux. tmux stays the runtime and the UI host; demux adds the world
model and the surfaces.

## goals

- every tmux entity (session / window / pane) and every agent modeled in one
  place, updated by events only — zero poll loops against tmux
- sidebar TUI with navigation (jump, kill, spawn) and live previews
- agent lifecycle (working / blocked / idle / done) exact for hooked agents,
  best-effort for the rest
- ambient signal in the tmux status line + notifications on blocked
- fully custom rendering — no dependence on choose-tree / display-menu
- packaged and wired through nix (hm module in dots)

## non-goals

- replacing tmux persistence (resurrect/continuum already handle it)
- remote/web access (a later client could add it; the protocol allows it)
- cross-machine fleets

## the one big decision: control mode, not hooks

the spike used `set-hook` + `run-shell`, which forks a process per event. fine
for 10 hooks; wrong for a world model, and it can't see output at all.

tmux **control mode** (`tmux -C attach`) is a persistent text protocol over the
server socket: one connection that receives every server event as a notification
line — `%session-changed`, `%sessions-changed`, `%window-add`, `%window-close`,
`%window-renamed`, `%layout-change`, `%pane-mode-changed`, `%output` (live pane
output), `%exit`. this is how iTerm2's tmux integration maintains a full mirror
of the server; it is almost certainly what herdr itself does under the hood.

consequences:

- **zero process spawns** on the event path. one long-lived connection.
- **pane output is an event.** activity detection and previews come from
  `%output` notifications, not `capture-pane` polling.
- tmux 3.2+ `refresh-client -A %pane:on` / `-B` subscriptions let the daemon opt
  into exactly the panes/formats it wants pushed.
- the daemon can also _send_ commands down the same connection (list-panes on
  startup for the initial snapshot, switch-client on jump, etc.)

hooks remain only where control mode has no equivalent: none known. claude code
hooks stay, but they talk to the daemon socket, not to tmux.

## components

```
                 ┌────────────────────────────────────────────┐
                 │              demuxd (daemon)                │
 tmux server ◄──►│  control-mode client ──► reducer ──► state │
                 │                                    │       │
 agent hooks ───►│  unix socket /demux.sock ◄── pub/sub┘       │
 (demux emit)     └───────────▲────────────────▲───────────────┘
                             │                │
                   ┌─────────┴───┐   ┌────────┴─────────┐
                   │ sidebar TUI │   │ statusline bridge │
                   │ (per pane)  │   │ notifier          │
                   └─────────────┘   │ demux CLI          │
                                     └───────────────────┘
```

### demuxd — the daemon

one per tmux server socket. owns everything:

- **ingest**: control-mode notifications + agent events on the unix socket
- **reducer**: events → state mutations → diffs
- **pub/sub**: clients connect to `$XDG_RUNTIME_DIR/demux/<server>.sock`, receive
  a full snapshot then diffs (NDJSON)
- **timers**: the only time-based logic allowed — debounce `%output` bursts,
  classify "no output for Ns" as idle for unhooked agents. these are internal
  state-machine timers, not polls of tmux.
- **command execution**: jump/kill/spawn requests from clients go out through
  the control-mode connection

lifecycle: lazily started by the first client (`demux` CLI or the toggle keybind
runs `demux daemon --ensure`), dies when the tmux server dies (`%exit`),
restartable at any time — state is rebuilt from a `list-sessions` /
`list-windows` / `list-panes` snapshot on connect, so the daemon is disposable
by design.

### event sources

1. **tmux** via control mode — topology, layout, activity, output
2. **hooked agents** — claude code (`Stop`, `Notification`, `PermissionRequest`,
   `UserPromptSubmit`, …) and anything else that can run a command on lifecycle
   events. hook scripts call
   `demux emit --pane "$TMUX_PANE" --state blocked --note "wants to run rm"` →
   daemon socket. exact, instant, no heuristics.
3. **unhooked agents** — daemon-side heuristics over `%output` streams +
   `pane_current_command`: known-agent process names, spinner/prompt patterns,
   output-silence timers. best-effort tier, clearly marked in the model so UIs
   can render certainty differently.

### clients (all thin, all subscribers)

- **sidebar TUI** — the flagship surface. runs inside a tmux pane (same
  toggle/layout mechanics as the spike). subscribes, renders the tree + agent
  states + preview, owns its input: j/k navigation, enter=jump, x=kill,
  p=preview focus, mouse clicks. rendering is fully ours (lipgloss/ratatui) —
  herdr-look achievable pixel for pixel.
- **statusline bridge** — on diff:
  `tmux set -g @demux_status "…" ; refresh-client -S`. the status line stays a
  dumb `#{@demux_status}` format reference: zero-cost render, instant updates, no
  `#()`.
- **notifier** — on working→blocked: `tmux display-message`, bell, or
  terminal-notifier. policy lives here, not in the daemon.
- **demux CLI** — `demux ls`, `demux jump <target>`, `demux emit …`, scripting
  surface for everything else (and the hook entrypoint).

one binary, subcommands (`demux daemon`, `demux sidebar`, `demux emit`, …) — single
nix package, shared model/protocol code.

## data model

```
Server   { sessions: [Session], clients: [Client] }
Session  { id, name, attached, windows: [Window] }
Window   { id, index, name, active, layout, panes: [Pane] }
Pane     { id, active, cmd, path, title,
           agent?: Agent, activity: { lastOutput, rate } }
Agent    { kind,                    # claude-code | codex | …
           state,                   # working | blocked | idle | done
           certainty,               # hooked | heuristic
           since, note }            # note: "waiting on permission: …"
```

ids are tmux's own (`$0`, `@1`, `%2`) — never indexes, which shift.

## protocol (daemon ⇄ clients)

NDJSON over the unix socket. deliberately boring.

- `→ {"v":1,"type":"hello","client":"sidebar"}`
- `← {"type":"snapshot","state":{…}}`
- `← {"type":"diff","ops":[{"op":"set","path":"panes.%12.agent.state","value":"blocked"},…]}`
- `→ {"type":"cmd","cmd":"jump","target":"%12"}`
- `→ {"type":"emit","pane":"%12","agent":{"kind":"claude-code","state":"working"}}`
- `→ {"type":"preview","pane":"%12","follow":true}` /
  `← {"type":"preview-frame","pane":"%12","lines":[…]}`

versioned from day one; a future web client speaks the same protocol.

## previews

two tiers, both event-driven:

1. **on selection** — sidebar highlights a pane → sends `preview` → daemon
   replies with a frame (`capture-pane -e -p` once, on demand — an event: the
   user navigated).
2. **follow mode** — daemon subscribes to that pane's `%output`
   (`refresh-client -A`), debounces (~50ms), pushes incremental frames while
   selected. unsubscribes on deselect. live preview with zero standing cost for
   unselected panes.

frames carry SGR escapes; the TUI paints them into the preview region verbatim
(with clipping). no vt-emulation layer unless/until we want scrollback in
previews — capture-pane already returns a rendered screen.

## navigation

- sidebar sends `jump` → daemon issues `switch-client -t` for the requesting
  client (daemon knows which client to move via the sidebar's pane→client
  mapping)
- kill/spawn follow the same path (`kill-pane`, `split-window` for "new agent
  here", worktree-aware spawn later)
- sidebar panes: per-session, spawned by the same toggle (layout
  snapshot/restore mechanics carry over from the spike unchanged). selection
  state is local to each sidebar process; world state is shared through the
  daemon. a later "mission control" full-window client is just another
  subscriber with a bigger canvas.

## language

go. matches the existing pattern in dots (`tmux-equalize-nvim`, buildGoModule),
bubbletea/lipgloss for the TUI, stdlib for sockets. rust + ratatui is equally
capable — choose go purely for repo consistency and iteration speed.

## nix / deployment

- flake package `demux` (buildGoModule) — its own repo once it stabilizes; starts
  life under `modules/home/demux/`
- hm module wires: toggle keybind (`prefix b` →
  `demux sidebar --toggle "#{pane_id}"`), claude code hook entries pointing at
  `demux emit`, statusline format reference, launchd/systemd user service
  optional (lazy start is the default)
- no tmux hooks needed anymore → the spike's 10 `set-hook` lines disappear

## failure semantics

- daemon crash: clients show "disconnected", retry with backoff; restart
  rebuilds from snapshot. no persistent state to corrupt.
- tmux server gone: `%exit` → daemon exits; next toggle restarts everything.
- hook fires with no daemon: `demux emit` starts it (`--ensure`) or drops the
  event after a short timeout — hooks must never block the agent.

## milestones

1. **demuxd core** — control-mode ingest, reducer, snapshot+diff protocol,
   `demux ls`. proves the event path end to end.
2. **sidebar TUI** — tree render, j/k/enter/x, mouse, toggle mechanics ported
   from spike. replaces sidebar.sh.
3. **agents** — claude code hooks → `demux emit`, agent states in sidebar,
   statusline bridge, blocked notifications.
4. **previews** — selection frames, then follow mode.
5. **heuristic tier** — unhooked agent detection from `%output` patterns.
6. **polish** — spawn-with-worktree, mission-control window, theming.

each milestone is independently useful; stop anywhere and the previous
milestones keep working.

## open questions

- exact `%output` semantics across detached sessions on tmux 3.5 (verify
  `refresh-client -A` covers panes in sessions no client is attached to;
  fallback: a throwaway control-mode attach per session — still events)
- diff granularity: per-field ops vs whole-entity replacement (start with
  whole-entity, measure)
- whether the sidebar should also host the "blocked queue" herdr shows, or that
  becomes a separate popup client
