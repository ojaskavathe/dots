package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
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

// cmdToggle is the M-s entrypoint: one short-lived connection, one cmd, wait
// for the daemon's reply so bind errors surface in tmux.
func cmdToggle(tmuxSock, demuxSock, client string) {
	conn, err := dialEnsure(tmuxSock, demuxSock)
	if err != nil {
		fmt.Fprintf(os.Stderr, "demuxd toggle: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()
	b, _ := json.Marshal(cmdMsg{Type: "cmd", Cmd: "toggle", Client: client})
	if _, err := conn.Write(append(b, '\n')); err != nil {
		fmt.Fprintf(os.Stderr, "demuxd toggle: %v\n", err)
		os.Exit(1)
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for sc.Scan() {
		var m wireMsg
		if json.Unmarshal(sc.Bytes(), &m) != nil || m.Type != "reply" {
			continue // snapshot/diff lines on the same conn
		}
		if m.OK != nil && !*m.OK {
			fmt.Fprintf(os.Stderr, "demuxd toggle: %s\n", m.Err)
			os.Exit(1)
		}
		return
	}
	fmt.Fprintln(os.Stderr, "demuxd toggle: no reply from daemon")
	os.Exit(1)
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
