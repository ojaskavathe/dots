package main

import (
	"errors"
	"fmt"
	"log"
	"time"
)

// The command router: client cmds arrive on the hub queue, get coalesced by
// handleCmd, and runCmd dispatches each to whichever mode owns it right now
// (docked sidebar, full-screen browse, or neither). All of it runs on the
// consume loop — one thread, serialized with re-lists and tmux commands.

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
	browsing := d.browse.open
	var err error
	switch env.msg.Cmd {
	case "toggle":
		err = d.toggle(ctl, env.msg.Client)
	case "browse":
		err = d.browseOpen(ctl, env.msg.Client)
	case "nav":
		err = d.dockNav(ctl, env.msg.Dir)
	case "preview":
		if d.dock != nil && !browsing {
			// Docked: selection leaving the real window starts billboard
			// scrubbing (zoom + captures); while scrubbing, previews and
			// prefetches are plain billboards. Nothing real moves until
			// commit.
			p := d.dock
			switch {
			case p.scrubbing:
				err = d.preview(ctl, env.msg.Window, env.msg.Prefetch)
			case !env.msg.Prefetch && env.msg.Window != "" && env.msg.Window != p.win:
				err = d.scrubStart(ctl, env.msg.Window)
			}
		} else {
			err = d.preview(ctl, env.msg.Window, env.msg.Prefetch)
		}
	case "winch":
		// The TUI's pane changed size. Docked idle that means a client
		// resize (monitor switch) rescaled the sidebar off its fixed width;
		// nothing else will tell us — geometry events don't cross sessions.
		// Zoomed (scrubbing) the sidebar is full-width by design, and the
		// full-screen browser owns its whole window: both skip.
		if d.dock != nil && !browsing && !d.dock.scrubbing && env.msg.Width != listWidth {
			_, err = ctl.run(fmt.Sprintf("resize-pane -t %s -x %d", q(d.sur.pane), listWidth))
		}
	case "focus":
		// C-l from the docked idle sidebar: select the pane geometrically
		// right of it — vim-tmux-navigator semantics, no origin reset.
		if d.dock != nil && !browsing && !d.dock.scrubbing {
			_, err = ctl.run("select-pane -R -t " + q(d.sur.pane))
		}
	case "commit":
		if d.dock != nil && !browsing {
			if d.dock.scrubbing {
				err = d.commitScrub(ctl, env.msg.Window)
			} else {
				err = d.dockCommit(ctl)
			}
		} else {
			err = d.commit(ctl, env.msg.Window)
		}
	case "close":
		if d.dock != nil && !browsing {
			if d.dock.scrubbing {
				// q mid-scrub: unzoom — the origin panes reappear untouched.
				d.scrubEnd(ctl, true)
				d.h.sendRole("list", marshalLine(selectMsg{Type: "select", Window: d.dock.win}))
			} else {
				err = d.dockClose(ctl, true)
			}
		} else {
			err = d.closeBrowse(ctl)
		}
	case "hello-list":
		// A TUI connected after state went out; replay selection + frame.
		if d.dock != nil {
			d.h.send(env.sub, marshalLine(selectMsg{Type: "select", Window: d.dock.win}))
			log.Printf("hello-list: replay docked select=%s", d.dock.win)
			break
		}
		replaySel := browsing && d.pv.target != ""
		if replaySel {
			d.h.send(env.sub, marshalLine(selectMsg{Type: "select", Window: d.pv.target}))
		}
		log.Printf("hello-list: replay select=%v target=%s frame=%v",
			replaySel, d.pv.target, d.pv.lastFrame != nil)
		if d.pv.lastFrame != nil {
			d.h.send(env.sub, d.pv.lastFrame)
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

// toggle is M-s: browsing (full-screen) -> commit/close as before; docked ->
// undock in place; otherwise dock the sidebar into the current window.
func (d *daemon) toggle(ctl *control, client string) error {
	if client == "" {
		return errors.New("toggle needs a client name")
	}
	if d.browse.open {
		sid, _, _, _, err := d.clientView(ctl, client)
		if err == nil && sid == d.sur.sess {
			d.browse.client = client
			// M-s while browsing commits to the current selection, like Enter —
			// muscle memory from the join-sidebar era. q / Ctrl-C remain
			// cancel-to-origin.
			if d.pv.target != "" && d.pv.target != d.browse.originWin {
				log.Printf("toggle-off commits to %s", d.pv.target)
				return d.commit(ctl, d.pv.target)
			}
			return d.closeBrowse(ctl)
		}
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
