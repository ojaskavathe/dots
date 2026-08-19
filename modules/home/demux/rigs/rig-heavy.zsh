#!/usr/bin/env zsh
# The live-@4 scenario: huge-scrollback window. First billboard pays one
# carve; every enter/leave/re-enter must be swap-fast with no slow RTs.
set -u
zmodload zsh/zpty
BIN=$1; L=righeavy
T() { env -u TMUX -u TMUX_PANE tmux -L $L "$@" }
T kill-server 2>/dev/null; pkill -f "${BIN:t} -S /private/tmp/tmux-501/$L" 2>/dev/null
rm -f /tmp/demux-501/${L}-*(N) 2>/dev/null; sleep 0.4
T -f /dev/null new-session -d -s w -x 230 -y 68
T set-option -g history-limit 100000000
W1=$(T display-message -p -t w: '#{window_id}')
T new-window -t w: -n heavy
T send-keys -t w:heavy 'seq 700000 | awk "{print \$0, \$0*3, \"pad pad pad pad pad\"}"; clear' Enter
T split-window -h -t w:heavy
sleep 12
zpty fake "stty rows 68 cols 230; exec env -u TMUX -u TMUX_PANE tmux -L $L attach -t w \; select-window -t $W1"
sleep 1
CL=$(T list-clients -F '#{client_name} #{client_control_mode}' | awk '$2==0{print $1; exit}')
env -u TMUX -u TMUX_PANE $BIN -L $L ls >/dev/null; sleep 0.5
env -u TMUX -u TMUX_PANE $BIN -L $L toggle $CL; sleep 1
SP=$(T list-panes -a -F '#{pane_id} #{pane_current_command}' | awk '$2 ~ /demux/{print $1; exit}')
LOG=$(ls /tmp/demux-501/${L}-*.sock.log | head -1)
# scrub onto heavy (carve happens here), enter, leave back, re-enter, unpin
T send-keys -t $SP j; sleep 0.8            # billboard heavy -> carve
T send-keys -t $SP Enter; sleep 1          # enter heavy      -> swap
T send-keys -t $SP k; sleep 0.5            # billboard w1
T send-keys -t $SP Enter; sleep 1          # enter w1         -> swap
T send-keys -t $SP j; sleep 0.5
T send-keys -t $SP Enter; sleep 1          # re-enter heavy   -> swap
env -u TMUX -u TMUX_PANE $BIN -L $L toggle $CL; sleep 2  # unpin -> release (may be slow, invisible)
echo "=== timings ==="
grep -E 'took|slow rt|carve' $LOG
T kill-server 2>/dev/null; zpty -d fake 2>/dev/null
