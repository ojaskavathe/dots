#!/usr/bin/env zsh
# tmux-equalize-nvim coexistence: equalize while pinned, geometry-free
# leave, proportional give-back at release and at undock
source ${0:a:h}/lib.zsh
EQ=${EQUALIZE:?export EQUALIZE=<tmux-equalize-nvim binary> (run.zsh does)}
rig_up

RIGTMUX="/private/tmp/tmux-501/$L,1,0"
mains() { T list-panes -t $W1 -F '#{pane_id} #{pane_left} #{pane_width} #{pane_current_command}' | grep -v demux }
M0=$(mains | awk 'NR==1{print $1}')
M1=$(mains | awk 'NR==2{print $1}')

echo "== M: equalize while pinned (main region only) =="
D toggle $CL; sleep 0.6
T select-window -t $W1
wait_until '[[ $(side | awk "{print \$3}") == $W1 ]]'
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
wait_until '[[ $(clientwin) != $W1 ]]'
sleep 0.4
# leaving is geometry-free: the equalized mains DON'T move, the spacer holds
# the slot, dirty survives until release
w0=$(mains | awk -v m=$M0 '$1==m{print $3}'); w1=$(mains | awk -v m=$M1 '$1==m{print $3}')
chk "leave keeps equalized mains" '[[ $w0 == $we0 && $w1 == $we1 ]]'
chk "spacer holds w1 slot"        '[[ $(T list-panes -t $W1 -F "#{pane_left} #{pane_width}" | awk "\$1==0&&\$2==40" | wc -l | tr -d " ") == 1 ]]'
chk "dirty survives leave"        '[[ $(T show-options -wv -t $W1 @demux_layout_dirty 2>/dev/null) == 1 ]]'
D toggle $CL; sleep 0.6   # unpin: release w1
w0=$(mains | awk -v m=$M0 '$1==m{print $3}'); w1=$(mains | awk -v m=$M1 '$1==m{print $3}')
chk "give-back full width"        '[[ $(( w0 + w1 )) == 199 ]]'
chk "give-back proportional"      '[[ $(( w0 > w1 ? w0-w1 : w1-w0 )) -le 1 ]]'
chk "dirty marker cleared"        '[[ -z $(T show-options -wv -t $W1 @demux_layout_dirty 2>/dev/null) ]]'
chk "release swept w1 spacer"     '[[ $(T list-panes -t $W1 -F "#{pane_start_command}" | grep -c "sleep 100000001") == 0 ]]'
D toggle $CL; sleep 0.6   # re-pin for O

echo "== O: equalize then toggle-off give-back =="
T select-window -t $W1
wait_until '[[ $(side | awk "{print \$3}") == $W1 ]]'
T resize-pane -t $M0 -x 50; sleep 0.3
TMUX=$RIGTMUX TMUX_PANE=$M0 $EQ || bad "equalize exit"
sleep 0.5
D toggle $CL; sleep 0.6
w0=$(mains | awk -v m=$M0 '$1==m{print $3}'); w1=$(mains | awk -v m=$M1 '$1==m{print $3}')
chk "undock give-back full width" '[[ $(( w0 + w1 )) == 199 ]]'
chk "undock give-back equal"      '[[ $(( w0 > w1 ? w0-w1 : w1-w0 )) -le 1 ]]'
chk "no sidebar in w1"            '[[ $(T list-panes -t $W1 -F "#{pane_current_command}" | grep -c demux) == 0 ]]'
chk "dirty cleared on undock"     '[[ -z $(T show-options -wv -t $W1 @demux_layout_dirty 2>/dev/null) ]]'

rig_done
