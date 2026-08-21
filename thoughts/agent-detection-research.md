# agent state detection — research for milestone 3

sources: herdr v0.8.2 source dive (2026-08-21, HEAD 8fe09d5, Apache 2.0) +
field survey of claude-squad / ccmanager / unitmux / agent-deck /
tmux-agent-status / tmux-agent-indicator / crystal.

## the headline: herdr TRIED hooks as state authority and REVERTED

- ~v0.4.x: claude/codex/opencode hooks were authoritative for state.
- failure modes accumulated (their changelog): post-tool hooks reporting
  `working` after the turn ended; SubagentStop reviving idle panes (claude
  recap/away-summary emits it late); stale `working` after user cancellations;
  stale `blocked` after permission prompts were dismissed. root cause: hooks
  are **edge-triggered and incomplete** — nothing fires on interrupt, prompt
  dismissal, or /clear; late async events resurrect wrong states. screens are
  **level-triggered ground truth**.
- v0.6.7 (2026-06-03): claude, codex, copilot, droid, kimi, qoder demoted to
  **session identity only**; state comes from screen detection. their
  installer now actively *deletes* the old lifecycle hook entries from user
  settings.json. hooks kept authority only for agents with complete
  lifecycle APIs: pi, omp, opencode (SSE), kilo, kimi >= 0.14.0.
- the one hook they kept for claude: `SessionStart` → report session_id +
  transcript_path over their socket (enables `claude --resume`). the hook
  script explicitly exits on SubagentStop with a "never let it revive an
  idle pane" comment.

## herdr's detection engine (port target)

four states: `idle | working | blocked | unknown`. per-agent declarative
TOML manifests compiled to a rule engine:

- rule = `{id, state, priority, region, visible_idle/blocker/working,
  skip_state_update, contains[], regex[], line_regex[], all[], any[], not[]}`
- gates recursive (depth 8); `contains` = case-insensitive substring (ALL
  must hit); `regex` vs whole region; `line_regex` must match >= 1 line each.
- arbitration: highest priority wins, ties → first in file. no match →
  idle for a known agent. `skip_state_update` rules (transcript viewer,
  model picker) freeze the previous state.
- regions: `whole_recent` (one viewport-height tail of the buffer — herdr
  never scans deeper than one screen), `bottom_lines(N)`,
  `bottom_non_empty_lines(N)`, `top_non_empty_lines(N)`, `prompt_box_body`
  (between the 2nd-from-last and last `─` horizontal rules),
  `above_prompt_box`, `last_non_empty_above_prompt_box`,
  `after_last_horizontal_rule`, codex prompt/block-marker regions,
  `osc_title`, `osc_progress`.
- input is the rendered grid as plain text (per-cell, wide-char tails
  skipped, rows trim_end'd) — **no ANSI reaches the matcher**. tmux
  equivalent: `capture-pane -p -J` (no `-e`).
- OSC tracker retains latest OSC 0/2 title (empty payload CLEARS it —
  codex signals idle that way; tmux can't distinguish cleared) and latest
  OSC 9 payload (ConEmu progress `4;<state>;<pct>` — tmux swallows these;
  claude has screen fallbacks for everything since herdr 0.8.2, droppable).
- agent identity is process-based: foreground process walk, normalized
  through node/bun/sh wrappers + nix `.foo-wrapped` argv0. tmux analogue:
  `#{pane_current_command}` + light pgrep walk.
- debugging: `herdr agent explain` dumps matched rule + per-rule evidence
  as JSON. worth copying as `demuxd agent explain`.
- remote manifest updates from herdr.dev; local override dir wins over
  bundled. engine version gate `min_engine_version`.

### the three load-bearing timing mechanisms

1. **working→plain-idle hold**: only for working → idle *without* visible
   evidence. 3 confirmations at 100ms recheck (normal tick 300ms), hard cap
   700ms. a `visible_idle` detection (❯ prompt box actually on screen, ✳
   title) **bypasses the hold**. no debounce toward blocked/working — those
   publish instantly.
2. **startup grace**: agent process changed → state=unknown, retained
   OSC title/progress wiped (new agent must not inherit the old title),
   screen scan suppressed 3s. process exit cancels grace and publishes
   synthetic idle.
3. **skip-scan-when-unchanged**: monotonic content-sequence counter bumped
   on PTY output; scan skipped only when idle + no pending hold + counter
   unchanged. any non-idle state always rescans. stable visible blockers
   re-published every 800ms.

### "done" is UI state, not detection

per-pane `seen` bit. set unseen on a completion transition
((working|blocked) → idle) **when the pane isn't in the user's focused
view**; completions in the focused view are seen immediately. cleared when
the containing tab/window gains focus. label: idle+unseen renders "done".
attention sort: blocked 4 > done 3 > working 2 > idle 1 > unknown 0,
aggregated upward.

## claude.toml (herdr v2026.08.19.1) — verbatim

```toml
id = "claude"
version = "2026.08.19.1"
min_engine_version = 2
updated_at = "2026-08-19T00:00:00Z"
aliases = ["claude-code"]

[[rules]]
id = "osc_title_working"
state = "working"
priority = 1100
region = "osc_title"
visible_working = true
# Braille covers <= 2.1.227; half-circles are the 2.1.228 busy spinner.
regex = ['^[\x{2800}-\x{28FF}\x{25D0}-\x{25D3}] ']

[[rules]]
id = "live_turn_working"
state = "working"
priority = 970
region = "bottom_non_empty_lines(12)"
visible_working = true
any = [
  { line_regex = ['^\s*[⏸⏵].*esc to interrupt(?:\s|·|$)'] },
  { line_regex = ['^\s*[\x{002A}\x{00B7}\x{2722}\x{2736}\x{273B}\x{273D}]\s+\S.*…(?:\s+\(\d+[smh](?:\s|·)|\s*$)'] },
]

[[rules]]
id = "background_shell_working"
state = "working"
priority = 965
region = "bottom_non_empty_lines(5)"
visible_working = true
line_regex = ['^\s*[⏸⏵].*·\s+[1-9]\d*\s+shells?\s+(?:·|$)']

[[rules]]
id = "background_agents_working"
state = "working"
priority = 965
region = "last_non_empty_above_prompt_box"
visible_working = true
line_regex = ['^\s*[\x{002A}\x{00B7}\x{2722}\x{2736}\x{273B}\x{273D}]\s+Waiting for [1-9]\d* background agents? to finish\s*$']

[[rules]]
id = "btw_overlay_working"
state = "working"
priority = 975
region = "bottom_non_empty_lines(5)"
visible_working = true
line_regex = [
  '^\s*/btw(?:\s|$)',
  '(?i)esc to close\s*$',
]

[[rules]]
id = "transcript_viewer"
state = "unknown"
priority = 1000
region = "bottom_non_empty_lines(3)"
skip_state_update = true
contains = ["showing detailed transcript"]
any = [
  { contains = ["ctrl+o", "to toggle"] },
  { contains = ["ctrl+e", "show all"] },
  { contains = ["ctrl+e", "collapse"] },
  { contains = ["↑↓ scroll"] },
  { contains = ["? for shortcuts"] },
]

[[rules]]
id = "live_blocked_form"
state = "blocked"
priority = 980
region = "after_last_horizontal_rule"
visible_blocker = true
contains = ["esc to cancel"]
any = [
  { contains = ["enter to confirm"] },
  { contains = ["enter to select"], any = [
    { contains = ["tab/arrow keys to navigate"] },
    { contains = ["arrow keys to navigate"] },
    { contains = ["arrows to navigate"] },
    { contains = ["↑/↓ to navigate"] },
    { contains = ["↑↓ to navigate"] },
  ] },
]

[[rules]]
id = "dynamic_workflow_prompt"
state = "blocked"
priority = 980
region = "whole_recent"
visible_blocker = true
contains = ["run a dynamic workflow?", "esc to cancel"]

[[rules]]
id = "live_prompt_box"
state = "idle"
priority = 950
region = "prompt_box_body"
visible_idle = true
line_regex = ['^\s*❯']
not = [
  { contains = ["enter to select"] },
  { contains = ["esc to cancel"] },
  { contains = ["tab/arrow keys"] },
  { contains = ["arrow keys to navigate"] },
  { contains = ["↑/↓ to navigate"] },
]

[[rules]]
id = "model_picker_menu"
state = "unknown"
priority = 900
region = "whole_recent"
skip_state_update = true
contains = ["select model", "enter to set as default", "esc to cancel"]
not = [
  { contains = ["do you want to proceed?"] },
  { contains = ["enter to select"] },
]

[[rules]]
id = "bash_permission_prompt"
state = "blocked"
priority = 850
region = "whole_recent"
visible_blocker = true
contains = ["do you want to proceed?"]
any = [
  { contains = ["bash command"] },
  { contains = ["bash("] },
  { contains = ["contains expansion"] },
  { contains = ["tab to amend"] },
  { contains = ["ctrl+e to explain"] },
]
all = [
  { any = [{ line_regex = ['(?i)^\s*❯?\s*yes\b'] }, { line_regex = ['(?i)^\s*1\.\s*yes\b'] }, { line_regex = ['(?i)^\s*2\.\s*no\b'] }] },
]

[[rules]]
id = "generic_permission_prompt"
state = "blocked"
priority = 840
region = "after_last_horizontal_rule"
visible_blocker = true
contains = ["do you want to proceed?", "esc to cancel"]
all = [
  { any = [
    { line_regex = ['(?i)^\s*❯?\s*1\.\s*yes\b'] },
    { line_regex = ['(?i)^\s*2\.\s*yes\b'] },
    { line_regex = ['(?i)^\s*2\.\s*no\b'] },
    { line_regex = ['(?i)^\s*3\.\s*no\b'] },
  ] },
]

[[rules]]
id = "legacy_no_prompt_blocker"
state = "blocked"
priority = 300
region = "whole_recent"
any = [
  { contains = ["do you want to"], any = [{ contains = ["yes"] }, { contains = ["❯"] }] },
  { contains = ["would you like to"], any = [{ contains = ["yes"] }, { contains = ["❯"] }] },
  { contains = ["waiting for permission"] },
  { contains = ["do you want to allow this connection?"] },
  { contains = ["tab to amend"] },
  { contains = ["ctrl+e to explain"] },
  { contains = ["do you want to proceed?", "esc to cancel"] },
  { contains = ["review your answers"] },
  { contains = ["skip interview and plan immediately"] },
]
not = [
  { regex = ['(?m)^\s*❯\s*$'] },
]

[[rules]]
id = "osc_title_idle"
state = "idle"
priority = 250
region = "osc_title"
visible_idle = true
regex = ['^\x{2733} ']

[[rules]]
id = "osc_progress_idle"
state = "idle"
priority = 250
region = "osc_progress"
regex = ['^4;0']
```

herdr bundles 20 manifests (codex, gemini, opencode, cursor, grok, qwen,
amp, kimi, kiro, devin, cline, droid, copilot, ...). notable:
- codex: title "Action Required" → blocked; braille frame set → working;
  any other non-empty title → idle (catch-all prio 100); screen fallback
  `^[•◦]\s+Working \([^)]*esc to interrupt\)`; weak blockers `[y/n]`,
  `yes (y)`.
- gemini: blocked on `│ Apply this change` / `│ Allow execution` /
  `waiting for user confirmation`; working = `esc to cancel`. tiny.
- qwen: ✳ title prefix means **blocked** (opposite of claude's idle!).
- cline: default rule = any non-empty screen → working (prio -10).

## field survey (non-herdr)

| tool | mechanism | states | lesson |
|---|---|---|---|
| claude-squad | sha256 of `capture-pane -p -e -J` every 500ms; changed→Running, stable→Ready; hardcoded prompt strings only for auto-yes | running/ready | hash-diff flaps: animated dialog while *waiting* reads busy forever |
| ccmanager | own PTY → headless xterm → per-agent regex over last 30 lines; busy/waiting instant, idle needs **1500ms unchanged** | idle/busy/waiting | second-best pattern set; same edge-instant/idle-debounced shape as herdr |
| unitmux | `pane_title` regex alone for claude (braille=busy, ✳=idle) + capture for prompts | idle/busy/waiting | title trick works over plain tmux formats |
| agent-deck (Go+tmux) | hybrid polling + installed hooks; staleness logic | running/waiting/idle/error | "codex cannot converge without the notify hook" |
| tmux-agent-status | hooks only → status files | working/done | handles Stop's `background_tasks` array; blind otherwise |
| crystal | owns process, `-p --output-format stream-json` | — | only works if you spawn the agent yourself |

## first-party signals inventory

- **claude code hooks**: UserPromptSubmit/PreToolUse (working),
  PermissionRequest + Notification matchers `permission_prompt`/`idle_prompt`
  (blocked/waiting), Stop (idle — check `background_tasks` array),
  SessionStart/End, SubagentStart/Stop (DANGER: fires late, revives idle).
  every payload: session_id, transcript_path, cwd; hook inherits
  `$TMUX_PANE` = join key. no event on interrupt/cancel/dismiss — the herdr
  revert reason.
- **claude code title**: continuously rewritten; braille/half-circle
  spinner = working, `✳ ` = idle. lands in `#{pane_title}`;
  `%pane-title-changed` is pushed in control mode (own-session scope needs
  a probe).
- **claude code OSC 9;4** progress: emitted but not passthrough-wrapped —
  tmux swallows it (claude-code#24901). only via pipe-pane; skip.
- **codex**: `notify` in config.toml fires ONE event
  (`agent-turn-complete`). title carries "Action Required" + spinner.
- **gemini**: real hooks now (BeforeAgent/AfterAgent/Notification), but
  AfterAgent can't distinguish done from pausing (gemini-cli#14596).
- **opencode**: `opencode serve` SSE stream (`session.idle`,
  `session.status`) — complete lifecycle, no scraping needed.
- **aider**: `--notifications-command` = stop-hook equivalent.
- **tmux side**: `monitor-silence`/`monitor-activity` push alerts in
  control mode (server-side quiescence timer); `pane_current_command` for
  agent identity (tmux bookkeeping — immune to the macOS ps entitlement
  hole); `%pane-title-changed` for title edges.

## product constraint (ojas, 2026-08-21)

demux is a product — a tmux sidecar for anyone, not just this dotfiles
setup. so: **zero-config detection is the contract** (server-side signals
only: pane_current_command, pane_title, capture-pane; no agent-side setup).
hooks are strictly opt-in (`demuxd hooks install` owning its own entries,
herdr-style removal of stale ones) and only ever ANNOTATE (identity for
resume, blocked-reason notes) — never state authority, no behavior fork on
their presence. manifests ship as data: bundled TOMLs + user override dir
(~/.config/demux/agents/*.toml) wins, so UI-copy breakage is a user patch,
not a release.

## optimization levers within the tmux-sidecar model (2026-08-21)

capture-pane is the cost center; all headroom lives above it, no VT engine
needed. in value order:
1. **row-diff frames**: daemon keeps last grid per pane, sends only changed
   rows; TUI paints only those (pattern proven by benchListPrev list diffs).
   makes a many-pane live wall feasible.
2. **dirty-gating**: skip capture for clean panes — one batched list-panes
   reading `#{history_size}`+cursor as a change counter; or subscribe
   `%output` as a dirty bit (control mode has per-pane output flags +
   pause/resume flow control via refresh-client -A/-f — tmux-side
   coalescing; cross-session scope needs the probe below).
3. **pipelined captures**: N capture commands per RTT (runPipelined shape).
4. **frame hashing**: identical capture → send nothing (spinner-only panes).
5. **partial captures**: capture-pane -S/-E row ranges — detection wants
   the bottom ~12–24 rows only.
6. **adaptive cadence**: followed pane ~30–50ms, visible-unselected slower,
   detection 300ms, all yielding to commands.
note: %output-fed in-house VT is a dead end regardless of language — it
only sees bytes from subscribe-time onward, so the grid must be seeded
from capture-pane anyway; capture stays the source of truth.

## probes — ANSWERED (2026-08-21, built as detect.go slice 1)

1. tmux emits NO control-mode notification for pane title changes AT ALL —
   not even same-session (probe: select-pane -T and OSC 2 both silent;
   %unlinked-window-renamed fired in the same capture, validating the
   probe). Also: a cross-session `split-window -d` emits NOTHING (no
   pane-changed — active pane unchanged; layout events don't cross).
   Consequence: detection is a tick (the daemon's only poll), and its own
   list-panes doubles as the self-heal for silently appearing panes.
2. #{pane_title} holds a STATIC spinner char (no frame cycling — sampled
   ⠂ constant over 1.5s live) — no churn, prefix regex suffices. Live
   titles: `⠂ <task>` working, `✳ <task>` idle; nix wrapper argv0 is
   `.claude-wrapped` (normalize before matching).
3. cost is fine: detect tick = one list-panes (~1ms) at 300ms only while
   agent panes exist (2s discovery cadence otherwise, 100ms while a
   pending-idle hold confirms); screen captures only for non-idle claude
   panes or on activity change.

## flap lessons from live deployment (same day)

- NEVER re-assert a weak title verdict (✳ idle) on quiet ticks over a
  kept screen state — instant idle<->working flap. herdr's exact
  skip-scan rule: only IDLE panes with an unmoved activity stamp skip
  the rescan; working/blocked always rescan.
- Hold ALL working->idle transitions (we dropped herdr's visible-idle
  bypass): narrow panes truncate the footer "· 1 shell ·" chip in and
  out per redraw while the ❯ prompt box stays visible — alternating
  verdicts flap straight through any bypass. Fast recheck keeps real
  idles under ~300ms.
- The post-turn "N shells still running" status line is steadier working
  evidence than the truncatable footer chip.
