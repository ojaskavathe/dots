package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// slowRT logs any control round trip the tmux server took long to answer —
// the per-phase attribution for slow commands (which RT inside a handler ate
// the time, and what the server was chewing on when it did).
func slowRT(start time.Time, line string) {
	if dur := time.Since(start); dur > 20*time.Millisecond {
		if len(line) > 80 {
			line = line[:80]
		}
		log.Printf("slow rt %s: %s", dur, line)
	}
}

// control is one persistent `tmux -C attach` connection. Notifications become
// dirty-kicks (never queued per-event: milestone 1 only needs "something
// changed"); command replies are correlated FIFO against %begin/%end blocks.
type control struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser

	// kick has capacity 1: the reader nudges it without ever blocking, so the
	// reader can never deadlock against a daemon that is mid-command.
	kick chan struct{}

	mu      sync.Mutex
	hello   bool // the unsolicited %begin/%end block from attach is still owed
	pending []*pendingCmd

	done    chan struct{} // closed when the reader exits (connection over)
	exitErr error
}

type cmdReply struct {
	lines []string
	err   error
}

// pendingCmd tracks one sent line. A line holding a "a ; b ; c" sequence gets
// one %begin/%end block PER command, and an %error ABORTS the rest of the
// sequence (verified: the blocks after a failing command never arrive) — so a
// reply is complete at n blocks or at the first %error, whichever comes first.
type pendingCmd struct {
	n     int
	got   int
	lines []string
	ch    chan cmdReply
}

// dialControl attaches a control-mode client to the tmux server. The attach
// target is whatever tmux picks (most recently used); the daemon does not care
// which session it sits on, only that the connection exists.
func dialControl(tmuxSock string) (*control, error) {
	cmd := exec.Command(tmuxPath, "-S", tmuxSock, "-C", "attach-session")
	// Never look nested, even when the daemon is spawned from inside tmux.
	env := os.Environ()
	filtered := env[:0]
	for _, e := range env {
		if strings.HasPrefix(e, "TMUX=") || strings.HasPrefix(e, "TMUX_PANE=") {
			continue
		}
		filtered = append(filtered, e)
	}
	cmd.Env = filtered

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	c := &control{
		cmd:   cmd,
		stdin: stdin,
		hello: true,
		kick:  make(chan struct{}, 1),
		done:  make(chan struct{}),
	}
	go c.read(stdout)
	return c, nil
}

func (c *control) read(stdout io.Reader) {
	sc := bufio.NewScanner(stdout)
	// %output lines and layout strings can be large.
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)

	inBlock := false
	blockErr := false
	var block []string
	for sc.Scan() {
		line := sc.Text()
		if inBlock {
			// Only the block terminators end a block: payload lines may
			// themselves start with '%' (pane ids in format output).
			if strings.HasPrefix(line, "%end ") || strings.HasPrefix(line, "%error ") {
				blockErr = strings.HasPrefix(line, "%error ")
				c.deliver(block, blockErr)
				inBlock = false
				block = nil
				continue
			}
			block = append(block, line)
			continue
		}
		if strings.HasPrefix(line, "%begin ") {
			inBlock = true
			continue
		}
		name, _, _ := strings.Cut(line, " ")
		switch name {
		case "%output", "%extended-output":
			// ignored until the preview milestone; must not cause re-lists
		case "%exit":
			// reader will hit EOF next; done covers it
		default:
			select {
			case c.kick <- struct{}{}:
			default:
			}
		}
	}
	if err := sc.Err(); err != nil {
		c.exitErr = err
	}
	c.mu.Lock()
	for _, p := range c.pending {
		p.ch <- cmdReply{err: errors.New("control connection closed")}
	}
	c.pending = nil
	c.mu.Unlock()
	close(c.done)
	_ = c.cmd.Wait()
}

func (c *control) deliver(lines []string, isErr bool) {
	c.mu.Lock()
	if c.hello {
		// The unsolicited block tmux emits for the attach itself. It is
		// guaranteed first (commands cannot be processed before the attach
		// completes); delivering it to a waiter would misalign every reply.
		c.hello = false
		c.mu.Unlock()
		return
	}
	if len(c.pending) == 0 {
		c.mu.Unlock()
		return
	}
	p := c.pending[0]
	p.lines = append(p.lines, lines...)
	p.got++
	if !isErr && p.got < p.n {
		c.mu.Unlock()
		return
	}
	c.pending = c.pending[1:]
	c.mu.Unlock()
	r := cmdReply{lines: p.lines}
	if isErr {
		r.err = fmt.Errorf("tmux: %s", strings.Join(lines, " "))
	}
	p.ch <- r
}

// run sends one tmux command down the connection and waits for its reply.
func (c *control) run(command string) ([]string, error) {
	return c.runSeq(command)
}

// runSeq sends the commands as one "a ; b ; c" line: a single client command
// list, contiguous in tmux's queue — nothing from another client can land
// between them, and an error in one aborts the rest (order critical-first).
func (c *control) runSeq(commands ...string) ([]string, error) {
	p := &pendingCmd{n: len(commands), ch: make(chan cmdReply, 1)}
	line := strings.Join(commands, " ; ")
	c.mu.Lock()
	c.pending = append(c.pending, p)
	start := time.Now()
	_, err := io.WriteString(c.stdin, line+"\n")
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}
	select {
	case r := <-p.ch:
		slowRT(start, line)
		return r.lines, r.err
	case <-c.done:
		return nil, errors.New("control connection closed")
	}
}

// runPipelined sends several command lines in ONE stdin write: the server
// reads them in a single pass, so their effects — resizes especially — land
// back-to-back and apps coalesce the SIGWINCHes into one reflow, while each
// line keeps its own reply and its own %error scope. Replies are awaited in
// order; errs[i] belongs to lines[i].
func (c *control) runPipelined(lines ...[]string) ([][]string, []error) {
	ps := make([]*pendingCmd, len(lines))
	var buf strings.Builder
	for i, cmds := range lines {
		ps[i] = &pendingCmd{n: len(cmds), ch: make(chan cmdReply, 1)}
		buf.WriteString(strings.Join(cmds, " ; "))
		buf.WriteByte('\n')
	}
	c.mu.Lock()
	c.pending = append(c.pending, ps...)
	start := time.Now()
	_, werr := io.WriteString(c.stdin, buf.String())
	c.mu.Unlock()
	outs := make([][]string, len(lines))
	errs := make([]error, len(lines))
	if werr != nil {
		for i := range errs {
			errs[i] = werr
		}
		return outs, errs
	}
	for i, p := range ps {
		select {
		case r := <-p.ch:
			outs[i], errs[i] = r.lines, r.err
			slowRT(start, strings.Join(lines[i], " ; "))
		case <-c.done:
			errs[i] = errors.New("control connection closed")
		}
	}
	return outs, errs
}

func (c *control) close() {
	_ = c.stdin.Close()
	_ = c.cmd.Process.Kill()
	<-c.done
}
