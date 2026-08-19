package rigs

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestTmuxPrimitives pins the tmux behaviors the dock architecture is built
// on, so a tmux upgrade that changes any of them fails loudly here instead
// of as a mystery drift in the sidebar:
//   - split-window and join-pane carve a left 40-col slot with IDENTICAL
//     remainder arithmetic (billboards are exact because of this)
//   - swap-pane is geometry-free on both windows (enters are free)
//   - swap-pane works with a zoomed source and auto-unzooms
//   - {top-left} pane addressing resolves inside a command sequence
func TestTmuxPrimitives(t *testing.T) {
	t.Parallel()
	L := "primitives" + strconv.Itoa(os.Getpid())
	T := func(args ...string) string {
		t.Helper()
		out, err := tmuxRaw(L, args...)
		if err != nil {
			t.Fatalf("tmux %v: %v\n%s", args, err, out)
		}
		return out
	}
	t.Cleanup(func() { tmuxRaw(L, "kill-server") })
	tmuxRaw(L, "kill-server")
	sleep(300)

	T("-f", "/dev/null", "new-session", "-d", "-s", "w", "-x", "230", "-y", "68")
	wa := T("display-message", "-p", "-t", "w:", "#{window_id}")
	T("split-window", "-h", "-t", wa)
	T("split-window", "-v", "-t", wa)
	T("new-window", "-d", "-t", "w:", "-n", "other")
	wb := T("display-message", "-p", "-t", "w:other", "#{window_id}")

	// carve equality: join a 40-col pane vs split one — same geometry
	base := T("display-message", "-p", "-t", wa, "#{window_layout}")
	T("new-window", "-d", "-t", "w:", "-n", "scratch", "sleep 1000")
	T("split-window", "-d", "-t", "w:scratch", "sleep 1000") // keep scratch alive after join
	sc := strings.Split(T("list-panes", "-t", "w:scratch", "-F", "#{pane_id}"), "\n")[0]
	T("join-pane", "-hb", "-f", "-l", "40", "-s", sc, "-t", wa)
	joinL := T("display-message", "-p", "-t", wa, "#{window_layout}")
	T("join-pane", "-d", "-s", sc, "-t", "w:scratch")
	T("select-layout", "-t", wa, base)
	T("split-window", "-d", "-hb", "-f", "-l", "40", "-t", wa, "sleep 1000")
	splitL := T("display-message", "-p", "-t", wa, "#{window_layout}")
	if g, w := geometry(joinL), geometry(splitL); g != w {
		t.Errorf("carve arithmetic diverged:\n join: %s\nsplit: %s", g, w)
	}

	// {top-left} addressing
	tl := T("display-message", "-p", "-t", wa+".{top-left}", "#{pane_id} #{pane_width}")
	if !strings.HasSuffix(tl, " 40") {
		t.Errorf("{top-left} is not the 40-col spacer: %q", tl)
	}

	// swap-pane geometry-free on both windows
	T("split-window", "-d", "-hb", "-f", "-l", "40", "-t", wb, "sleep 1000")
	sa := strings.Fields(T("display-message", "-p", "-t", wa+".{top-left}", "#{pane_id}"))[0]
	sb := strings.Fields(T("display-message", "-p", "-t", wb+".{top-left}", "#{pane_id}"))[0]
	la, lb := T("display-message", "-p", "-t", wa, "#{window_layout}"), T("display-message", "-p", "-t", wb, "#{window_layout}")
	T("swap-pane", "-d", "-s", sa, "-t", sb)
	la2, lb2 := T("display-message", "-p", "-t", wa, "#{window_layout}"), T("display-message", "-p", "-t", wb, "#{window_layout}")
	if geometry(la) != geometry(la2) || geometry(lb) != geometry(lb2) {
		t.Errorf("swap-pane moved geometry:\nA %s -> %s\nB %s -> %s", la, la2, lb, lb2)
	}

	// swap with zoomed source auto-unzooms
	T("resize-pane", "-Z", "-t", sb) // sb now lives in wa
	T("swap-pane", "-d", "-s", sb, "-t", sa)
	if T("display-message", "-p", "-t", wa, "#{window_zoomed_flag}") != "0" {
		t.Error("swap from zoom did not unzoom")
	}
}

// geometry strips the checksum and every pane id from a layout string,
// leaving pure rects.
func geometry(layout string) string {
	if i := strings.Index(layout, ","); i >= 0 {
		layout = layout[i+1:]
	}
	re := regexp.MustCompile(`,\d+([,\}\]])`)
	for {
		next := re.ReplaceAllString(layout, "$1")
		if next == layout {
			return regexp.MustCompile(`,\d+$`).ReplaceAllString(next, "")
		}
		layout = next
	}
}

func tmuxRaw(sock string, args ...string) (string, error) {
	return (&Rig{L: sock}).TQ(args...)
}
