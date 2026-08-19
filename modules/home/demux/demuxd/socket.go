package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolveTmuxSocket mirrors tmux's own socket resolution so that the daemon
// and its clients agree on server identity no matter where they run:
// -S wins, then -L, then $TMUX's first field, then the default socket.
func resolveTmuxSocket(sPath, lName string) string {
	if sPath != "" {
		return canonical(sPath)
	}
	name := lName
	if name == "" {
		if env := os.Getenv("TMUX"); env != "" {
			if i := strings.Index(env, ","); i > 0 {
				return canonical(env[:i])
			}
		}
		name = "default"
	}
	dir := os.Getenv("TMUX_TMPDIR")
	if dir == "" {
		dir = "/tmp"
	}
	return canonical(filepath.Join(dir, fmt.Sprintf("tmux-%d", os.Getuid()), name))
}

// canonical resolves symlinks (macOS /tmp -> /private/tmp) so the same server
// hashes to the same demux socket whether the path came from $TMUX (already
// resolved) or was constructed here.
func canonical(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

// demuxSocketPath derives this server's demux socket. Kept short: unix socket
// paths cap at ~104 bytes on darwin.
func demuxSocketPath(tmuxSock string) string {
	sum := sha256.Sum256([]byte(tmuxSock))
	dir := filepath.Join("/tmp", fmt.Sprintf("demux-%d", os.Getuid()))
	return filepath.Join(dir, fmt.Sprintf("%s-%x.sock", filepath.Base(tmuxSock), sum[:4]))
}
