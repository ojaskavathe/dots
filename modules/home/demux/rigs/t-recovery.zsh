#!/usr/bin/env zsh
# failure recovery: hosting window killed, user splits while pinned,
# daemon restart sweeping leaked spacers (restart LAST: its reattach gap
# would flake anything racing it)
source ${0:a:h}/lib.zsh
rig_up

echo "== I: kill hosting window recovers =="
D toggle $CL; sleep 0.6
T kill-window -t $(clientwin)
sleep 0.8
chk "pin state cleaned"           '[[ -z $(T show-options -t play -v @demux_pinned 2>/dev/null) && -z $(T show-options -t work -v @demux_pinned 2>/dev/null) ]]'
D toggle $CL && ok "re-toggle works" || bad "re-toggle failed"
sleep 0.8
S=$(side)
chk "sidebar re-docked"           '[[ $(echo $S | awk "{print \$4}") == 0 && $(echo $S | awk "{print \$5}") == 40 ]]'
D toggle $CL; sleep 0.4

echo "== J: user splits while pinned; undock restore fails cleanly =="
D toggle $CL; sleep 0.6
CUR=$(clientwin)
NP0=$(T list-panes -t $CUR -F '#{pane_current_command}' | grep -cv demux)
T split-window -v -t $CUR
sleep 0.3
D toggle $CL; sleep 0.6
chk "user split survives undock"  '[[ $(T list-panes -t $CUR -F x | wc -l | tr -d " ") == $((NP0+1)) ]]'
chk "no sidebar left behind"      '[[ $(T list-panes -t $CUR -F "#{pane_current_command}" | grep -c demux) == 0 ]]'
chk "stale restore logged"        'grep -qE "restore layout|pin undock" ${SOCK}.log'
T kill-pane -t $(T list-panes -t $CUR -F '#{pane_id}' | tail -1) 2>/dev/null

echo "== S: daemon restart sweeps leaked spacers =="
T split-window -d -hb -f -l 40 -t $W3 'sleep 100000001'   # fake a leak
pkill -f "${BIN:t} -S /private/tmp/tmux-501/$L run"; sleep 0.5
D ls >/dev/null; sleep 2
chk "leaked spacer swept"         '[[ $(spacers) == 0 ]]'
chk "gamma layout intact"         '[[ $(T display-message -p -t $W3 "#{window_layout}" | cut -d, -f2-) == ${L_W3#*,} ]]'
chk "daemon alive"                'pgrep -f "${BIN:t} -S /private/tmp/tmux-501/$L run" >/dev/null'

rig_done
