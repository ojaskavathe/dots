#!/usr/bin/env zsh
# demux test infra: build demuxd from source, run the rigs against isolated
# tmux servers (each rig picks its own -L socket; nothing touches the default
# server). Usage:
#   zsh rigs/run.zsh              # build + full assert suite
#   zsh rigs/run.zsh <rig-name>   # build + one rig (e.g. rig-enter)
set -eu
cd ${0:a:h}
BIN=${TMPDIR:-/tmp}/demuxd-rig
(cd ../demuxd && go build -o $BIN .)
echo "built $BIN"
if (( $# )); then
  zsh ./$1.zsh $BIN
else
  zsh ./rig-pin-full.zsh $BIN
fi
