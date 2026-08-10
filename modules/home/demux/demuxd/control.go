package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

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
	pending []chan cmdReply

	done    chan struct{} // closed when the reader exits (connection over)
	exitErr error
}

type cmdReply struct {
	lines []string
	err   error
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
	for _, ch := range c.pending {
		ch <- cmdReply{err: errors.New("control connection closed")}
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
	ch := c.pending[0]
	c.pending = c.pending[1:]
	c.mu.Unlock()
	r := cmdReply{lines: lines}
	if isErr {
		r.err = fmt.Errorf("tmux: %s", strings.Join(lines, " "))
	}
	ch <- r
}

// run sends one tmux command down the connection and waits for its reply.
func (c *control) run(command string) ([]string, error) {
	ch := make(chan cmdReply, 1)
	c.mu.Lock()
	c.pending = append(c.pending, ch)
	_, err := io.WriteString(c.stdin, command+"\n")
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}
	select {
	case r := <-ch:
		return r.lines, r.err
	case <-c.done:
		return nil, errors.New("control connection closed")
	}
}

func (c *control) close() {
	_ = c.stdin.Close()
	_ = c.cmd.Process.Kill()
	<-c.done
}
