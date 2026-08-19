#!/usr/bin/env zsh
# dock, billboard scrub, enter exactness, escape recovery, M-s dismiss
source ${0:a:h}/lib.zsh
rig_up

echo "== A: toggle on (dock into beta) =="
D toggle $CL || bad "toggle exit"
sleep 0.8
S=$(side)
chk "sidebar pane exists"        '[[ -n "$S" ]]'
chk "sidebar in beta"            '[[ $(echo $S | awk "{print \$3}") == $W2 ]]'
chk "sidebar at left edge"       '[[ $(echo $S | awk "{print \$4}") == 0 ]]'
chk "sidebar 40 cols"            '[[ $(echo $S | awk "{print \$5}") == 40 ]]'
chk "sidebar focused"            '[[ $(echo $S | awk "{print \$6}") == 1 ]]'
chk "@demux_pinned on work"      '[[ $(T show-options -t work -v @demux_pinned 2>/dev/null) == 1 ]]'
SL=$(T show-options -t work status-left)
chk "status-left padded 41"      '[[ ${#SL} -ge 45 ]]'
chk "_demux alive"               'T has-session -t _demux 2>/dev/null'
CAP=$(T capture-pane -p -t $(echo $S | awk '{print $1}'))
chk "narrow list shows sessions" '[[ $CAP == *work* && $CAP == *play* ]]'
chk "narrow list has no border"  '[[ $CAP != *│* ]]'
SP=$(echo $S | awk '{print $1}')
BETA_MAIN=$(T list-panes -t $W2 -F '#{pane_id} #{pane_width}' | grep -v "^$SP" | head -1)

echo "== B: scrub k -> billboard (zoom, nothing real moves) =="
t0=$(now_ms)
T send-keys -t $SP k
lat=TIMEOUT
for i in {1..200}; do
  if T capture-pane -p -t $SP 2>/dev/null | grep -q MARKW1; then lat=$(( $(now_ms) - t0 )); break; fi
  sleep 0.01
done
chk "billboard shows w1 content" '[[ $lat != TIMEOUT ]]'
echo "  first-billboard latency: ${lat}ms"
chk "client window UNCHANGED"    '[[ $(clientwin) == $W2 ]]'
chk "beta zoomed"                '[[ $(zoomflag $W2) == 1 ]]'
S=$(side)
chk "sidebar full width (zoom)"  '[[ $(echo $S | awk "{print \$5}") == 200 ]]'
chk "hidden main kept size"      '[[ "$(T list-panes -t $W2 -F "#{pane_id} #{pane_width}" | grep -v "^$SP" | head -1)" == "$BETA_MAIN" ]]'
chk "wide list border painted"   '[[ $(T capture-pane -p -t $SP) == *│* ]]'
# w1 is a ~100/99 split; the marker lives in the RIGHT pane. Cropped (old
# behavior) it starts at col ~142; scaled to the 159-col canvas, col ~122.
POS=$(T capture-pane -p -t $SP | awk '{i=index($0,"MARKW1"); if(i){print i; exit}}')
chk "split scaled to canvas"     '[[ -n "$POS" && $POS -lt 135 ]]'

echo "== B2: Enter -> commits for real =="
T send-keys -t $SP Enter
wait_until '[[ $(clientwin) == $W1 ]]'
sleep 0.4
S=$(side)
chk "client now on w1"           '[[ $(clientwin) == $W1 ]]'
chk "sidebar docked in w1"       '[[ $(echo $S | awk "{print \$3}") == $W1 && $(echo $S | awk "{print \$5}") == 40 ]]'
chk "zoom cleared"               '[[ $(zoomflag $W1) == 0 && $(zoomflag $W2) == 0 ]]'
# leaving is geometry-free: beta keeps its docked shape, a spacer holding
# the sidebar's slot (byte-exact restore happens at release, checked in C)
chk "beta holds spacer at left"  '[[ $(T list-panes -t $W2 -F "#{pane_left} #{pane_width}" | awk "\$1==0&&\$2==40" | wc -l | tr -d " ") == 1 ]]'
chk "commit focuses main"        '[[ $(echo $S | awk "{print \$6}") == 0 ]]'
# billboard EXACTNESS: the marker column on the billboard must equal the
# marker pane's real on-screen column after entering (pane_left is 0-based)
MREAL=$(( $(T list-panes -t $W1 -F '#{pane_id} #{pane_left}' | awk '$2>41{print $2}' | head -1) + 1 ))
echo "  billboard col=$POS real col=$MREAL"
chk "billboard == docked reality" '[[ -n "$POS" && $(( POS > MREAL ? POS-MREAL : MREAL-POS )) -le 1 ]]'

echo "== P: escaping a billboard (C-l) recovers =="
T select-pane -t $SP
T send-keys -t $SP j
sleep 0.5
chk "scrub zoomed"                '[[ $(zoomflag $W1) == 1 ]]'
MAIN=$(T list-panes -t $W1 -F '#{pane_id}' | grep -v "^$SP" | head -1)
T select-pane -t $MAIN
sleep 0.5
chk "escape unzoomed (tmux)"      '[[ $(zoomflag $W1) == 0 ]]'
chk "daemon ended scrub"          'grep -q "scrub unzoomed externally" ${SOCK}.log'
T select-pane -t $SP
T send-keys -t $SP j
sleep 0.5
chk "j scrubs again after escape" '[[ $(zoomflag $W1) == 1 ]]'
T send-keys -t $SP q
sleep 0.5
chk "q settles back"              '[[ $(zoomflag $W1) == 0 && $(clientwin) == $W1 ]]'

echo "== C: storm kkkk + M-s commits AND dismisses =="
T select-pane -t $SP
T send-keys -t $SP k k k k
sleep 0.6
chk "storm stays put (billboard)" '[[ $(clientwin) == $W1 ]]'
D toggle $CL || bad "toggle exit"
sleep 0.8
chk "M-s landed on play"          '[[ $(clientsess) == play ]]'
chk "sidebar dismissed"           '[[ $(T list-panes -s -t work -F "#{pane_current_command}" | grep -c demux) == 0 && $(T list-panes -s -t play -F "#{pane_current_command}" | grep -c demux) == 0 ]]'
chk "w1 layout exact after unpin" '[[ $(T display-message -p -t $W1 "#{window_layout}" | cut -d, -f2-) == ${L_W1#*,} ]]'
chk "beta layout exact after unpin" '[[ $(T display-message -p -t $W2 "#{window_layout}" | cut -d, -f2-) == ${L_W2#*,} ]]'
chk "no spacers remain"           '[[ $(spacers) == 0 ]]'
chk "@demux_pinned cleared"       '[[ -z $(T show-options -t play -v @demux_pinned 2>/dev/null) && -z $(T show-options -t work -v @demux_pinned 2>/dev/null) ]]'
chk "status unpadded everywhere"  '[[ -z $(T show-options -t work status-left) && -z $(T show-options -t play status-left) ]]'

rig_done
