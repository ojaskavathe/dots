#!/usr/bin/env zsh
# routed nav, scrub focus handoff, unrouted-switch follow, toggle-off
# restore, q mid-scrub
source ${0:a:h}/lib.zsh
rig_up

D toggle $CL; sleep 0.8   # pin on beta

echo "== D: nav next (routed M-l, real switch) =="
NAVWIN=$(clientwin)
D nav next $CL || bad "nav exit"
sleep 0.5
S=$(side)
chk "nav changed window"          '[[ $(clientwin) != $NAVWIN ]]'
chk "sidebar rode along"          '[[ $(echo $S | awk "{print \$3}") == $(clientwin) ]]'
chk "nav focuses MAIN pane"       '[[ $(echo $S | awk "{print \$6}") == 0 ]]'

echo "== E: scrub grabs focus; Enter hands it back =="
SP=$(side | awk '{print $1}')
T select-pane -t $SP
# k, not j: nav landed on gamma, the last list row — j has nowhere to go
T send-keys -t $SP k
sleep 0.5
S=$(side)
chk "scrub zooms sidebar"         '[[ $(echo $S | awk "{print \$5}") == 200 ]]'
chk "scrub keeps sidebar focus"   '[[ $(echo $S | awk "{print \$6}") == 1 ]]'
T send-keys -t $SP Enter
sleep 0.6
S=$(side)
chk "enter docks at 40"           '[[ $(echo $S | awk "{print \$5}") == 40 ]]'
chk "enter focuses main"          '[[ $(echo $S | awk "{print \$6}") == 0 ]]'

echo "== F: unrouted switch -> follow =="
T switch-client -c $CL -t work \; select-window -t $W3
wait_until '[[ $(side | awk "{print \$3}") == $W3 ]]'
S=$(side)
chk "follow docked into gamma"    '[[ $(echo $S | awk "{print \$3}") == $W3 ]]'
chk "follow focuses main"         '[[ $(echo $S | awk "{print \$6}") == 0 ]]'

echo "== G: toggle off (stay, restore) =="
D toggle $CL || bad "toggle-off exit"
sleep 0.6
chk "sidebar left user windows"   '[[ $(T list-panes -t $W3 -F "#{pane_current_command}" | grep -c demux) == 0 ]]'
chk "TUI home in _demux"          '[[ $(T list-panes -t _demux -s -F "#{pane_current_command}" | grep -c demux) -ge 1 ]]'
chk "gamma layout exact"          '[[ $(T display-message -p -t $W3 "#{window_layout}" | cut -d, -f2-) == ${L_W3#*,} ]]'
chk "client still on gamma"       '[[ $(clientwin) == $W3 ]]'
chk "@demux_pinned off work"      '[[ -z $(T show-options -t work -v @demux_pinned 2>/dev/null) ]]'
chk "work status-left restored"   '[[ -z $(T show-options -t work status-left) ]]'
chk "no spacers remain"           '[[ $(spacers) == 0 ]]'

echo "== H: q mid-scrub unzooms in place, still pinned =="
D toggle $CL; sleep 0.6
SP=$(side | awk '{print $1}')
T send-keys -t $SP k k
sleep 0.5
chk "scrubbing (zoomed)"          '[[ $(side | awk "{print \$5}") == 200 ]]'
T send-keys -t $SP q
sleep 0.5
S=$(side)
chk "q never moved the client"    '[[ $(clientwin) == $W3 ]]'
chk "q unzoomed, still docked"    '[[ $(echo $S | awk "{print \$3}") == $W3 && $(echo $S | awk "{print \$5}") == 40 ]]'
chk "gamma unzoomed"              '[[ $(zoomflag $W3) == 0 ]]'

rig_done
