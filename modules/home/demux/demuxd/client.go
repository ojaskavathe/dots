package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// dialEnsure connects to the daemon, lazily starting one if the socket is
// dead. The retry loop is a startup handshake, not a poll: it ends the moment
// the daemon binds, or after 2s with the daemon's error.
func dialEnsure(tmuxSock, demuxSock string) (net.Conn, error) {
	if conn, err := net.DialTimeout("unix", demuxSock, 250*time.Millisecond); err == nil {
		return conn, nil
	}

	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(self, "-S", tmuxSock, "run")
	logPath := demuxSock + ".log"
	_ = os.MkdirAll(filepath.Dir(demuxSock), 0o700)
	if logf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
		cmd.Stdout = logf
		cmd.Stderr = logf
		defer logf.Close()
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start daemon: %v", err)
	}
	go cmd.Wait() // reap if it dies while we retry

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if conn, err := net.DialTimeout("unix", demuxSock, 250*time.Millisecond); err == nil {
			return conn, nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return nil, fmt.Errorf("daemon did not come up (see %s)", logPath)
}

func cmdLs(tmuxSock, demuxSock string) {
	conn, err := dialEnsure(tmuxSock, demuxSock)
	if err != nil {
		fmt.Fprintf(os.Stderr, "demuxd ls: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	if !sc.Scan() {
		fmt.Fprintln(os.Stderr, "demuxd ls: no snapshot from daemon")
		os.Exit(1)
	}
	var snap snapshotMsg
	if err := json.Unmarshal(sc.Bytes(), &snap); err != nil || snap.Type != "snapshot" {
		fmt.Fprintf(os.Stderr, "demuxd ls: bad snapshot: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(snap.world.String())
}

// staleBindWarning detects the M-s bind running a different nix build than
// the installed one: binds bake the store path in, and a tmux server that
// hasn't re-sourced its config keeps executing the old binary forever —
// while every fix ships into the new one. run-shell displays this output.
func staleBindWarning() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	if !strings.HasPrefix(exe, "/nix/store/") {
		return "" // dev binary; nothing to compare against
	}
	prof, err := filepath.EvalSymlinks(os.Getenv("HOME") + "/.nix-profile/bin/demuxd")
	if err != nil || prof == exe {
		return ""
	}
	return fmt.Sprintf("demux: M-s runs a STALE build\n  bind:      %s\n  installed: %s\n  fix: tmux source-file ~/.config/tmux/tmux.conf ; pkill demuxd", exe, prof)
}

// sendCmd is every bind entrypoint (toggle, nav, browse): one short-lived
// connection, one cmd, wait for the daemon's reply so bind errors surface in
// tmux via run-shell output.
func sendCmd(tmuxSock, demuxSock string, m cmdMsg) {
	m.Type = "cmd"
	conn, err := dialEnsure(tmuxSock, demuxSock)
	if err != nil {
		fmt.Fprintf(os.Stderr, "demuxd %s: %v\n", m.Cmd, err)
		os.Exit(1)
	}
	defer conn.Close()
	b, _ := json.Marshal(m)
	if _, err := conn.Write(append(b, '\n')); err != nil {
		fmt.Fprintf(os.Stderr, "demuxd %s: %v\n", m.Cmd, err)
		os.Exit(1)
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for sc.Scan() {
		var r wireMsg
		if json.Unmarshal(sc.Bytes(), &r) != nil || r.Type != "reply" {
			continue // snapshot/diff lines on the same conn
		}
		if r.OK != nil && !*r.OK {
			fmt.Fprintf(os.Stderr, "demuxd %s: %s\n", m.Cmd, r.Err)
			os.Exit(1)
		}
		return
	}
	fmt.Fprintf(os.Stderr, "demuxd %s: no reply from daemon\n", m.Cmd)
	os.Exit(1)
}

// cmdToggle is the M-s entrypoint.
func cmdToggle(tmuxSock, demuxSock, client string) {
	if w := staleBindWarning(); w != "" {
		fmt.Println(w)
	}
	sendCmd(tmuxSock, demuxSock, cmdMsg{Cmd: "toggle", Client: client})
}

// cmdNav is the routed M-h / M-l while docked: previous/next window with the
// sidebar riding along in one atomic server sequence.
func cmdNav(tmuxSock, demuxSock, dir, client string) {
	sendCmd(tmuxSock, demuxSock, cmdMsg{Cmd: "nav", Dir: dir, Client: client})
}

// cmdBrowse docks the sidebar and zooms straight into billboard scrubbing.
func cmdBrowse(tmuxSock, demuxSock, client string) {
	sendCmd(tmuxSock, demuxSock, cmdMsg{Cmd: "browse", Client: client})
}

func cmdEvents(tmuxSock, demuxSock string) {
	conn, err := dialEnsure(tmuxSock, demuxSock)
	if err != nil {
		fmt.Fprintf(os.Stderr, "demuxd events: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	out := bufio.NewWriter(os.Stdout)
	for sc.Scan() {
		out.Write(sc.Bytes())
		out.WriteByte('\n')
		out.Flush()
	}
}
