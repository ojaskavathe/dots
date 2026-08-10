# demux (frame spike): sidebar chrome AROUND your real tmux, not inside it.
#
#   up [inner-socket]   launch/attach the frame (default subcommand)
#   ensure [socket]     build the frame without attaching (used by tests)
#   run                 the sidebar process (lives in the outer frame)
#   refresh             signal sidebars to repaint (wire inner hooks to this)
#   frame-focus [pane]  M-s: open the sidebar / focus it / close if focused
#
# The outer frame server owns exactly two panes: the sidebar and a nested
# client attached to your real tmux. Your windows never contain the sidebar,
# so no demux operation can rearrange your splits — the entire class of
# select-layout/join-pane corruption is structurally gone.
#
# Sidebar keys: j/k or arrows preview (switches the inner client live),
# g/G first/last, Enter commits focus into your tmux, q closes the sidebar.

set -euo pipefail

SELF="${BASH_SOURCE[0]}"
STATE_DIR="${XDG_RUNTIME_DIR:-$HOME/.cache}/demux"
WIDTH="${DEMUX_WIDTH:-32}"
FRAME_CONF="${DEMUX_FRAME_CONF:-@frameconf@}"
FRAME_SOCKET="${DEMUX_FRAME_SOCKET:-demux-frame}"
mkdir -p "$STATE_DIR"

# inner = your real tmux server; DEMUX_INNER is its socket path, stored in
# the frame server's global environment by `up`
itmux() { tmux -S "${DEMUX_INNER:?DEMUX_INNER not set (run inside the demux frame)}" "$@"; }

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
ITEM_ARG=()   # "session|active_window_id" or "window_id|session"
SEL=0
MYSESSION=""
CURWID=""

inner_client() {
  local s=""
  IFS=' ' read -r s < <(itmux list-clients -F '#{client_session}' 2>/dev/null) || true
  MYSESSION=$s
  CURWID=""
  [[ -n $s ]] && CURWID=$(itmux display-message -p -t "=$s" '#{window_id}' 2>/dev/null) || true
}

build() {
  local width prev sess_idx
  local sname attached wcount widx wid wactive wpath scolor dot dotchar mark wcolor smax wmax
  width=$(tmux display-message -p -t "$TMUX_PANE" '#{pane_width}')
  # self-heal our width; the frame layout is trivial so this is always safe
  if ((width != WIDTH)); then
    tmux resize-pane -t "$TMUX_PANE" -x "$WIDTH" 2>/dev/null || true
    width=$WIDTH
  fi
  inner_client
  smax=$((width - 8))
  wmax=$((width - 7))
  ITEM_TEXT=()
  ITEM_PLAIN=()
  ITEM_KIND=()
  ITEM_ARG=()
  prev=""
  sess_idx=-1
  while IFS='|' read -r sname attached wcount widx wid wactive wpath; do
    if [[ $sname != "$prev" ]]; then
      prev=$sname
      scolor=$C_SES
      [[ $sname == "$MYSESSION" ]] && scolor=$C_SES_ME
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
      ITEM_ARG+=("${sname}|")
      sess_idx=$((${#ITEM_KIND[@]} - 1))
    fi
    mark=" "
    wcolor=$C_DIM
    if [[ $wactive == 1 ]]; then
      mark="▸"
      wcolor=$R
      ((sess_idx >= 0)) && ITEM_ARG[sess_idx]="${sname}|${wid}"
    fi
    ITEM_TEXT+=("   ${C_DIM}${widx}${R} ${wcolor}${mark}${wpath:0:wmax}${R}")
    ITEM_PLAIN+=("   ${widx} ${mark}${wpath:0:wmax}")
    ITEM_KIND+=("window")
    ITEM_ARG+=("${wid}|${sname}")
  done < <(itmux list-windows -a -F '#{session_name}|#{session_attached}|#{session_windows}|#{window_index}|#{window_id}|#{window_active}|#{b:pane_current_path}' 2>/dev/null)
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

# signal-safe render: never nest (bash runs traps between commands, and a
# nested build duplicates entries); coalesce instead
IN_RENDER=0
PENDING=0
render() {
  if ((IN_RENDER)); then
    PENDING=1
    return 0
  fi
  IN_RENDER=1
  PENDING=1
  while ((PENDING)); do
    PENDING=0
    build
    paint
  done
  IN_RENDER=0
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
  preview
}

# the sidebar is a previewer: moving the selection switches the INNER client
# immediately. one itmux call; the sidebar itself never moves.
preview() {
  local n=${#ITEM_KIND[@]} twid sname
  ((n == 0)) && return
  case "${ITEM_KIND[$SEL]}" in
  session)
    sname=${ITEM_ARG[$SEL]%%|*}
    if [[ -n $sname && $sname != "$MYSESSION" ]]; then
      itmux switch-client -t "=$sname" 2>/dev/null || true
      MYSESSION=$sname
    fi
    ;;
  window)
    twid=${ITEM_ARG[$SEL]%%|*}
    sname=${ITEM_ARG[$SEL]#*|}
    if [[ $twid != "$CURWID" ]]; then
      itmux switch-client -t "=$sname" \; select-window -t "$twid" 2>/dev/null || true
      MYSESSION=$sname
      CURWID=$twid
    fi
    ;;
  esac
}

# enter commits: the view already switched; move focus into your tmux
land() {
  tmux last-pane 2>/dev/null || tmux select-pane -t :.+ 2>/dev/null || true
}

close_self() {
  tmux kill-pane -t "$TMUX_PANE" 2>/dev/null || true
}

run() {
  : "${TMUX_PANE:?must run inside a tmux pane}"
  local pidfile key rest i
  pidfile="$STATE_DIR/pid.$$"
  echo "$$" >"$pidfile"
  trap 'rm -f "$pidfile"' EXIT
  trap 'exit 0' HUP TERM INT
  trap 'render' USR1 WINCH
  printf '\033[?25l\033[2J'
  render
  # start the selection on the inner client's current window
  for i in "${!ITEM_KIND[@]}"; do
    if [[ ${ITEM_KIND[$i]} == "window" && ${ITEM_ARG[$i]%%|*} == "$CURWID" ]]; then
      SEL=$i
      paint
      break
    fi
  done
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
      preview
      ;;
    G)
      SEL=$((${#ITEM_KIND[@]} - 1))
      clamp_sel
      paint
      preview
      ;;
    "") land ;;
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

# M-s in the frame: open the sidebar / focus it / close if already focused.
# runs via the OUTER server's run-shell, so plain tmux talks to the frame.
frame_focus() {
  local active sb pane
  active="${1:-}"
  sb=$(tmux list-panes -f '#{==:#{@demux_sidebar},1}' -F '#{pane_id}')
  if [[ -z $sb ]]; then
    pane=$(tmux split-window -hbf -l "$WIDTH" -P -F '#{pane_id}' "exec bash '$SELF' run")
    tmux set-option -p -t "$pane" @demux_sidebar 1
  elif [[ $active == "$sb" ]]; then
    tmux kill-pane -t "$sb"
  else
    tmux select-pane -t "$sb"
  fi
}

frame_ensure() {
  local inner_sock=${1:-}
  if [[ -z $inner_sock ]]; then
    if ! env -u TMUX -u TMUX_PANE tmux has-session 2>/dev/null; then
      env -u TMUX -u TMUX_PANE tmux new-session -d -s main
    fi
    inner_sock=$(env -u TMUX -u TMUX_PANE tmux display-message -p '#{socket_path}')
  fi
  local F=(tmux -L "$FRAME_SOCKET")
  if ! "${F[@]}" has-session -t frame 2>/dev/null; then
    tmux -L "$FRAME_SOCKET" -f "$FRAME_CONF" new-session -d -s frame -n frame \
      "exec env -u TMUX -u TMUX_PANE tmux -S '$inner_sock' attach"
    "${F[@]}" set-environment -g DEMUX_INNER "$inner_sock"
    "${F[@]}" bind -n M-s run-shell -b "bash '$SELF' frame-focus '#{pane_id}'"
    # frame starts with the sidebar open
    DEMUX_INNER=$inner_sock frame_focus_in_frame "$inner_sock"
  fi
}

frame_focus_in_frame() {
  local inner_sock=$1 pane
  pane=$(tmux -L "$FRAME_SOCKET" split-window -hbf -l "$WIDTH" -t frame -P -F '#{pane_id}' "exec bash '$SELF' run")
  tmux -L "$FRAME_SOCKET" set-option -p -t "$pane" @demux_sidebar 1
}

up() {
  frame_ensure "${1:-}"
  exec env -u TMUX -u TMUX_PANE tmux -L "$FRAME_SOCKET" attach -t frame
}

case "${1:-up}" in
up) up "${2:-}" ;;
ensure) frame_ensure "${2:-}" ;;
run) run ;;
refresh) refresh ;;
frame-focus) frame_focus "${2:-}" ;;
*)
  echo "usage: demux {up [inner-socket]|run|refresh|frame-focus [pane]}" >&2
  exit 2
  ;;
esac
