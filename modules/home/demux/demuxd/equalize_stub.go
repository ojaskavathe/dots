//go:build noequalize

package main

import (
	"fmt"
	"os"
)

// The equalize subcommand is compile-time optional: `-tags noequalize`
// drops equalize.go and the nvim RPC client from the binary entirely.
// Everything else in demuxd is independent of them.
func cmdEqualize(_, _ string) {
	fmt.Fprintln(os.Stderr, "demuxd: built without equalize (-tags noequalize)")
	os.Exit(1)
}
