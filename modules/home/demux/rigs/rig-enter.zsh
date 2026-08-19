#!/usr/bin/env zsh
# Reproduce intermittent slow/flickery enters: nvim-heavy windows, scrub then
# Enter after varying pauses; the daemon log attributes where time goes.
set -u
zmodload zsh/datetime; zmodload zsh/zpty
BIN=$1; L=rigenter
T() { env -u TMUX -u TMUX_PANE tmux -L $L "$@" }
now_ms() { printf '%.0f' $((EPOCHREALTIME * 1000)) }
T kill-server 2>/dev/null; pkill -f "${BIN:t} -S /private/tmp/tmux-501/$L" 2>/dev/null
rm -f /tmp/demux-501/${L}-*(N) 2>/dev/null; sleep 0.4

T -f /dev/null new-session -d -s work -x 230 -y 68
W1=$(T display-message -p -t work: '#{window_id}')
# heavy windows: nvim splits over real source (big reflow on every WINCH)
T send-keys -t $W1 "nvim -O2 ~/dots/modules/home/demux/demuxd/pin.go ~/dots/modules/home/demux/demuxd/browse.go" Enter
for n in beta gamma delta; do
  T new-window -t work: -n $n >/dev/null
  T send-keys -t work:$n "nvim -O2 ~/dots/modules/home/demux/demuxd/tui.go ~/dots/modules/home/demux/demuxd/control.go" Enter
done
T split-window -v -t $W1 'while :; do echo MARKW1; sleep 0.3; done'
# a second session so scrubs cross sessions (status pad + savedStatus path),
# with panes streaming like agent sessions do
T new-session -d -s agents -x 230 -y 68
T send-keys -t agents: 'while :; do seq 500; sleep 0.05; done' Enter
T new-window -t agents: -n a2 >/dev/null
T send-keys -t agents:a2 'while :; do seq 500; sleep 0.05; done' Enter
T new-window -t agents: -n a3 >/dev/null
T send-keys -t agents:a3 "nvim -O2 ~/dots/modules/home/demux/demuxd/pin.go" Enter
sleep 3

zpty fake "stty rows 68 cols 230; exec env -u TMUX -u TMUX_PANE tmux -L $L attach -t work"
sleep 1
CL=$(T list-clients -F '#{client_name} #{client_control_mode}' | awk '$2==0{print $1; exit}')
env -u TMUX -u TMUX_PANE $BIN -L $L ls >/dev/null; sleep 0.5
env -u TMUX -u TMUX_PANE $BIN -L $L toggle $CL; sleep 1.5
SP=$(T list-panes -a -F '#{pane_id} #{pane_current_command}' | awk '$2 ~ /demux/{print $1; exit}')
[[ -z $SP ]] && { echo "NO SIDEBAR"; exit 1 }

LOG=$(ls /tmp/demux-501/${L}-*.sock.log 2>/dev/null | head -1)
echo "log: $LOG"

# cadence sweep: scrub two rows, pause D ms, Enter, settle. Alternate
# direction so every rep lands on a different window (selection follows the
# entered window, so same-direction reps run off the end of the list).
K=k
for D in 50 120 180 250 400; do
  for rep in 1 2 3; do
    T send-keys -t $SP $K; sleep 0.15
    T send-keys -t $SP $K; sleep $((D / 1000.0))
    t0=$(now_ms)
    T send-keys -t $SP Enter
    sleep 1.2
    echo "pause=${D}ms rep=$rep dir=$K sent_at=$t0"
    [[ $K == k ]] && K=j || K=k
  done
done
sleep 0.5
echo "=== slow lines ==="
grep -E 'took|slow rt|relist' $LOG | tail -60
T kill-server 2>/dev/null
zpty -d fake 2>/dev/null
