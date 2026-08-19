package rigs

import (
	"os"
	"regexp"
	"testing"
)

// TestHeavyHistory is the live-@4 scenario: a window whose pane carries
// hundreds of thousands of scrollback lines. tmux reflows that history
// synchronously on ANY width change (~200ms observed live), which is why
// enters are swaps, not resizes. First billboard pays the one-time carve;
// every enter/leave/re-enter must stay under the daemon's 25ms slow-log
// threshold.
func TestHeavyHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("slow: fills 700k lines of scrollback")
	}
	r := New(t)
	r.T("set-option", "-g", "history-limit", "100000000")
	r.T("new-window", "-t", "work:", "-n", "heavy")
	r.T("send-keys", "-t", "work:heavy",
		`seq 700000 | awk '{print $0, $0*3, "pad pad pad pad pad"}'; clear`, "Enter")
	r.T("split-window", "-h", "-t", "work:heavy")
	sleep(12000)
	r.T("select-window", "-t", r.W2)
	sleep(300)

	r.D("toggle", r.CL)
	sleep(1000)
	sp := r.Side().Pane
	// scrub onto heavy (carve happens here), enter, leave, re-enter, undock
	r.SendKeys(sp, "j")
	sleep(800)
	r.SendKeys(sp, "Enter")
	sleep(1000)
	r.SendKeys(sp, "k")
	sleep(500)
	r.SendKeys(sp, "Enter")
	sleep(1000)
	r.SendKeys(sp, "j")
	sleep(500)
	r.SendKeys(sp, "Enter")
	sleep(1000)
	r.D("toggle", r.CL) // undock: release may be slow but is invisible
	sleep(2500)

	b, err := os.ReadFile(r.Sock + ".log")
	if err != nil {
		t.Fatalf("daemon log: %v", err)
	}
	// commits/toggles crossing 25ms log as "<cmd> took" — none allowed
	for _, m := range regexp.MustCompile(`(?m)^.*(commit|toggle|nav) took.*$`).FindAllString(string(b), -1) {
		t.Errorf("slow interaction: %s", m)
	}
	r.Chk("no spacers remain", r.Spacers() == 0)
}
