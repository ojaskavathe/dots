# lib.zsh — shared harness for demux rigs. Each test sources this with the
# demuxd binary as $1; it gets an ISOLATED tmux server (socket named after
# the test file + pid, so the full suite runs in parallel), the standard
# world, a fake client, a daemon, and helpers. Call rig_up first, rig_done
# last (prints PASS/FAIL totals, tears everything down, exits nonzero on
# any failure).
set -u
zmodload zsh/datetime
zmodload zsh/zpty

BIN=${1:?usage: <test>.zsh <demuxd-binary>}
RIG=${0:t:r}
L="${${RIG//[^a-zA-Z0-9]/}}$$"
T() { env -u TMUX -u TMUX_PANE tmux -L $L "$@" }
D() { env -u TMUX -u TMUX_PANE $BIN -L $L "$@" }
now_ms() { printf '%.0f' $((EPOCHREALTIME * 1000)) }

PASS=0; FAIL=0
ok()  { PASS=$((PASS+1)); echo "  ok  - $1" }
bad() { FAIL=$((FAIL+1)); echo "  FAIL- $1" }
chk() { if eval "$2"; then ok "$1"; else bad "$1 [[ $2 ]]"; fi }

rig_teardown() {
  T kill-server 2>/dev/null
  pkill -f "${BIN:t} -S /private/tmp/tmux-501/$L" 2>/dev/null
  zpty -d fake 2>/dev/null
  rm -f /tmp/demux-501/${L}-*(N) 2>/dev/null
}
trap rig_teardown INT TERM

# rig_up: standard world — session work (w1 = split with a MARKW1 loop pane,
# beta, gamma), session play (2 windows), fake 200x50 client on work:beta,
# per-window layout baselines (captured while each window is current: the
# client's status line resizes only the current window), daemon started.
# Exports: W1 W2 W3 P1 L_W1 L_W2 L_W3 CL SOCK SP(unset until pinned).
rig_up() {
  rig_teardown; sleep 0.3
  T -f /dev/null new-session -d -s work -x 200 -y 50
  W1=$(T display-message -p -t work: '#{window_id}')
  T split-window -h -t $W1 'while :; do echo MARKW1; sleep 2; done'
  T new-window -t work: -n beta >/dev/null
  W2=$(T list-windows -t work -F '#{window_id} #{window_name}' | awk '$2=="beta"{print $1}')
  T new-window -t work: -n gamma >/dev/null
  W3=$(T list-windows -t work -F '#{window_id} #{window_name}' | awk '$2=="gamma"{print $1}')
  T new-session -d -s play -x 200 -y 50
  P1=$(T display-message -p -t play: '#{window_id}')
  T new-window -t play: -n ptwo >/dev/null
  T select-window -t $W2
  sleep 0.3
  zpty fake "stty rows 50 cols 200; exec env -u TMUX -u TMUX_PANE tmux -L $L attach -t work"
  sleep 1
  T select-window -t $W1; sleep 0.2
  L_W1=$(T display-message -p -t $W1 '#{window_layout}')
  T select-window -t $W3; sleep 0.2
  L_W3=$(T display-message -p -t $W3 '#{window_layout}')
  T select-window -t $W2; sleep 0.2
  L_W2=$(T display-message -p -t $W2 '#{window_layout}')
  CL=$(T list-clients -F '#{client_name} #{client_control_mode}' | awk '$2==0{print $1; exit}')
  [[ -n $CL ]] || { echo "no fake client"; exit 1 }
  D ls >/dev/null; sleep 0.5
  SOCK=$(D sock | awk '/demux:/{print $2}')
}

side()       { T list-panes -a -F '#{pane_id} #{pane_current_command} #{window_id} #{pane_left} #{pane_width} #{pane_active}' | awk '$2 ~ /demux/ {print; exit}' }
clientwin()  { T list-clients -F '#{client_name} #{window_id}' | awk -v c=$CL '$1==c{print $2}' }
clientsess() { T list-clients -F '#{client_name} #{session_name}' | awk -v c=$CL '$1==c{print $2}' }
zoomflag()   { T display-message -p -t $1 '#{window_zoomed_flag}' }
spacers()    { T list-panes -a -F '#{pane_start_command}' | grep -c 'sleep 100000001' }

# wait_until <check> [tries=100]: poll a condition at 10ms
wait_until() { local i; for i in {1..${2:-100}}; do eval "$1" && return 0; sleep 0.01; done; return 1 }

rig_done() {
  echo
  echo "PASS=$PASS FAIL=$FAIL"
  rig_teardown
  (( FAIL == 0 ))
  exit $?
}
