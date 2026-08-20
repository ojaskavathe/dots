//go:build !noequalize

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Pins the hand-rolled msgpack-rpc client against a real nvim: dial, eval
// (the json_encode round trip equalize relies on), command, and error
// surfacing. Skips when nvim isn't on PATH (e.g. the nix build sandbox).
func TestNvimRPC(t *testing.T) {
	nvimBin, err := exec.LookPath("nvim")
	if err != nil {
		t.Skip("nvim not on PATH")
	}
	sock := filepath.Join(t.TempDir(), "nvim.sock")
	cmd := exec.Command(nvimBin, "--headless", "--clean", "--listen", sock)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start nvim: %v", err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(sock); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("nvim socket never appeared")
		}
		time.Sleep(20 * time.Millisecond)
	}

	c, err := nvimDial(sock)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	text, err := c.EvalJSON("winlayout()")
	if err != nil {
		t.Fatalf("eval winlayout: %v", err)
	}
	var layout any
	if err := json.Unmarshal([]byte(text), &layout); err != nil {
		t.Fatalf("winlayout json %q: %v", text, err)
	}
	items, ok := layout.([]any)
	if !ok || len(items) < 1 || items[0] != "leaf" {
		t.Fatalf("fresh nvim should be a single leaf, got %q", text)
	}

	if err := c.Command("vsplit"); err != nil {
		t.Fatalf("command vsplit: %v", err)
	}
	text, err = c.EvalJSON("winlayout()")
	if err != nil {
		t.Fatalf("eval after vsplit: %v", err)
	}
	if err := json.Unmarshal([]byte(text), &layout); err != nil {
		t.Fatalf("json after vsplit: %v", err)
	}
	if items, _ = layout.([]any); len(items) < 1 || items[0] != "row" {
		t.Fatalf("after vsplit expected row, got %q", text)
	}
	if got := eqAxisCount(layout, "x"); got != 2 {
		t.Fatalf("axis count x after vsplit = %d, want 2", got)
	}

	if err := c.Command("not-a-real-command"); err == nil {
		t.Fatal("bad command should surface an rpc error")
	}
	// The connection must survive an error reply.
	if _, err := c.EvalJSON("1+1"); err != nil {
		t.Fatalf("eval after error: %v", err)
	}
}
