package main

import (
	"errors"
	"fmt"
	"log"
	"time"
)

// The command router: client cmds arrive on the hub queue, get coalesced by
// handleCmd, and runCmd dispatches each. Everything is docked-mode now —
// `demuxd browse` is dock + zoom — so the dock owns every command. All of it
// runs on the consume loop — one thread, serialized with re-lists and tmux
// commands.

// handleCmd drains everything already queued and executes only what still
// matters: every non-preview command in order, the NEWEST real preview, and
// the prefetches that came after it (they describe the final position).
// Stale previews and stale prefetches are acked without running — during a
// fast scrub they'd otherwise serialize ahead of the frame the user is
// actually looking at. Remaining prefetches are abandoned the moment fresher
// input arrives.
func (d *daemon) handleCmd(ctl *control, env cmdEnvelope) {
	batch := []cmdEnvelope{env}
	for {
		select {
		case next := <-d.h.cmds:
			batch = append(batch, next)
			continue
		default:
		}
		break
	}
	lastReal := -1
	for i, e := range batch {
		if e.msg.Cmd == "preview" && !e.msg.Prefetch {
			lastReal = i
		}
	}
	for i, e := range batch {
		isPreview := e.msg.Cmd == "preview"
		switch {
		case !isPreview:
			d.runCmd(ctl, e)
		case !e.msg.Prefetch && i == lastReal:
			d.runCmd(ctl, e)
		case e.msg.Prefetch && i > lastReal:
			if len(d.h.cmds) > 0 {
				// Fresher input queued: this prefetch is already history.
				d.h.send(e.sub, marshalLine(replyMsg{Type: "reply", OK: true}))
				continue
			}
			d.runCmd(ctl, e)
		default:
			d.h.send(e.sub, marshalLine(replyMsg{Type: "reply", OK: true}))
		}
	}
}

func (d *daemon) runCmd(ctl *control, env cmdEnvelope) {
	start := time.Now()
	var err error
	if d.handoff != nil && env.msg.Cmd != "hello-list" {
		// A commit is mid-handoff (≤300ms): the world is half-moved, so
		// anything else acks and drops rather than racing it.
		d.h.send(env.sub, marshalLine(replyMsg{Type: "reply", OK: true}))
		return
	}
	switch env.msg.Cmd {
	case "toggle":
		err = d.toggle(ctl, env.msg.Client)
	case "browse":
		err = d.browseOpen(ctl, env.msg.Client)
	case "nav":
		err = d.dockNav(ctl, env.msg.Dir)
	case "preview":
		// Selection leaving the real window starts billboard scrubbing
		// (zoom + captures); while scrubbing, previews and prefetches are
		// plain billboards. Nothing real moves until commit.
		if p := d.dock; p != nil {
			switch {
			case p.scrubbing:
				if !env.msg.Prefetch && env.msg.Window != "" && env.msg.Window != d.pv.target {
					d.scrubStatusSet(ctl, env.msg.Window)
				}
				err = d.preview(ctl, env.msg.Window, env.msg.Prefetch)
			case !env.msg.Prefetch && env.msg.Window != "" && env.msg.Window != p.win:
				err = d.scrubStart(ctl, env.msg.Window)
			}
		}
	case "winch":
		// The TUI's pane changed size. Docked idle that means a client
		// resize (monitor switch) rescaled the sidebar off its fixed width;
		// nothing else will tell us — geometry events don't cross sessions.
		// Zoomed (scrubbing) the sidebar is full-width by design: skip.
		if d.dock != nil && !d.dock.scrubbing && env.msg.Width != listWidth {
			_, err = ctl.run(fmt.Sprintf("resize-pane -t %s -x %d", q(d.dock.pane), listWidth))
		}
	case "focus":
		// C-l from the docked idle sidebar: select the pane geometrically
		// right of it — vim-tmux-navigator semantics, no origin reset.
		if d.dock != nil && !d.dock.scrubbing {
			_, err = ctl.run("select-pane -R -t " + q(d.dock.pane))
		}
	case "commit":
		if d.dock != nil {
			if d.dock.scrubbing {
				err = d.commitScrub(ctl, env.msg.Window, env.msg.Pane)
			} else {
				err = d.dockCommit(ctl)
			}
		}
	case "close":
		if d.dock != nil {
			if d.dock.scrubbing {
				// q mid-scrub: unzoom — the origin panes reappear untouched.
				d.scrubEnd(ctl, true)
				d.h.sendRole("list", marshalLine(selectMsg{Type: "select", Window: d.dock.win}))
			} else {
				err = d.dockClose(ctl, true)
			}
		}
	case "hello-list":
		// A fresh TUI connected. Mid-handoff this IS the go signal: it has
		// the world, gets the target selection, paints, and the client
		// switches onto an already-painted sidebar.
		if d.handoff != nil {
			d.h.send(env.sub, marshalLine(selectMsg{Type: "select", Window: d.handoff.wid}))
			d.handoffFinish(ctl)
			break
		}
		// Otherwise dockOpen just spawned it; replay the selection it
		// missed (and the current frame, when browse pre-zoomed into a scrub).
		if d.dock != nil {
			d.h.send(env.sub, marshalLine(selectMsg{Type: "select", Window: d.dock.win}))
			if d.dock.scrubbing && d.pv.lastFrame != nil {
				d.h.send(env.sub, d.pv.lastFrame)
			}
			log.Printf("hello-list: replay docked select=%s spawn_ms=%d",
				d.dock.win, time.Since(d.dock.openedAt).Milliseconds())
		}
	default:
		err = fmt.Errorf("unknown cmd %q", env.msg.Cmd)
	}
	wait := start.Sub(env.recv)
	if dur := time.Since(start); dur > 25*time.Millisecond || wait > 25*time.Millisecond {
		// wait = time queued behind whatever the event loop was doing
		// (a re-list, a stream tick, an earlier command) before this ran.
		log.Printf("%s took %s (queued %s)", env.msg.Cmd, dur, wait)
	} else if bench {
		log.Printf("bench cmd=%s prefetch=%v dur_us=%d", env.msg.Cmd, env.msg.Prefetch, dur.Microseconds())
	}
	r := replyMsg{Type: "reply", OK: err == nil}
	if err != nil {
		r.Err = err.Error()
	}
	d.h.send(env.sub, marshalLine(r))
}

// toggle is M-s: docked -> undock (mid-scrub: commit-and-dismiss); otherwise
// dock the sidebar into the current window.
func (d *daemon) toggle(ctl *control, client string) error {
	if client == "" {
		return errors.New("toggle needs a client name")
	}
	if d.dock != nil {
		if d.dock.scrubbing {
			// M-s mid-scrub commits AND dismisses — browse-era muscle
			// memory: one press lands you in the selection. (Enter is the
			// commit that keeps the sidebar docked.)
			target := d.pv.target
			sid := d.sessionOf(target)
			if sid != "" && target != d.dock.win {
				if t := d.dock.carved[target]; t != nil && t.spacer != "" {
					// Carved: swap the sidebar in first (geometry-free —
					// content in front of the user instantly), THEN undock;
					// the expand-to-full-width reflow happens behind content
					// the user is already reading, never before it.
					if err := d.dockMove(ctl, target, true); err != nil {
						return err
					}
					return d.dockClose(ctl, false)
				}
				// Uncarved (huge-scrollback windows deliberately stay so):
				// the target is already at full width — land on it directly,
				// zero geometry changes, zero reflows. Committing first
				// would carve it (~200ms history reflow) only to expand it
				// right back.
				d.dock.originSess, d.dock.originWin = sid, target
				return d.dockClose(ctl, true)
			}
			return d.dockClose(ctl, false)
		}
		return d.dockClose(ctl, false)
	}
	return d.dockOpen(ctl, client)
}

// browseOpen is `demuxd browse`: dock into the client's current window and
// zoom straight into billboard scrubbing — the old full-screen browser is
// now just the dock's scrub view, opened from row one.
func (d *daemon) browseOpen(ctl *control, client string) error {
	if d.dock == nil {
		if err := d.dockOpen(ctl, client); err != nil {
			return err
		}
	}
	if d.dock.scrubbing {
		return nil
	}
	return d.scrubStart(ctl, d.dock.win)
}
