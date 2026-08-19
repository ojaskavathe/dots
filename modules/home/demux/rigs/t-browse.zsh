#!/usr/bin/env zsh
# full-screen browse surface, and browse-from-pinned auto-undock
source ${0:a:h}/lib.zsh
rig_up

echo "== K: full-screen browse =="
D browse $CL; sleep 0.8
chk "client on _demux"            '[[ $(clientsess) == _demux ]]'
TP=$(T list-panes -t _demux -s -F '#{pane_id} #{pane_current_command} #{pane_width}' | awk '/demux/{print}')
chk "tui full width"              '[[ $(echo $TP | awk "{print \$3}") == 200 ]]'
chk "wide mode has border"        '[[ $(T capture-pane -p -t $(echo $TP | awk "{print \$1}")) == *│* ]]'
T send-keys -t $(echo $TP | awk '{print $1}') q; sleep 0.6
chk "q leaves browse"             '[[ $(clientsess) != _demux ]]'

echo "== L: browse from pinned auto-undocks =="
D toggle $CL; sleep 0.5
PINWIN=$(clientwin)
D browse $CL; sleep 0.8
chk "browse took over"            '[[ $(clientsess) == _demux ]]'
chk "pin auto-undocked"           '[[ $(T list-panes -t $PINWIN -F "#{pane_current_command}" | grep -c demux) == 0 ]]'
chk "no spacers remain"           '[[ $(spacers) == 0 ]]'
TP2=$(T list-panes -t _demux -s -F '#{pane_id} #{pane_current_command}' | awk '/demux/{print $1}')
T send-keys -t $TP2 q; sleep 0.6
chk "q from browse returns"       '[[ $(clientsess) != _demux ]]'

rig_done
