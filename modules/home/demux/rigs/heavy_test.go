package rigs

import (
	"os"
	"regexp"
	"testing"
)

// TestHeavyHistory is the live-@4 scenario: a window whose pane carries
// hundreds of thousands of scrollback lines. tmux reflows that history
// synchronously on ANY width change (~250ms observed live), so such windows
// are never pre-carved during scrubbing (billboard = scaled approximation)
// — only an actual Enter pays the carve, once, and the release drains
// deferred after undock.
func TestHeavyHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("slow: fills 700k lines of scrollback")
	}
	r := New(t)
	r.T("set-option", "-g", "history-limit", "100000000")
	r.T("new-window", "-t", "work:", "-n", "heavy")
	heavy := r.T("display-message", "-p", "-t", "work:heavy", "#{window_id}")
	r.T("send-keys", "-t", "work:heavy",
		`seq 700000 | awk '{print $0, $0*3, "pad pad pad pad pad"}'; clear`, "Enter")
	r.T("split-window", "-h", "-t", "work:heavy")
	sleep(12000)
	r.T("select-window", "-t", r.W2)
	sleep(300)

	r.D("toggle", r.CL)
	sleep(1000)
	sp := r.Side().Pane
	// scrub onto heavy (no carve — over the history cap), enter (pays the
	// carve, once), leave, re-enter (pure swaps), undock (deferred release)
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
	r.D("toggle", r.CL)
	sleep(2500)

	b, err := os.ReadFile(r.Sock + ".log")
	if err != nil {
		t.Fatalf("daemon log: %v", err)
	}
	log := string(b)
	// billboarding must never carve a heavy window
	if regexp.MustCompile(`bench carve win=` + regexp.QuoteMeta(heavy)).MatchString(log) {
		t.Errorf("scrub pre-carved the heavy window")
	}
	// only the FIRST enter may cross the slow threshold (its one-time carve);
	// re-enters are swaps and undock must not stall on the release
	slow := regexp.MustCompile(`(?m)^.*(commit|toggle|nav) took.*$`).FindAllString(log, -1)
	if len(slow) > 1 {
		for _, m := range slow[1:] {
			t.Errorf("slow interaction beyond the entry carve: %s", m)
		}
	}
	r.Chk("deferred release drained", r.Spacers() == 0)
}
