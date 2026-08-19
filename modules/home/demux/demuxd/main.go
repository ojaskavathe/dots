// demuxd — the demux daemon (milestone 1: world model + pub/sub).
//
// One daemon per tmux server. It holds a single persistent control-mode
// connection (`tmux -C attach`), reduces notifications into a world model
// (sessions/windows/panes/clients), and publishes snapshot+diff NDJSON to
// subscribers on a unix socket.
//
// Verified on tmux 3.7b (2026-08-10), single control client:
//   - topology changes in OTHER sessions all notify: %unlinked-window-add,
//     %unlinked-window-close, %unlinked-window-renamed, %session-window-changed,
//     %session-renamed $id, %sessions-changed, %window-pane-changed
//   - geometry does NOT cross sessions: %layout-change is own-session only,
//     resize-pane elsewhere emits NOTHING, and refresh-client -B "%*" format
//     subscriptions only evaluate panes of the attached session
//   - %output is own-session only by default
//   - killing the attached session emits %exit (detach-on-destroy) while the
//     server lives on -> the daemon must reattach, not die
//   - the initial attach emits one unsolicited %begin/%end block -> command
//     replies cannot assume every block has a sender
//
// Consequences: notifications are dirty-triggers only; every trigger causes one
// debounced re-list over the same connection (whole-world, ~KBs, sub-ms — the
// doc's "start with whole-entity" choice). Cross-session geometry is refreshed
// by those re-lists and must be re-queried at point-of-use by clients that are
// about to act on it (the sidebar queries right before a join).
package main

import (
	"flag"
	"fmt"
	"os"
)

// set via -ldflags "-X main.tmuxPath=..."
var tmuxPath = "tmux"

func usage() {
	fmt.Fprintf(os.Stderr, `usage: demuxd [-L name | -S path] <command>

commands:
  run                    run the daemon in the foreground
  ls                     print the world (starts the daemon if needed)
  events                 stream snapshot + diffs as NDJSON (starts the daemon if needed)
  toggle <client>        dock / undock the sidebar for a tmux client
  nav <next|prev> <cl>   window nav with the sidebar riding along (routed M-h/M-l)
  browse <client>        full-screen billboard browser (the pre-pin M-s)
  tui                    the sidebar TUI (spawned by the daemon)
  sock                   print the tmux and demux socket paths and exit
`)
	os.Exit(2)
}

func main() {
	fs := flag.NewFlagSet("demuxd", flag.ExitOnError)
	fs.Usage = usage
	lName := fs.String("L", "", "tmux socket name (as tmux -L)")
	sPath := fs.String("S", "", "tmux socket path (as tmux -S)")
	_ = fs.Parse(os.Args[1:])

	tmuxSock := resolveTmuxSocket(*sPath, *lName)
	demuxSock := demuxSocketPath(tmuxSock)

	args := fs.Args()
	if len(args) < 1 {
		usage()
	}
	switch args[0] {
	case "run":
		runDaemon(tmuxSock, demuxSock)
	case "ls":
		cmdLs(tmuxSock, demuxSock)
	case "events":
		cmdEvents(tmuxSock, demuxSock)
	case "toggle":
		client := ""
		if len(args) > 1 {
			client = args[1]
		}
		cmdToggle(tmuxSock, demuxSock, client)
	case "nav":
		if len(args) < 3 || (args[1] != "next" && args[1] != "prev") {
			usage()
		}
		cmdNav(tmuxSock, demuxSock, args[1], args[2])
	case "browse":
		client := ""
		if len(args) > 1 {
			client = args[1]
		}
		cmdBrowse(tmuxSock, demuxSock, client)
	case "tui":
		cmdTui(tmuxSock, demuxSock)
	case "sock":
		fmt.Printf("tmux:  %s\ndemux: %s\n", tmuxSock, demuxSock)
	default:
		usage()
	}
}
