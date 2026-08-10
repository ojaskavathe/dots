package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// The preview canvas: a passive renderer in the browse window's big pane.
// Paints frames (captured target windows) the daemon sends; takes no input.
func cmdCanvas(tmuxSock, demuxSock string) {
	conn, err := dialEnsure(tmuxSock, demuxSock)
	if err != nil {
		fmt.Fprintf(os.Stderr, "demuxd canvas: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()
	conn.Write([]byte(`{"type":"hello","role":"canvas"}` + "\n"))

	// Hide cursor; disable autowrap so pane lines wider than the canvas clip
	// at the right edge instead of wrapping and wrecking the layout.
	fmt.Print("\033[?25l\033[?7l\033[2J")

	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)

	frames := make(chan frameMsg, 8)
	go func() {
		sc := bufio.NewScanner(conn)
		sc.Buffer(make([]byte, 64*1024), 64*1024*1024)
		for sc.Scan() {
			var m frameMsg
			if json.Unmarshal(sc.Bytes(), &m) == nil && m.Type == "frame" {
				// Keep only the newest queued frame.
				for {
					select {
					case <-frames:
						continue
					default:
					}
					break
				}
				frames <- m
			}
		}
		close(frames)
	}()

	var last *frameMsg
	for {
		select {
		case m, ok := <-frames:
			if !ok {
				return
			}
			last = &m
			paintFrame(m)
		case <-winch:
			if last != nil {
				paintFrame(*last)
			}
		}
	}
}

func paintFrame(m frameMsg) {
	cols, rows, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || cols <= 0 {
		cols, rows = 80, 24
	}
	var b strings.Builder
	b.WriteString("\033[2J\033[0m")
	for _, p := range m.Panes {
		// Panes fully outside the canvas would clamp onto its last
		// column/row as garbage; drop them (autowrap-off clips the rest).
		if p.Left >= cols || p.Top >= rows {
			continue
		}
		for i, ln := range p.Lines {
			if i >= p.Height || p.Top+i >= rows {
				break
			}
			// 1-based cursor addressing; SGR reset per line so pane edges
			// don't bleed attributes into each other.
			fmt.Fprintf(&b, "\033[%d;%dH%s\033[0m", p.Top+1+i, p.Left+1, ln)
		}
	}
	os.Stdout.WriteString(b.String())
}
