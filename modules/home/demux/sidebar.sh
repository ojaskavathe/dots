# demux: session/window sidebar for tmux. three modes, one core.
#
#   focus [pane]    PANE mode summon (M-s): open the sidebar as a real pane
#                   / focus it / close if focused
#   toggle [pane]   PANE mode: open here or close wherever it is
#   follow          PANE mode hook: sidebar joins the active window
#   nav next|prev [window]  PANE mode: cycle windows, sidebar rides along
#   popup           POPUP mode: same sidebar inside display-popup (overlay)
#   up [socket]     FRAME mode: outer chrome tmux hosting sidebar + nested
#                   client ("ensure" builds without attaching; for tests)
#   run             the sidebar process itself
#   refresh         signal sidebars to repaint (wired to tmux hooks)
#
# HARD-WON RULES (see thoughts/demux-architecture.md):
# - NEVER call select-layout: tmux assigns layout geometry to panes by index
#   order, ignoring the pane ids in the string; join-pane -b desyncs index
#   from geometric order, so any layout restore shuffles pane contents.
# - join-pane / kill-pane / resize-pane are content-safe: they target panes
#   by id and never remap. width restoration is therefore done by recording
#   pane widths on entry (@demux_widths) and resize-pane'ing them back on
#   leave. structure is never touched.
#
# Sidebar keys: j/k or arrows preview (switches the view live), g/G
# first/last, Enter commits, q closes (popup: cancels back to origin).

set -euo pipefail

SELF="${BASH_SOURCE[0]}"
STATE_DIR="${XDG_RUNTIME_DIR:-$HOME/.cache}/demux"
WIDTH="${DEMUX_WIDTH:-32}"
FRAME_CONF="${DEMUX_FRAME_CONF:-@frameconf@}"
FRAME_SOCKET="${DEMUX_FRAME_SOCKET:-demux-frame}"
mkdir -p "$STATE_DIR"

# inner = the tmux server being browsed. FRAME mode carries its socket in
# DEMUX_INNER; pane/popup modes are already inside it.
itmux() {
  if [[ -n ${DEMUX_INNER:-} ]]; then
    tmux -S "$DEMUX_INNER" "$@"
  else
    tmux "$@"
  fi
}

POPUP="${DEMUX_POPUP:-}"
PANEMODE=""
ORIG_SESSION=""
ORIG_WID=""
# the client this sidebar serves. NEVER rely on tmux's "current client"
# resolution: with a second client attached (another terminal, a nested
# frame), implicit switch-client acts on the wrong one.
CLIENT=""

# switch-client args targeting our client explicitly when known
sw_args() {
  SW=(switch-client)
  [[ -n $CLIENT ]] && SW+=(-c "$CLIENT")
}

# pane-mode movers run as concurrent processes (hooks + keybinds);
# serialize. noclobber redirection = atomic create with zero forks.
lock() {
  local i=0
  while ! { set -C && : >"$STATE_DIR/lock.f"; } 2>/dev/null; do
    set +C
    ((++i >= 40)) && rm -f "$STATE_DIR/lock.f" 2>/dev/null
    ((i >= 45)) && return 0
    sleep 0.01
  done
  set +C
}

unlock() {
  rm -f "$STATE_DIR/lock.f" 2>/dev/null || true
}

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

ITEM_TEXT=()  # colored line
ITEM_PLAIN=() # uncolored line (used when selected, so reverse video survives)
ITEM_KIND=()  # session | window | sep
ITEM_ARG=()   # "session|active_window_id" or "window_id|session"
SEL=0
MYSESSION=""
MYWID=""
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
  local sname attached wcount widx wid wactive pactive issb ppath
  local scolor dot dotchar mark wcolor smax wmax cur_wid cur_widx cur_wactive cur_sname cur_label label_locked
  if [[ -n $POPUP || -z ${TMUX_PANE:-} ]]; then
    IFS=' ' read -r _ width < <(stty size </dev/tty 2>/dev/null) || width=$WIDTH
    [[ -n ${width:-} ]] || width=$WIDTH
  else
    width=$(tmux display-message -p -t "$TMUX_PANE" '#{pane_width}')
    if ((width != WIDTH)); then
      tmux resize-pane -t "$TMUX_PANE" -x "$WIDTH" 2>/dev/null || true
      width=$WIDTH
    fi
  fi
  if [[ -n $PANEMODE ]]; then
    # follow moves this pane between windows; re-read where we are (and
    # rewarm the width-record cache for the next leave, same call)
    IFS='|' read -r MYSESSION MYWID WREC < <(tmux display-message -p -t "$TMUX_PANE" '#{session_name}|#{window_id}|#{@demux_widths}')
    CURWID=$MYWID
  else
    inner_client
  fi
  smax=$((width - 8))
  wmax=$((width - 7))
  ITEM_TEXT=()
  ITEM_PLAIN=()
  ITEM_KIND=()
  ITEM_ARG=()
  prev=""
  sess_idx=-1
  cur_wid=""

  # windows flush after their panes are seen so labels can skip the sidebar
  # pane (else focusing the sidebar relabels the window to its own cwd)
  flush_window() {
    [[ -n $cur_wid ]] || return 0
    mark=" "
    wcolor=$C_DIM
    if [[ $cur_wactive == 1 ]]; then
      mark="▸"
      wcolor=$R
    fi
    ITEM_TEXT+=("   ${C_DIM}${cur_widx}${R} ${wcolor}${mark}${cur_label:0:wmax}${R}")
    ITEM_PLAIN+=("   ${cur_widx} ${mark}${cur_label:0:wmax}")
    ITEM_KIND+=("window")
    ITEM_ARG+=("${cur_wid}|${cur_sname}")
    if [[ $cur_wactive == 1 ]] && ((sess_idx >= 0)); then
      ITEM_ARG[sess_idx]="${cur_sname}|${cur_wid}"
    fi
    cur_wid=""
  }

  while IFS='|' read -r sname attached wcount widx wid wactive pactive issb ppath; do
    if [[ $sname != "$prev" ]]; then
      flush_window
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
    if [[ $wid != "$cur_wid" ]]; then
      flush_window
      cur_wid=$wid
      cur_widx=$widx
      cur_wactive=$wactive
      cur_sname=$sname
      cur_label=""
      label_locked=0
    fi
    if [[ $issb != 1 ]]; then
      if [[ $pactive == 1 ]]; then
        cur_label=$ppath
        label_locked=1
      elif [[ -z $cur_label && $label_locked == 0 ]]; then
        cur_label=$ppath
      fi
    fi
  done < <(itmux list-panes -a -F '#{session_name}|#{session_attached}|#{session_windows}|#{window_index}|#{window_id}|#{window_active}|#{pane_active}|#{@demux_sidebar}|#{b:pane_current_path}' 2>/dev/null)
  flush_window
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

# ---- width bookkeeping (pane mode) ------------------------------------
# @demux_widths on a window = "id=w id=w ..." recorded BEFORE the sidebar
# joined; leaving resize-panes them back. resize-pane is content-safe.

OLD_WIDTHS=""
WREC="" # cached width record of the window we're currently in

# old window's record + its LIVE panes in one call. tmux ABORTS a command
# sequence at the first error, so a resize targeting a dead pane would kill
# every command after it — records must be filtered to live panes.
read_move_state() {
  local oldwid=$1 line rec="" live=" " kv keep=""
  OLD_WIDTHS=""
  while IFS= read -r line; do
    case "$line" in
    O%*) live+="${line#O} " ;;
    *) rec=$line ;;
    esac
  done < <(itmux list-panes -t "$oldwid" -F 'O#{pane_id}' \; show-options -wqv -t "$oldwid" @demux_widths 2>/dev/null)
  # shellcheck disable=SC2086
  for kv in $rec; do
    [[ $live == *" ${kv%%=*} "* ]] && keep+="$kv "
  done
  OLD_WIDTHS=${keep% }
}

resize_args() { # appends resize-pane commands for record "$1" to CMD
  local kv
  # shellcheck disable=SC2086
  for kv in $1; do
    CMD+=(resize-pane -t "${kv%%=*}" -x "${kv#*=}" ";")
  done
}

# ---- pane-mode movement ------------------------------------------------

# hot path: sidebar (us) moves itself to $1, runs trailing commands in the
# same batch. two tmux spawns per keypress.
# restore a left-behind window's widths from its record; runs async off the
# keypress path. filters against live panes (sequences abort on error).
# REFUSES to touch a window the sidebar re-entered meanwhile: restoring
# full-width values around a present sidebar over-constrains the layout,
# and clearing the record would drop the true baseline (whiplash scrubbing)
restore_widths() {
  local wid=$1 rec=$2 line live=" " kv hassb=0
  while IFS= read -r line; do
    case "$line" in
    *S)
      hassb=1
      live+="${line%S} "
      ;;
    *) live+="$line " ;;
    esac
  done < <(itmux list-panes -t "$wid" -F '#{pane_id}#{?#{@demux_sidebar},S,}' 2>/dev/null)
  ((hassb)) && return 0
  # unset first (guaranteed cleanup even if a resize aborts the rest), then
  # best-effort resizes for panes that were alive at read time
  local str="set-option -wu -t '$wid' @demux_widths"
  # shellcheck disable=SC2086
  for kv in $rec; do
    [[ $live == *" ${kv%%=*} "* ]] && str+=" ; resize-pane -t '${kv%%=*}' -x ${kv#*=}"
  done
  # the client-side check above races the sidebar whiplashing back in; this
  # if-shell re-checks presence SERVER-SIDE, atomic with the restore, so
  # full-width values can never be applied around a present sidebar (which
  # over-constrains the window, crushes panes, and poisons later baselines)
  itmux if-shell -t "$wid" -F '#{m:*S*,#{P:#{?#{@demux_sidebar},S,}}}' '' "$str" 2>/dev/null || true
}

# HOT PATH: exactly one synchronous tmux call per keypress. the target's
# width record is captured server-side (set-option -F, pre-join, target
# context) and echoed back for our cache; @demux_nav stamps the move so
# follow ignores hooks we caused ourselves. the window we left is restored
# asynchronously, then a self-signal repaints markers off the keypress path.
preview_move() {
  local twid=$1
  shift
  local oldwid=$MYWID oldrec=$WREC out
  # -oq: capture the width baseline only if none exists — a pending async
  # restore means the stored record is the TRUE pre-sidebar baseline, and
  # overwriting it with absorbed widths would corrupt every later restore
  if ! out=$(itmux set-option -Fwoq -t "$twid" @demux_widths '#{P:#{pane_id}=#{pane_width} }' \; \
    set-option -g @demux_nav "$EPOCHSECONDS" \; \
    join-pane -hbdf -l "$WIDTH" -s "$TMUX_PANE" -t "$twid" \; \
    "$@" \; \
    display-message -p -t "$twid" '#{@demux_widths}' 2>/dev/null); then
    render
    return 0
  fi
  WREC=$out
  MYWID=$twid
  (
    restore_widths "$oldwid" "$oldrec"
    kill -USR1 $$ 2>/dev/null
  ) &
}

# external movers (follow/nav/focus): find the sidebar, move it
move_with() {
  local twid=$1
  shift
  local sb="" sbwid=""
  IFS=' ' read -r sb sbwid < <(itmux list-panes -a -f '#{==:#{@demux_sidebar},1}' -F '#{pane_id} #{window_id}') || true
  local CMD=()
  if [[ -n $sb && $sbwid != "$twid" ]]; then
    read_move_state "$sbwid"
    CMD+=(set-option -g @demux_nav "$EPOCHSECONDS" ";")
    CMD+=(set-option -Fwoq -t "$twid" @demux_widths '#{P:#{pane_id}=#{pane_width} }' ";")
    CMD+=(join-pane -hbdf -l "$WIDTH" -s "$sb" -t "$twid" ";")
    [[ ${#} -gt 0 ]] && CMD+=("$@" ";")
    resize_args "$OLD_WIDTHS"
    CMD+=(set-option -wu -t "$sbwid" @demux_widths)
  else
    CMD+=("$@")
  fi
  ((${#CMD[@]} > 0)) && itmux "${CMD[@]}" 2>/dev/null || true
}

close_pane() {
  local pane=$1 wid=$2 line rec="" live=" " kv keep=""
  while IFS= read -r line; do
    case "$line" in
    O%*) live+="${line#O} " ;;
    *) rec=$line ;;
    esac
  done < <(itmux list-panes -t "$wid" -F 'O#{pane_id}' \; show-options -wqv -t "$wid" @demux_widths 2>/dev/null)
  # shellcheck disable=SC2086
  for kv in $rec; do
    [[ ${kv%%=*} == "$pane" ]] && continue
    [[ $live == *" ${kv%%=*} "* ]] && keep+="$kv "
  done
  local CMD=(kill-pane -t "$pane" ";")
  resize_args "${keep% }"
  CMD+=(set-option -wu -t "$wid" @demux_widths ";" set-option -gu @demux_open)
  itmux "${CMD[@]}" 2>/dev/null || itmux kill-pane -t "$pane" 2>/dev/null || true
}

open_at() {
  local target=$1 client=${2:-} pane rec wid
  wid=$(itmux display-message -p -t "$target" '#{window_id}')
  rec=$(itmux list-panes -t "$wid" -F '#{pane_id}=#{pane_width}' 2>/dev/null | tr '\n' ' ')
  rec=${rec% }
  # no -d: opening the sidebar focuses it, ready for j/k/enter
  pane=$(itmux split-window -hbf -l "$WIDTH" -t "$target" -P -F '#{pane_id}' "exec bash '$SELF' run")
  itmux set-option -w -t "$wid" @demux_widths "$rec" \; set-option -p -t "$pane" @demux_sidebar 1 \; set-option -p -t "$pane" @demux_client "$client" \; set-option -g @demux_open 1
}

# the sidebar is global: toggle closes it wherever it lives, or opens here
toggle() {
  local target client sb sbwid found
  target="${1:-${TMUX_PANE:?no pane: pass one or run inside tmux}}"
  client="${2:-}"
  lock
  found=0
  while read -r sb sbwid; do
    [[ -n $sb ]] || continue
    found=1
    close_pane "$sb" "$sbwid"
  done < <(itmux list-panes -a -f '#{==:#{@demux_sidebar},1}' -F '#{pane_id} #{window_id}')
  ((found)) || open_at "$target" "$client"
  unlock
}

# summon cycle: closed -> open focused; open but unfocused -> pull here and
# focus; already focused (keybind's active pane IS the sidebar) -> close
focus() {
  local target client sb sbwid cwid
  target="${1:-${TMUX_PANE:?no pane: pass one or run inside tmux}}"
  client="${2:-}"
  lock
  sb=""
  IFS=' ' read -r sb sbwid < <(itmux list-panes -a -f '#{==:#{@demux_sidebar},1}' -F '#{pane_id} #{window_id}') || true
  if [[ -z $sb ]]; then
    open_at "$target" "$client"
  elif [[ $target == "$sb" ]]; then
    close_pane "$sb" "$sbwid"
  else
    cwid=$(itmux display-message -p -t "$target" '#{window_id}')
    if [[ $sbwid != "$cwid" ]]; then
      move_with "$cwid" select-pane -t "$sb"
      unlock
      refresh
      return 0
    fi
    itmux select-pane -t "$sb" 2>/dev/null || true
  fi
  unlock
}

# hook fallback for switches demux didn't make itself. follows the CLIENT
# the sidebar was opened for, never "some client".
follow() {
  local sb sbwid sbclient cwid ts
  # skip hooks demux's own moves triggered (@demux_nav stamps them); the
  # sidebar repaints itself, and these no-op follows were serializing
  # behind the preview lock, lagging every keypress
  ts=$(itmux show-options -gqv @demux_nav 2>/dev/null) || ts=""
  [[ -n $ts ]] && ((EPOCHSECONDS - ts <= 1)) && return 0
  sb=""
  IFS='|' read -r sb sbwid sbclient < <(itmux list-panes -a -f '#{==:#{@demux_sidebar},1}' -F '#{pane_id}|#{window_id}|#{@demux_client}') || true
  [[ -n $sb ]] || return 0
  lock
  cwid=""
  if [[ -n $sbclient ]]; then
    cwid=$(itmux display-message -p -c "$sbclient" '#{window_id}' 2>/dev/null) || cwid=""
  fi
  if [[ -z $cwid ]]; then
    local csess=""
    IFS=' ' read -r csess < <(itmux list-clients -F '#{client_session}') || true
    [[ -n $csess ]] && cwid=$(itmux display-message -p -t "=$csess" '#{window_id}' 2>/dev/null) || true
  fi
  [[ -n $cwid && $cwid != "$sbwid" ]] && move_with "$cwid"
  unlock
  refresh
}

# window cycling that carries the sidebar in the same batch; bound to
# M-h/M-l only while the sidebar is open (@demux_open)
nav() {
  local dir=$1 cw=${2:-} csess wids=() i n target
  [[ -n $cw ]] || return 0
  csess=$(itmux display-message -p -t "$cw" '#{session_name}')
  while read -r i; do wids+=("$i"); done < <(itmux list-windows -t "=$csess" -F '#{window_id}')
  n=${#wids[@]}
  ((n > 1)) || return 0
  target=""
  for i in "${!wids[@]}"; do
    if [[ ${wids[$i]} == "$cw" ]]; then
      case "$dir" in
      next) target=${wids[$(((i + 1) % n))]} ;;
      prev) target=${wids[$(((i - 1 + n) % n))]} ;;
      esac
      break
    fi
  done
  [[ -n $target ]] || return 0
  lock
  move_with "$target" select-window -t "$target"
  unlock
  refresh
}

# ---- previews (all modes) ---------------------------------------------

preview() {
  local n=${#ITEM_KIND[@]} twid sname
  ((n == 0)) && return
  local SW
  sw_args
  if [[ -n $PANEMODE ]]; then
    lock
    case "${ITEM_KIND[$SEL]}" in
    session)
      sname=${ITEM_ARG[$SEL]%%|*}
      twid=${ITEM_ARG[$SEL]#*|}
      if [[ -n $twid && $twid != "$MYWID" ]]; then
        preview_move "$twid" "${SW[@]}" -t "=$sname" ";" select-pane -t "$TMUX_PANE"
      fi
      ;;
    window)
      twid=${ITEM_ARG[$SEL]%%|*}
      sname=${ITEM_ARG[$SEL]#*|}
      if [[ $twid != "$MYWID" ]]; then
        preview_move "$twid" "${SW[@]}" -t "=$sname" ";" select-window -t "$twid" ";" select-pane -t "$TMUX_PANE"
      fi
      ;;
    esac
    unlock
    return
  fi
  # popup/frame: the sidebar never moves; just steer the inner client
  case "${ITEM_KIND[$SEL]}" in
  session)
    sname=${ITEM_ARG[$SEL]%%|*}
    if [[ -n $sname && $sname != "$MYSESSION" ]]; then
      itmux "${SW[@]}" -t "=$sname" 2>/dev/null || true
      MYSESSION=$sname
    fi
    ;;
  window)
    twid=${ITEM_ARG[$SEL]%%|*}
    sname=${ITEM_ARG[$SEL]#*|}
    if [[ $twid != "$CURWID" ]]; then
      itmux "${SW[@]}" -t "=$sname" \; select-window -t "$twid" 2>/dev/null || true
      MYSESSION=$sname
      CURWID=$twid
    fi
    ;;
  esac
}

# enter commits: the view already switched; leave the sidebar
land() {
  if [[ -n $POPUP ]]; then
    exit 0
  fi
  tmux last-pane 2>/dev/null || tmux select-pane -t :.+ 2>/dev/null || true
}

# q: pane/frame close the sidebar; popup cancels back to the origin
close_self() {
  if [[ -n $POPUP ]]; then
    if [[ -n $ORIG_SESSION ]]; then
      local SW
      sw_args
      itmux "${SW[@]}" -t "=$ORIG_SESSION" 2>/dev/null || true
      [[ -n $ORIG_WID ]] && itmux select-window -t "$ORIG_WID" 2>/dev/null || true
    fi
    exit 0
  fi
  if [[ -n $PANEMODE ]]; then
    lock
    close_pane "$TMUX_PANE" "$MYWID"
    unlock
    return
  fi
  tmux kill-pane -t "$TMUX_PANE" 2>/dev/null || true
}

run() {
  [[ -n $POPUP || -n ${TMUX_PANE:-} ]] || {
    echo "demux run: need a tmux pane or popup" >&2
    exit 1
  }
  [[ -z $POPUP && -z ${DEMUX_INNER:-} && -n ${TMUX_PANE:-} ]] && PANEMODE=1
  # resolve the client we serve (set by open_at for pane mode; the popup
  # runs in its client's context; frame's nested client is the only one)
  if [[ -n $PANEMODE ]]; then
    CLIENT=$(tmux display-message -p -t "$TMUX_PANE" '#{@demux_client}' 2>/dev/null) || CLIENT=""
  elif [[ -n $POPUP ]]; then
    CLIENT=$(tmux display-message -p '#{client_name}' 2>/dev/null) || CLIENT=""
  fi
  local pidfile key rest i
  pidfile="$STATE_DIR/pid.$$"
  echo "$$" >"$pidfile"
  trap 'rm -f "$pidfile"' EXIT
  trap 'exit 0' HUP TERM INT
  trap 'render' USR1 WINCH
  printf '\033[?25l\033[2J'
  render
  # remember where the client started, so popup-q can cancel back to it
  ORIG_SESSION=$MYSESSION
  ORIG_WID=$CURWID
  # start the selection on the current window
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
      s)
        # M-s inside the popup closes it, keeping the previewed view
        [[ -n $POPUP ]] && exit 0
        continue
        ;;
      '')
        [[ -n $POPUP ]] && exit 0
        continue
        ;;
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

# ---- frame mode (kept as an alternative; nesting breaks image passthrough)

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
    local pane
    pane=$(tmux -L "$FRAME_SOCKET" split-window -hbf -l "$WIDTH" -t frame -P -F '#{pane_id}' "exec bash '$SELF' run")
    tmux -L "$FRAME_SOCKET" set-option -p -t "$pane" @demux_sidebar 1
  fi
}

up() {
  frame_ensure "${1:-}"
  exec env -u TMUX -u TMUX_PANE tmux -L "$FRAME_SOCKET" attach -t frame
}

case "${1:-focus}" in
focus) focus "${2:-}" "${3:-}" ;;
toggle) toggle "${2:-}" "${3:-}" ;;
follow) follow ;;
nav) nav "${2:?next|prev}" "${3:-}" ;;
popup)
  DEMUX_POPUP=1
  POPUP=1
  run
  ;;
up) up "${2:-}" ;;
ensure) frame_ensure "${2:-}" ;;
run) run ;;
refresh) refresh ;;
frame-focus) frame_focus "${2:-}" ;;
*)
  echo "usage: demux {focus|toggle|follow|nav next|prev|popup|up|run|refresh}" >&2
  exit 2
  ;;
esac
