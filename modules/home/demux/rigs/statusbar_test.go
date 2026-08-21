package rigs

import (
	"strings"
	"testing"
)

// TestScrubStatus: while scrubbing, the status line must describe the
// TARGET — its session's window list with the scrub target marked current,
// via the daemon's status-format override — not the origin the client is
// still parked in. And it must snap back exactly on q.
func TestScrubStatus(t *testing.T) {
	r := New(t)

	r.D("toggle", r.CL) // dock on beta (work session)
	sleep(800)
	sp := r.Side().Pane
	// beta(5) -> w1(4) -> work(3) -> ptwo(2): a cross-session scrub
	r.SendKeys(sp, "k", "k", "k")
	sleep(900)

	pt := r.T("display-message", "-p", "-t", "play:ptwo", "#{window_id}")
	over, _ := r.TQ("show-options", "-t", "work", "status-format")
	r.Chk("override installed on origin", strings.Contains(over, pt))
	rendered := r.T("display-message", "-p", "-t", "work:", "#{T:status-format[0]}")
	r.Chk("status renders the target session's windows", strings.Contains(rendered, "ptwo"))
	r.Chk("origin windows gone from the bar", !strings.Contains(rendered, "beta"))

	r.SendKeys(sp, "q")
	sleep(700)
	over, _ = r.TQ("show-options", "-t", "work", "status-format")
	r.Chk("override removed on q", strings.TrimSpace(over) == "")
	rendered = r.T("display-message", "-p", "-t", "work:", "#{T:status-format[0]}")
	r.Chk("status back to the origin's windows", strings.Contains(rendered, "beta"))
}
