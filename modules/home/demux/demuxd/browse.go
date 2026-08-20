package main

import (
	"errors"
	"log"
)

// Full-screen browse mode: choose-tree mechanics, our UI. The TUI's home
// window becomes the whole screen (list pane + preview canvas). M-s/`demuxd
// browse` switches the client TO it; scrolling paints capture frames into
// the canvas; Enter switches the client to the target; q returns to origin.
// User windows are never joined into, resized, renamed, or otherwise touched
// — the entire baseline/restore problem class of the join-based sidebar (git
// history: f2538c2) does not exist here, and scrubbing costs one capture
// round trip instead of a window-wide reflow. No longer on M-s — docked mode
// replaced it there — but fully functional.
//
// Rules carried from the spike, still load-bearing:
//   - sequences abort at the first error -> critical commands first, targets
//     validated against the live model
//   - all client switching is explicit (-c): implicit switch-client picks
//     the wrong client whenever more than one is attached (the daemon's own
//     control client always is)
//   - `display-message -p -c X` does NOT expand formats in X's context;
//     per-client truth comes from list-clients rows

// browseState is the mode's state: whether the client is parked on the
// browse surface, which client, and where q returns to.
type browseState struct {
	open       bool
	client     string // browsing client (explicit switching)
	originSess string // where q returns to
	originWin  string
}

// browseOpen is the full-screen billboard browser (`demuxd browse`): switch
// the client to the _demux window and stream capture frames.
func (d *daemon) browseOpen(ctl *control, client string) error {
	if client == "" {
		return errors.New("browse needs a client name")
	}
	if d.dock != nil {
		if err := d.dockClose(ctl, false); err != nil {
			return err
		}
	}
	sid, wid, cw, ch, err := d.clientView(ctl, client)
	if err != nil {
		return err
	}
	if d.sur != nil && d.browse.open && sid == d.sur.sess {
		return nil // already browsing
	}
	if err := d.ensureSurface(ctl, cw, ch); err != nil {
		return err
	}
	d.browse = browseState{open: true, client: client, originSess: sid, originWin: wid}
	// Selection first: the list repaints while the browse window is still
	// hidden, so it is already on the origin row when the client lands. A
	// freshly spawned TUI isn't connected yet (n=0) — the hello-list replay
	// covers that; n=0 on a WARM browse means the select was lost.
	n := d.h.sendRole("list", marshalLine(selectMsg{Type: "select", Window: wid}))
	log.Printf("browse open client=%s origin=%s/%s size=%dx%d select_receivers=%d", client, sid, wid, cw, ch, n)
	if _, err := ctl.runSeq(
		"select-window -t "+q(d.sur.win),
		"switch-client -c "+q(client)+" -t "+q(d.sur.sess)); err != nil {
		return err
	}
	d.startStream()
	return d.preview(ctl, wid, false)
}

// commit: switch the client to the chosen window for real. The browse window
// stays alive for next time.
func (d *daemon) commit(ctl *control, wid string) error {
	if !d.browse.open {
		return nil
	}
	sid := ""
	for _, w := range d.h.getWorld().Windows {
		if w.ID == wid {
			sid = w.SessionID
			break
		}
	}
	if sid == "" {
		log.Printf("commit target=%s unknown, closing to origin", wid)
		return d.closeBrowse(ctl)
	}
	d.browse.open = false
	d.stopStream()
	log.Printf("commit client=%s -> %s/%s", d.browse.client, sid, wid)
	_, err := ctl.runSeq(
		"select-window -t "+q(wid),
		"switch-client -c "+q(d.browse.client)+" -t "+q(sid))
	return err
}

// closeBrowse returns the client to where it was when it summoned.
func (d *daemon) closeBrowse(ctl *control) error {
	if !d.browse.open {
		return nil
	}
	d.browse.open = false
	d.stopStream()
	log.Printf("close client=%s -> origin %s/%s", d.browse.client, d.browse.originSess, d.browse.originWin)
	_, err := ctl.runSeq(
		"select-window -t "+q(d.browse.originWin),
		"switch-client -c "+q(d.browse.client)+" -t "+q(d.browse.originSess))
	if err != nil {
		// Origin may have died while browsing; session alone, then give up
		// gracefully (the user can switch by hand).
		_, err = ctl.run("switch-client -c " + q(d.browse.client) + " -t " + q(d.browse.originSess))
	}
	return err
}

// checkBrowse runs after every re-list: if the client escaped the browse
// window by other means (detach, manual switch), browsing is over.
func (d *daemon) checkBrowse(ctl *control, w world) {
	if !d.browse.open {
		return
	}
	found := false
	for _, c := range w.Clients {
		if c.Name == d.browse.client {
			found = true
			if c.SessionID != d.sur.sess {
				d.browse.open = false
				d.stopStream()
				return
			}
			break
		}
	}
	if !found {
		d.browse.open = false // client detached
		d.stopStream()
	}
}
