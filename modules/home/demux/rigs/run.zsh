#!/usr/bin/env zsh
# demux test infra. Builds demuxd + tmux-equalize-nvim from source, then runs
# rigs — each on its own isolated tmux server (socket = test name + pid), so
# the suite runs in PARALLEL and never touches the default server.
#
#   zsh rigs/run.zsh                    # full suite (t-*.zsh) in parallel
#   zsh rigs/run.zsh t-equalize         # one test
#   zsh rigs/run.zsh t-browse rig-heavy # any mix; perf rigs (rig-*) run only
#                                       # when named — their timings are the
#                                       # point, so don't run them under load
set -u
zmodload zsh/datetime
cd ${0:a:h}

BIN=${TMPDIR:-/tmp}/demuxd-rig
(cd ../demuxd && go build -o $BIN .) || exit 1
export EQUALIZE=${TMPDIR:-/tmp}/equalize-rig
(cd ../../tmux-equalize-nvim && go build -o $EQUALIZE .) || exit 1

typeset -a tests
if (( $# )); then
  for t in "$@"; do tests+=(${t%.zsh}) done
else
  tests=(t-*.zsh(N:r))
fi
(( $#tests )) || { echo "no tests found"; exit 1 }

t0=$EPOCHREALTIME
typeset -a pids outs
for t in $tests; do
  [[ -f $t.zsh ]] || { echo "no such test: $t.zsh"; exit 1 }
  o=$(mktemp)
  outs+=($o)
  zsh ./$t.zsh $BIN >$o 2>&1 &
  pids+=($!)
done

fail=0 pass_total=0 fail_total=0
for i in {1..$#tests}; do
  wait $pids[$i]; rc=$?
  (( rc )) && fail=1
  echo "──── $tests[$i] $( (( rc )) && echo '✗ FAIL' || echo '✓' ) ────"
  cat $outs[$i]
  p=$(grep -o 'PASS=[0-9]*' $outs[$i] | tail -1 | cut -d= -f2)
  f=$(grep -o 'FAIL=[0-9]*' $outs[$i] | tail -1 | cut -d= -f2)
  pass_total=$((pass_total + ${p:-0})); fail_total=$((fail_total + ${f:-0}))
  rm -f $outs[$i]
done
printf '\nTOTAL PASS=%d FAIL=%d  (%.1fs)\n' $pass_total $fail_total $((EPOCHREALTIME - t0))
exit $fail
