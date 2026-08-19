#!/usr/bin/env zsh
# pinned-mode rig: dock / billboard-scrub / commit / nav / follow / close.
# usage: rig-pin-full.zsh <demuxd-binary>
set -u
zmodload zsh/datetime
zmodload zsh/zpty

BIN=$1
L=rigpinf
T() { env -u TMUX -u TMUX_PANE tmux -L $L "$@" }
now_ms() { printf '%.0f' $((EPOCHREALTIME * 1000)) }

PASS=0; FAIL=0
ok()   { PASS=$((PASS+1)); echo "  ok  - $1" }
bad()  { FAIL=$((FAIL+1)); echo "  FAIL- $1" }
chk()  { if eval "$2"; then ok "$1"; else bad "$1 [[ $2 ]]"; fi }

# --- clean slate ---------------------------------------------------------
T kill-server 2>/dev/null
pkill -f "${BIN:t} -S /private/tmp/tmux-501/$L" 2>/dev/null
rm -f /tmp/demux-501/${L}-*(N) 2>/dev/null
sleep 0.4

# --- world: work (w1 split, w2 beta, w3 gamma), play (2 windows) ---------
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

# --- fake client 200x50 on work:beta -------------------------------------
zpty fake "stty rows 50 cols 200; exec env -u TMUX -u TMUX_PANE tmux -L $L attach -t work"
sleep 1
# baselines AFTER attach, each while ITS window is current — the client's
# status line resizes only the current window (window-size latest)
T select-window -t $W1; sleep 0.2
L_W1=$(T display-message -p -t $W1 '#{window_layout}')
T select-window -t $W3; sleep 0.2
L_W3=$(T display-message -p -t $W3 '#{window_layout}')
T select-window -t $W2; sleep 0.2
L_W2=$(T display-message -p -t $W2 '#{window_layout}')
CL=$(T list-clients -F '#{client_name} #{client_control_mode}' | awk '$2==0{print $1; exit}')
[[ -n $CL ]] || { echo "no fake client"; exit 1 }

env -u TMUX -u TMUX_PANE $BIN -L $L ls >/dev/null
sleep 0.5
SOCK=$(env -u TMUX -u TMUX_PANE $BIN -L $L sock | awk '/demux:/{print $2}')

side() { T list-panes -a -F '#{pane_id} #{pane_current_command} #{window_id} #{pane_left} #{pane_width} #{pane_active}' | awk '$2 ~ /demux/ {print; exit}' }
clientwin() { T list-clients -F '#{client_name} #{window_id}' | awk -v c=$CL '$1==c{print $2}' }
clientsess() { T list-clients -F '#{client_name} #{session_name}' | awk -v c=$CL '$1==c{print $2}' }
zoomflag() { T display-message -p -t $1 '#{window_zoomed_flag}' }

echo "== A: toggle on (dock into beta) =="
env -u TMUX -u TMUX_PANE $BIN -L $L toggle $CL || bad "toggle exit"
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
for i in {1..100}; do [[ $(clientwin) == $W1 ]] && break; sleep 0.01; done
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
env -u TMUX -u TMUX_PANE $BIN -L $L toggle $CL || bad "toggle exit"
sleep 0.8
chk "M-s landed on play"          '[[ $(clientsess) == play ]]'
chk "sidebar dismissed"           '[[ $(T list-panes -s -t work -F "#{pane_current_command}" | grep -c demux) == 0 && $(T list-panes -s -t play -F "#{pane_current_command}" | grep -c demux) == 0 ]]'
chk "w1 layout exact after unpin" '[[ $(T display-message -p -t $W1 "#{window_layout}" | cut -d, -f2-) == ${L_W1#*,} ]]'
chk "beta layout exact after unpin" '[[ $(T display-message -p -t $W2 "#{window_layout}" | cut -d, -f2-) == ${L_W2#*,} ]]'
chk "no spacers remain"           '[[ $(T list-panes -a -F "#{pane_start_command}" | grep -c "sleep 100000001") == 0 ]]'
chk "@demux_pinned cleared"       '[[ -z $(T show-options -t play -v @demux_pinned 2>/dev/null) && -z $(T show-options -t work -v @demux_pinned 2>/dev/null) ]]'
chk "status unpadded everywhere"  '[[ -z $(T show-options -t work status-left) && -z $(T show-options -t play status-left) ]]'
env -u TMUX -u TMUX_PANE $BIN -L $L toggle $CL; sleep 0.8   # re-pin for D
chk "re-pinned on play"           '[[ $(side | awk "{print \$3}") == $(clientwin) ]]'

echo "== D: nav next (routed M-l, real switch) =="
NAVWIN=$(clientwin)
env -u TMUX -u TMUX_PANE $BIN -L $L nav next $CL || bad "nav exit"
sleep 0.5
S=$(side)
chk "nav changed window"          '[[ $(clientwin) != $NAVWIN ]]'
chk "sidebar rode along"          '[[ $(echo $S | awk "{print \$3}") == $(clientwin) ]]'
chk "nav focuses MAIN pane"       '[[ $(echo $S | awk "{print \$6}") == 0 ]]'

echo "== E: scrub grabs focus; Enter hands it back =="
SP=$(side | awk '{print $1}')
T select-pane -t $SP
T send-keys -t $SP j
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
for i in {1..100}; do [[ $(side | awk '{print $3}') == $W3 ]] && break; sleep 0.01; done
S=$(side)
chk "follow docked into gamma"    '[[ $(echo $S | awk "{print \$3}") == $W3 ]]'
chk "follow focuses main"         '[[ $(echo $S | awk "{print \$6}") == 0 ]]'

echo "== G: toggle off (stay, restore) =="
env -u TMUX -u TMUX_PANE $BIN -L $L toggle $CL || bad "toggle-off exit"
sleep 0.6
chk "sidebar left user windows"   '[[ $(T list-panes -t $W3 -F "#{pane_current_command}" | grep -c demux) == 0 ]]'
chk "TUI home in _demux"          '[[ $(T list-panes -t _demux -s -F "#{pane_current_command}" | grep -c demux) -ge 1 ]]'
chk "gamma layout exact"          '[[ $(T display-message -p -t $W3 "#{window_layout}" | cut -d, -f2-) == ${L_W3#*,} ]]'
chk "client still on gamma"       '[[ $(clientwin) == $W3 ]]'
chk "@demux_pinned off work"      '[[ -z $(T show-options -t work -v @demux_pinned 2>/dev/null) ]]'
chk "work status-left restored"   '[[ -z $(T show-options -t work status-left) ]]'

echo "== H: q mid-scrub unzooms in place, still pinned =="
env -u TMUX -u TMUX_PANE $BIN -L $L toggle $CL; sleep 0.6
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
env -u TMUX -u TMUX_PANE $BIN -L $L toggle $CL; sleep 0.5   # unpin for I

echo "== I: kill hosting window recovers =="
env -u TMUX -u TMUX_PANE $BIN -L $L toggle $CL; sleep 0.6
T kill-window -t $(clientwin)
sleep 0.8
chk "pin state cleaned"           '[[ -z $(T show-options -t play -v @demux_pinned 2>/dev/null) && -z $(T show-options -t work -v @demux_pinned 2>/dev/null) ]]'
env -u TMUX -u TMUX_PANE $BIN -L $L toggle $CL && ok "re-toggle works" || bad "re-toggle failed"
sleep 0.8
S=$(side)
chk "sidebar re-docked"           '[[ $(echo $S | awk "{print \$4}") == 0 && $(echo $S | awk "{print \$5}") == 40 ]]'
env -u TMUX -u TMUX_PANE $BIN -L $L toggle $CL; sleep 0.4

EQ=${EQUALIZE:-${0:A:h}/equalize-pin}
RIGTMUX="/private/tmp/tmux-501/$L,1,0"
mains() { T list-panes -t $W1 -F '#{pane_id} #{pane_left} #{pane_width} #{pane_current_command}' | grep -v demux }
M0=$(mains | awk 'NR==1{print $1}')
M1=$(mains | awk 'NR==2{print $1}')

echo "== M: equalize while pinned (main region only) =="
env -u TMUX -u TMUX_PANE $BIN -L $L toggle $CL; sleep 0.6
T switch-client -c $CL -t work \; select-window -t $W1
for i in {1..100}; do [[ $(side | awk '{print $3}') == $W1 ]] && break; sleep 0.01; done
chk "docked in w1"                '[[ $(side | awk "{print \$3}") == $W1 ]]'
T resize-pane -t $M0 -x 50; sleep 0.3
chk "mains unequal before"        '[[ $(mains | awk -v m=$M0 "\$1==m{print \$3}") == 50 ]]'
TMUX=$RIGTMUX TMUX_PANE=$M0 $EQ || bad "equalize exit"
sleep 0.5
S=$(side)
w0=$(mains | awk -v m=$M0 '$1==m{print $3}'); w1=$(mains | awk -v m=$M1 '$1==m{print $3}')
chk "sidebar untouched at 40"     '[[ $(echo $S | awk "{print \$4}") == 0 && $(echo $S | awk "{print \$5}") == 40 ]]'
chk "mains equalized"             '[[ $(( w0 > w1 ? w0-w1 : w1-w0 )) -le 1 ]]'
chk "pane order preserved"        '[[ $(mains | awk -v m=$M0 "\$1==m{print \$2}") -lt $(mains | awk -v m=$M1 "\$1==m{print \$2}") ]]'
chk "dirty marker set"            '[[ $(T show-options -wv -t $W1 @demux_layout_dirty 2>/dev/null) == 1 ]]'

echo "== N: commit elsewhere keeps geometry; unpin gives back =="
we0=$(mains | awk -v m=$M0 '$1==m{print $3}'); we1=$(mains | awk -v m=$M1 '$1==m{print $3}')
SP=$(side | awk '{print $1}')
T select-pane -t $SP
T send-keys -t $SP j
sleep 0.4
T send-keys -t $SP Enter
for i in {1..100}; do [[ $(clientwin) != $W1 ]] && break; sleep 0.01; done
sleep 0.4
# leaving is geometry-free: the equalized mains DON'T move, the spacer holds
# the slot, dirty survives until release
w0=$(mains | awk -v m=$M0 '$1==m{print $3}'); w1=$(mains | awk -v m=$M1 '$1==m{print $3}')
chk "leave keeps equalized mains" '[[ $w0 == $we0 && $w1 == $we1 ]]'
chk "spacer holds w1 slot"        '[[ $(T list-panes -t $W1 -F "#{pane_left} #{pane_width}" | awk "\$1==0&&\$2==40" | wc -l | tr -d " ") == 1 ]]'
chk "dirty survives leave"        '[[ $(T show-options -wv -t $W1 @demux_layout_dirty 2>/dev/null) == 1 ]]'
env -u TMUX -u TMUX_PANE $BIN -L $L toggle $CL; sleep 0.6   # unpin: release w1
w0=$(mains | awk -v m=$M0 '$1==m{print $3}'); w1=$(mains | awk -v m=$M1 '$1==m{print $3}')
chk "give-back full width"        '[[ $(( w0 + w1 )) == 199 ]]'
chk "give-back proportional"      '[[ $(( w0 > w1 ? w0-w1 : w1-w0 )) -le 1 ]]'
chk "dirty marker cleared"        '[[ -z $(T show-options -wv -t $W1 @demux_layout_dirty 2>/dev/null) ]]'
chk "release swept w1 spacer"     '[[ $(T list-panes -t $W1 -F "#{pane_start_command}" | grep -c "sleep 100000001") == 0 ]]'
env -u TMUX -u TMUX_PANE $BIN -L $L toggle $CL; sleep 0.6   # re-pin for O

echo "== O: equalize then toggle-off give-back =="
T select-window -t $W1
for i in {1..100}; do [[ $(side | awk '{print $3}') == $W1 ]] && break; sleep 0.01; done
T resize-pane -t $M0 -x 50; sleep 0.3
TMUX=$RIGTMUX TMUX_PANE=$M0 $EQ || bad "equalize exit"
sleep 0.5
env -u TMUX -u TMUX_PANE $BIN -L $L toggle $CL; sleep 0.6
w0=$(mains | awk -v m=$M0 '$1==m{print $3}'); w1=$(mains | awk -v m=$M1 '$1==m{print $3}')
chk "undock give-back full width" '[[ $(( w0 + w1 )) == 199 ]]'
chk "undock give-back equal"      '[[ $(( w0 > w1 ? w0-w1 : w1-w0 )) -le 1 ]]'
chk "no sidebar in w1"            '[[ $(T list-panes -t $W1 -F "#{pane_current_command}" | grep -c demux) == 0 ]]'
chk "dirty cleared on undock"     '[[ -z $(T show-options -wv -t $W1 @demux_layout_dirty 2>/dev/null) ]]'

echo "== J: user splits while pinned; undock restore fails cleanly =="
env -u TMUX -u TMUX_PANE $BIN -L $L toggle $CL; sleep 0.6
CUR=$(clientwin)
NP0=$(T list-panes -t $CUR -F '#{pane_current_command}' | grep -cv demux)
T split-window -v -t $CUR
sleep 0.3
env -u TMUX -u TMUX_PANE $BIN -L $L toggle $CL; sleep 0.6
chk "user split survives undock"  '[[ $(T list-panes -t $CUR -F x | wc -l | tr -d " ") == $((NP0+1)) ]]'
chk "no sidebar left behind"      '[[ $(T list-panes -t $CUR -F "#{pane_current_command}" | grep -c demux) == 0 ]]'
chk "stale restore logged"        'grep -qE "restore layout|pin undock" ${SOCK}.log'
T kill-pane -t $(T list-panes -t $CUR -F '#{pane_id}' | tail -1) 2>/dev/null

echo "== K: full-screen browse still works =="
env -u TMUX -u TMUX_PANE $BIN -L $L browse $CL; sleep 0.8
chk "client on _demux"            '[[ $(clientsess) == _demux ]]'
TP=$(T list-panes -t _demux -s -F '#{pane_id} #{pane_current_command} #{pane_width}' | awk '/demux/{print}')
chk "tui full width"              '[[ $(echo $TP | awk "{print \$3}") == 200 ]]'
chk "wide mode has border"        '[[ $(T capture-pane -p -t $(echo $TP | awk "{print \$1}")) == *│* ]]'
T send-keys -t $(echo $TP | awk '{print $1}') q; sleep 0.6
chk "q leaves browse"             '[[ $(clientsess) != _demux ]]'

echo "== L: browse from pinned auto-undocks =="
env -u TMUX -u TMUX_PANE $BIN -L $L toggle $CL; sleep 0.5
PINWIN=$(clientwin)
env -u TMUX -u TMUX_PANE $BIN -L $L browse $CL; sleep 0.8
chk "browse took over"            '[[ $(clientsess) == _demux ]]'
chk "pin auto-undocked"           '[[ $(T list-panes -t $PINWIN -F "#{pane_current_command}" | grep -c demux) == 0 ]]'
TP2=$(T list-panes -t _demux -s -F '#{pane_id} #{pane_current_command}' | awk '/demux/{print $1}')
T send-keys -t $TP2 q; sleep 0.6
chk "q from browse returns"       '[[ $(clientsess) != _demux ]]'

# LAST behavioral section: the daemon restart leaves a reattach gap that
# would flake any section racing it.
echo "== S: daemon restart sweeps leaked spacers =="
T split-window -d -hb -f -l 40 -t $W2 'sleep 100000001'   # fake a leak
pkill -f "${BIN:t} -S /private/tmp/tmux-501/$L run"; sleep 0.5
env -u TMUX -u TMUX_PANE $BIN -L $L ls >/dev/null; sleep 2
chk "leaked spacer swept"         '[[ $(T list-panes -a -F "#{pane_start_command}" | grep -c "sleep 100000001") == 0 ]]'
chk "beta layout intact"          '[[ $(T display-message -p -t $W2 "#{window_layout}" | cut -d, -f2-) == ${L_W2#*,} ]]'

echo "== daemon health =="
chk "daemon alive"                'pgrep -f "${BIN:t} -S /private/tmp/tmux-501/$L run" >/dev/null'
echo "--- slow cmds / errors in daemon log:"
grep -E "took|error" ${SOCK}.log | tail -5
echo
echo "PASS=$PASS FAIL=$FAIL"
