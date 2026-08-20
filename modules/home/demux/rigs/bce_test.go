package rigs

import (
	"regexp"
	"strings"
	"testing"
)

// TestBCEBillboard: TUIs paint full-width bars as bg + erase-to-EOL (BCE);
// capture-pane drops the fill and leaves the bg OPEN at line end. The
// billboard painter must pad to the pane edge in that live state so the bar
// spans the cell like the real pane — not stop dead at the last glyph.
func TestBCEBillboard(t *testing.T) {
	r := New(t)

	shell := ""
	for _, ln := range strings.Split(r.T("list-panes", "-t", r.W1, "-F", "#{pane_id} #{pane_start_command}"), "\n") {
		f := strings.SplitN(ln, " ", 2)
		if len(f) == 2 && !strings.Contains(f[1], "MARKW1") {
			shell = f[0]
		}
	}
	// B=... keeps the markers out of the echoed command line, so only the
	// printed output contains the contiguous strings. Second line: an OSC 8
	// hyperlink (grok's TUI emits these) — its payload is zero-width and
	// must not skew padding or leak into the billboard.
	r.SendKeys(shell,
		`clear; B=BCE; printf '\033[44m'"$B"'BAR\033[K\033[0m\n\033]8;;http://osc-leak.test\033\\'"$B"'LINK\033]8;;\033\\ \033[45m'"$B"'BAR2\033[K\033[0m\n'`,
		"Enter")
	sleep(500)

	r.D("toggle", r.CL) // dock on beta
	sleep(800)
	sp := r.Side().Pane
	r.SendKeys(sp, "k") // billboard w1
	sleep(900)

	// poll: under full-suite CPU load the billboard paint can land seconds
	// late — the content is what matters, not the latency
	bar := regexp.MustCompile(`\x1b\[[0-9;]*44m[^\x1b]*BCEBAR {10,}`)
	out := ""
	r.WaitUntil(500, func() bool {
		out, _ = r.TQ("capture-pane", "-e", "-p", "-t", sp)
		return bar.MatchString(out)
	})
	r.Chk("billboard shows the bar", strings.Contains(out, "BCEBAR"))
	// the bg-open padding: the real bar (ESC[44m, not echoed text) runs on
	// in spaces still inside the 44 background, not an immediate reset
	r.Chk("bar extends past the text", bar.MatchString(out))
	r.Chk("hyperlink text kept", strings.Contains(out, "BCELINK"))
	r.Chk("OSC payload stripped", !strings.Contains(out, "osc-leak.test"))
	r.Chk("bar after link intact", regexp.MustCompile(`45m[^\x1b]*BCEBAR2 {5,}`).MatchString(out))
	for _, ln := range strings.Split(out, "\n") {
		if strings.Contains(ln, "BCEBAR") {
			t.Logf("bar line: %q", ln)
		}
	}
}
