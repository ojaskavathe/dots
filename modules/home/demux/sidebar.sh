# demux-sidebar (spike): event-driven, navigable session sidebar for tmux
#
#   toggle [pane]   spawn/kill the sidebar pane in [pane]'s window; the
#                   keybind passes "#{pane_id}" (run-shell's env has no
#                   reliable TMUX_PANE)
#   run             the sidebar process itself (spawned by toggle)
#   refresh         signal all running sidebars to repaint (wired to hooks)
#
# No polling anywhere: hooks fire `refresh`, which SIGUSR1s the sidebar
# processes; each rebuilds from one tmux call and goes back to blocking
# on keyboard input.
#
# Keys: j/k or arrows move, Enter jumps (session or window), g/G first/last,
# q closes. Jumping assumes the common single-client case; a multi-client
# setup would need explicit -c targeting (daemon's job, later).

set -euo pipefail

SELF="${BASH_SOURCE[0]}"
STATE_DIR="${XDG_RUNTIME_DIR:-$HOME/.cache}/demux"
WIDTH="${DEMUX_WIDTH:-32}"
mkdir -p "$STATE_DIR"

# 256-color, roughly catppuccin mocha
C_TITLE=$'\033[1;38;5;183m'
C_SES=$'\033[1;38;5;189m'
C_SES_ME=$'\033[1;38;5;147m'
C_ON=$'\033[38;5;114m'
C_OFF=$'\033[38;5;240m'
C_DIM=$'\033[38;5;245m'
R=$'\033[0m'
K=$'\033[K'
REV=$'\033[7m'
NOREV=$'\033[27m'

# item model, rebuilt by build(): parallel arrays, one entry per line
ITEM_TEXT=()  # colored line
ITEM_PLAIN=() # uncolored line (used when selected, so reverse video survives)
ITEM_KIND=()  # session | window | sep
ITEM_ARG=()   # session name, or "window_id|session_name"
SEL=0
MYWID=""

build() {
  local width mysession prev
  local sname attached wcount widx wid wactive wpath scolor dot dotchar mark wcolor smax wmax
  # MYWID re-read here because follow mode moves this pane between windows
  IFS='|' read -r width mysession MYWID < <(tmux display-message -p -t "$TMUX_PANE" '#{pane_width}|#{session_name}|#{window_id}')
  smax=$((width - 8))
  wmax=$((width - 7))
  ITEM_TEXT=()
  ITEM_PLAIN=()
  ITEM_KIND=()
  ITEM_ARG=()
  prev=""
  while IFS='|' read -r sname attached wcount widx wid wactive wpath; do
    if [[ $sname != "$prev" ]]; then
      prev=$sname
      scolor=$C_SES
      [[ $sname == "$mysession" ]] && scolor=$C_SES_ME
      dotchar="○"
      [[ $attached -gt 0 ]] && dotchar="●"
      dot="${C_OFF}${dotchar}"
      [[ $attached -gt 0 ]] && dot="${C_ON}${dotchar}"
      ITEM_TEXT+=("")
      ITEM_PLAIN+=("")
      ITEM_KIND+=("sep")
      ITEM_ARG+=("")
      ITEM_TEXT+=(" ${dot}${R} ${scolor}${sname:0:smax}${R} ${C_DIM}${wcount}${R}")
      ITEM_PLAIN+=(" ${dotchar} ${sname:0:smax} ${wcount}")
      ITEM_KIND+=("session")
      ITEM_ARG+=("$sname")
    fi
    mark=" "
    wcolor=$C_DIM
    if [[ $wactive == 1 ]]; then
      mark="▸"
      wcolor=$R
    fi
    ITEM_TEXT+=("   ${C_DIM}${widx}${R} ${wcolor}${mark}${wpath:0:wmax}${R}")
    ITEM_PLAIN+=("   ${widx} ${mark}${wpath:0:wmax}")
    ITEM_KIND+=("window")
    ITEM_ARG+=("${wid}|${sname}")
  done < <(tmux list-windows -a -F '#{session_name}|#{session_attached}|#{session_windows}|#{window_index}|#{window_id}|#{window_active}|#{b:pane_current_path}')
  clamp_sel
}

clamp_sel() {
  local n=${#ITEM_KIND[@]}
  ((n == 0)) && {
    SEL=0
    return
  }
  ((SEL >= n)) && SEL=$((n - 1))
  ((SEL < 0)) && SEL=0
  # never rest on a separator
  while ((SEL < n - 1)) && [[ ${ITEM_KIND[$SEL]} == "sep" ]]; do SEL=$((SEL + 1)); done
  while ((SEL > 0)) && [[ ${ITEM_KIND[$SEL]} == "sep" ]]; do SEL=$((SEL - 1)); done
}

paint() {
  local buf i now
  buf="${C_TITLE} demux${R}${K}"$'\n'
  for i in "${!ITEM_KIND[@]}"; do
    if ((i == SEL)) && [[ ${ITEM_KIND[$i]} != "sep" ]]; then
      buf+="${REV}${ITEM_PLAIN[$i]}${NOREV}${K}"$'\n'
    else
      buf+="${ITEM_TEXT[$i]}${K}"$'\n'
    fi
  done
  if ((${#ITEM_KIND[@]} == 0)); then
    buf+="${K}"$'\n'" ${C_DIM}no sessions${R}${K}"$'\n'
  fi
  printf -v now '%(%H:%M:%S)T' -1
  buf+="${K}"$'\n'" ${C_OFF}j/k enter q · ${now}${R}${K}"
  printf '\033[H%s\033[0J' "$buf"
}

render() {
  build
  paint
}

sel_move() {
  local dir=$1 i n
  n=${#ITEM_KIND[@]}
  ((n == 0)) && return
  i=$SEL
  while :; do
    i=$((i + dir))
    ((i < 0 || i >= n)) && return
    [[ ${ITEM_KIND[$i]} != "sep" ]] && break
  done
  SEL=$i
  paint
}

jump() {
  local n=${#ITEM_KIND[@]}
  ((n == 0)) && return
  case "${ITEM_KIND[$SEL]}" in
  session)
    tmux switch-client -t "=${ITEM_ARG[$SEL]}" 2>/dev/null || true
    ;;
  window)
    local wid sname
    wid=${ITEM_ARG[$SEL]%%|*}
    sname=${ITEM_ARG[$SEL]#*|}
    tmux switch-client -t "=$sname" 2>/dev/null || true
    tmux select-window -t "$wid" 2>/dev/null || true
    # jumping to our own window: move focus off the sidebar
    if [[ $wid == "$MYWID" ]]; then
      tmux last-pane -t "$MYWID" 2>/dev/null || true
    fi
    ;;
  esac
}

close_self() {
  close_pane "$TMUX_PANE" "$MYWID"
}

run() {
  : "${TMUX_PANE:?must run inside a tmux pane}"
  local pidfile key rest
  MYWID=$(tmux display-message -p -t "$TMUX_PANE" '#{window_id}')
  pidfile="$STATE_DIR/pid.$$"
  echo "$$" >"$pidfile"
  trap 'rm -f "$pidfile"' EXIT
  trap 'exit 0' HUP TERM INT
  trap 'render' USR1 WINCH
  printf '\033[?25l\033[2J'
  render
  # block on keyboard; signals interrupt the read, trap repaints, loop resumes
  while :; do
    key=""
    if ! IFS= read -rsn1 key; then
      # EINTR (event repaint) or dead tty; the tiny sleep stops an EOF spin
      sleep 0.05
      continue
    fi
    if [[ $key == $'\033' ]]; then
      rest=""
      IFS= read -rsn2 -t 0.005 rest || true
      case "$rest" in
      '[A') key=k ;;
      '[B') key=j ;;
      *) continue ;;
      esac
    fi
    case "$key" in
    j) sel_move 1 ;;
    k) sel_move -1 ;;
    g)
      SEL=0
      clamp_sel
      paint
      ;;
    G)
      SEL=$((${#ITEM_KIND[@]} - 1))
      clamp_sel
      paint
      ;;
    "") jump ;;
    q) close_self ;;
    esac
  done
}

refresh() {
  local f pid
  for f in "$STATE_DIR"/pid.*; do
    [[ -e $f ]] || continue
    pid=$(<"$f")
    # PIDs get recycled; only signal if it's really a sidebar (USR1's
    # default action TERMINATES an unsuspecting process)
    case "$(ps -p "$pid" -o command= 2>/dev/null)" in
    *demux*) kill -USR1 "$pid" 2>/dev/null || rm -f "$f" ;;
    *) rm -f "$f" ;;
    esac
  done
}

# close one sidebar pane: kill + restore its window's layout + unset, all in
# one batch so the neighbour pane never sees the intermediate full-width size
close_pane() {
  local pane=$1 wid=$2 layout
  layout=$(tmux show-options -wqv -t "$wid" @demux_saved_layout)
  if [[ -n $layout ]]; then
    tmux kill-pane -t "$pane" \; select-layout -t "$wid" "$layout" \; set-option -wu -t "$wid" @demux_saved_layout 2>/dev/null ||
      tmux kill-pane -t "$pane" 2>/dev/null || true
  else
    tmux kill-pane -t "$pane" 2>/dev/null || true
  fi
}

# the sidebar is global: toggle closes it wherever it lives, or opens it here
toggle() {
  local target pane wid layout sb sbwid found
  target="${1:-${TMUX_PANE:?no pane: pass one or run inside tmux}}"
  found=0
  while read -r sb sbwid; do
    [[ -n $sb ]] || continue
    found=1
    close_pane "$sb" "$sbwid"
  done < <(tmux list-panes -a -f '#{==:#{@demux_sidebar},1}' -F '#{pane_id} #{window_id}')
  ((found)) && return 0
  wid=$(tmux display-message -p -t "$target" '#{window_id}')
  layout=$(tmux display-message -p -t "$wid" '#{window_layout}')
  pane=$(tmux split-window -hbdf -l "$WIDTH" -t "$target" -P -F '#{pane_id}' "exec bash '$SELF' run")
  tmux set-option -w -t "$wid" @demux_saved_layout "$layout"
  tmux set-option -p -t "$pane" @demux_sidebar 1
}

# ensure the sidebar lives in the attached client's active window; wired to
# window/session switch hooks. single-client assumption, like jump().
follow() {
  local sb sbwid csess cwid oldlayout newlayout
  sb=""
  IFS=' ' read -r sb sbwid < <(tmux list-panes -a -f '#{==:#{@demux_sidebar},1}' -F '#{pane_id} #{window_id}') || true
  [[ -n $sb ]] || return 0
  csess=""
  IFS=' ' read -r csess < <(tmux list-clients -F '#{client_session}') || true
  [[ -n $csess ]] || return 0
  cwid=$(tmux display-message -p -t "=$csess" '#{window_id}')
  if [[ $cwid != "$sbwid" ]]; then
    oldlayout=$(tmux show-options -wqv -t "$sbwid" @demux_saved_layout)
    newlayout=$(tmux display-message -p -t "$cwid" '#{window_layout}')
    if [[ -n $oldlayout ]]; then
      tmux join-pane -hbdf -l "$WIDTH" -s "$sb" -t "$cwid" \; \
        select-layout -t "$sbwid" "$oldlayout" \; \
        set-option -wu -t "$sbwid" @demux_saved_layout \; \
        set-option -w -t "$cwid" @demux_saved_layout "$newlayout" 2>/dev/null || return 0
    else
      tmux join-pane -hbdf -l "$WIDTH" -s "$sb" -t "$cwid" \; \
        set-option -w -t "$cwid" @demux_saved_layout "$newlayout" 2>/dev/null || return 0
    fi
  fi
  refresh
}

case "${1:-run}" in
run) run ;;
toggle) toggle "${2:-}" ;;
refresh) refresh ;;
follow) follow ;;
*)
  echo "usage: demux-sidebar {run|toggle [pane]|refresh|follow}" >&2
  exit 2
  ;;
esac
