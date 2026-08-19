#!/usr/bin/env zsh
set -u
zmodload zsh/datetime
L=rigswap2
T() { env -u TMUX -u TMUX_PANE tmux -L $L "$@" }
now_ms() { printf '%.0f' $((EPOCHREALTIME * 1000)) }
# strip checksum AND pane ids: geometry only
geo() { echo "$1" | sed -E 's/^[0-9a-f]+,//; s/,[0-9]+([,}\]])/\1/g; s/,[0-9]+$//' }
T kill-server 2>/dev/null; sleep 0.3
T -f /dev/null new-session -d -s w -x 230 -y 68
T set-option -g history-limit 100000000
WA=$(T display-message -p -t w: '#{window_id}')
T split-window -h -t $WA; T split-window -v -t $WA

echo "== 1: join carve vs split carve, geometry-only compare =="
BASE_A=$(T display-message -p -t $WA '#{window_layout}')
T new-window -d -t w: -n scratch 'sleep 1000'
T split-window -d -t w:scratch 'sleep 1000'   # keep scratch alive after join
SC=$(T list-panes -t w:scratch -F '#{pane_id}' | head -1)
T join-pane -hb -f -l 40 -s $SC -t $WA
JOIN_L=$(T display-message -p -t $WA '#{window_layout}')
T join-pane -d -s $SC -t w:scratch   # put it back
T select-layout -t $WA "$BASE_A"
T split-window -d -hb -f -l 40 -t $WA 'sleep 1000'
SPLIT_L=$(T display-message -p -t $WA '#{window_layout}')
[[ "$(geo $JOIN_L)" == "$(geo $SPLIT_L)" ]] && echo CARVE-EQUAL || { echo CARVE-DIFF; echo "join:  $(geo $JOIN_L)"; echo "split: $(geo $SPLIT_L)" }

echo "== 2: history reflow cost with big scrollback =="
T new-window -d -t w: -n heavy
T send-keys -t w:heavy 'seq 700000 | awk "{print \$0, \$0*3, \"pad pad pad\"}"; echo DONE' Enter
sleep 10
HS=$(T display-message -p -t w:heavy '#{history_size}')
WH=$(T display-message -p -t w:heavy '#{window_id}')
T set-option -wq -t $WH window-size manual
t0=$(now_ms); T resize-window -t $WH -x 189; t1=$(now_ms)
T resize-window -t $WH -x 230; t2=$(now_ms)
T set-option -wq -u -t $WH window-size
echo "history=$HS resize-shrink=$((t1-t0))ms resize-grow=$((t2-t1))ms"

echo "== 3: spacer ops on the heavy window =="
t0=$(now_ms); T split-window -d -hb -f -l 40 -t $WH 'sleep 1000'; t1=$(now_ms)
SPH=$(T display-message -p -t "$WH.{top-left}" '#{pane_id}')
SPA=$(T display-message -p -t "$WA.{top-left}" '#{pane_id}')
LH_B=$(T display-message -p -t $WH '#{window_layout}')
t2=$(now_ms); T swap-pane -d -s $SPH -t $SPA; t3=$(now_ms)
LH_A=$(T display-message -p -t $WH '#{window_layout}')
t4=$(now_ms); T swap-pane -d -s $SPA -t $SPH; t5=$(now_ms)
echo "carve=$((t1-t0))ms swap-in=$((t3-t2))ms swap-back=$((t5-t4))ms"
[[ "$(geo $LH_B)" == "$(geo $LH_A)" ]] && echo HEAVY-SWAP-GEOMETRY-FREE || echo HEAVY-SWAP-MOVED
echo "== 4: release cost (kill spacer + restore layout, one batch) =="
ORIG=$(geo_dummy=1; echo "$BASE_A")
t0=$(now_ms); T kill-pane -t "$(T display-message -p -t "$WH.{top-left}" '#{pane_id}')"; t1=$(now_ms)
echo "kill-spacer=$((t1-t0))ms"
T kill-server 2>/dev/null
