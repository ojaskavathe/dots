package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// debounce for notification bursts: long enough to coalesce a storm (a
// session teardown emits dozens), short enough to be imperceptible.
const debounce = 15 * time.Millisecond

func runDaemon(tmuxSock, demuxSock string) {
	log.SetPrefix("demuxd: ")
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	h := newHub()
	d := &daemon{tmuxSock: tmuxSock, h: h}
	var ln net.Listener

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	for attempt := 0; ; attempt++ {
		ctl, err := dialControl(tmuxSock)
		if err != nil {
			log.Fatalf("spawn tmux: %v", err)
		}
		w, err := fetchWorld(ctl)
		if err != nil {
			// Attach failed (server gone, or no sessions left). First attempt
			// is a startup error; afterwards it is the normal end of life.
			ctl.close()
			if attempt == 0 {
				log.Fatalf("attach %s: %v", tmuxSock, err)
			}
			log.Printf("tmux server gone (%v), exiting", err)
			h.closeAll()
			return
		}
		// Snapshot, not diff: after a reconnect gap, diffs against the old
		// world could be stale mid-gap; a snapshot is always truthful.
		h.setWorld(w, nil, true, tmuxSock)
		if ln == nil {
			// Bind only now, with a populated world: a subscriber must never
			// see the socket before there is a truthful snapshot behind it
			// (an early `ls` would print an empty world and exit 0).
			if err := os.MkdirAll(filepath.Dir(demuxSock), 0o700); err != nil {
				ctl.close()
				log.Fatalf("socket dir: %v", err)
			}
			ln, err = listenExclusive(demuxSock)
			if err != nil {
				ctl.close()
				log.Fatalf("%v", err)
			}
			defer os.Remove(demuxSock)
			go serve(ln, h, tmuxSock)
			log.Printf("attached to %s: %d sessions, %d windows, %d panes",
				tmuxSock, len(w.Sessions), len(w.Windows), len(w.Panes))
		} else {
			log.Printf("reattached to %s", tmuxSock)
		}

		if !consume(d, ctl, w, sig) {
			ctl.close()
			h.closeAll()
			return
		}
		ctl.close()
		// The attached session died (detach-on-destroy emits %exit while the
		// server lives on — verified). Give tmux a beat, then reattach.
		time.Sleep(100 * time.Millisecond)
	}
}

// consume runs the event loop for one control connection: re-lists, client
// commands, and sidebar upkeep all execute here, single-threaded — the
// serialization that makes the sh spike's race classes unrepresentable.
// Returns true if the connection ended but the daemon should reattach, false
// on shutdown.
func consume(d *daemon, ctl *control, w world, sig chan os.Signal) bool {
	var timer *time.Timer
	var fire <-chan time.Time
	for {
		select {
		case <-ctl.kick:
			if fire == nil {
				timer = time.NewTimer(debounce)
				fire = timer.C
			}
		case <-fire:
			fire = nil
			next, err := fetchWorld(ctl)
			if err != nil {
				// Connection is dying; the done case handles the rest.
				continue
			}
			ops := diffWorlds(w, next)
			w = next
			d.h.setWorld(w, ops, false, d.tmuxSock)
			d.checkBrowse(ctl, w)
		case env := <-d.h.cmds:
			d.handleCmd(ctl, env)
		case <-d.tickC:
			// Live preview stream: nil (never fires) unless browsing.
			if d.br != nil && d.br.open && d.br.target != "" {
				_ = d.preview(ctl, d.br.target)
			}
		case <-ctl.done:
			if timer != nil {
				timer.Stop()
			}
			return true
		case s := <-sig:
			log.Printf("%v, shutting down", s)
			return false
		}
	}
}

// listenExclusive binds the demux socket, stealing it only from the dead: if
// something answers on the socket, another daemon owns this server.
func listenExclusive(path string) (net.Listener, error) {
	ln, err := net.Listen("unix", path)
	if err == nil {
		return ln, nil
	}
	conn, derr := net.DialTimeout("unix", path, 250*time.Millisecond)
	if derr == nil {
		conn.Close()
		return nil, fmt.Errorf("daemon already running on %s", path)
	}
	if rerr := os.Remove(path); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
		return nil, fmt.Errorf("stale socket %s: %v", path, rerr)
	}
	return net.Listen("unix", path)
}
