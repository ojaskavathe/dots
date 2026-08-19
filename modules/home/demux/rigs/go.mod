// Integration rigs for demux. A separate module on purpose: demuxd's own
// build/vet/test (and its nix checkPhase) must never pull these in — they
// need a live tmux server and real ptys, which only exist on a dev machine.
module demuxrigs

go 1.22
