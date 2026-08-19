// Package rigs is demux's integration test harness. Each test gets an
// ISOLATED tmux server (socket named after the test + pid), a fake attached
// client on a real pty, and a daemon — so the whole suite runs in parallel
// and never touches your real tmux.
//
//	go test ./...                # full suite, parallel
//	go test -run TestEqualize    # one test
//	go test -short               # skip the slow big-scrollback test
//	go test -v                   # per-assert output
package rigs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

var (
	demuxdBin   string
	equalizeBin string
)

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "demux-rig-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	demuxdBin = filepath.Join(tmp, "demuxd")
	equalizeBin = filepath.Join(tmp, "equalize")
	for dir, out := range map[string]string{
		"../demuxd":                demuxdBin,
		"../../tmux-equalize-nvim": equalizeBin,
	} {
		cmd := exec.Command("go", "build", "-o", out, ".")
		cmd.Dir = dir
		if b, err := cmd.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "build %s: %v\n%s", dir, err, b)
			os.Exit(1)
		}
	}
	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

// Rig is one isolated tmux world. Standard shape: session work (w1 = a
// split holding a MARKW1 loop pane, beta, gamma), session play (2 windows),
// a fake 200x50 client attached to work:beta, daemon running.
type Rig struct {
	t *testing.T
	L string // tmux socket name

	W1, W2, W3, P1 string // window ids
	LW1, LW2, LW3  string // pre-dock layout baselines
	CL             string // fake client name
	Sock           string // demux socket path (log at Sock+".log")

	clientCmd *exec.Cmd
	ptyMaster *os.File
}

func New(t *testing.T) *Rig {
	t.Parallel()
	name := regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAllString(t.Name(), "")
	r := &Rig{t: t, L: strings.ToLower(name) + strconv.Itoa(os.Getpid())}
	t.Cleanup(r.teardown)
	r.setup()
	return r
}

func (r *Rig) teardown() {
	r.TQ("kill-server")
	// By socket path, not binary name: catches the daemon, the TUI it
	// spawned, and stragglers from a crashed earlier run of this test.
	exec.Command("pkill", "-f", "/private/tmp/tmux-501/"+r.L).Run()
	if r.clientCmd != nil && r.clientCmd.Process != nil {
		r.clientCmd.Process.Kill()
	}
	if r.ptyMaster != nil {
		r.ptyMaster.Close()
	}
	if r.t != nil && r.t.Failed() {
		r.t.Logf("keeping daemon log: %s.log", r.Sock)
		return
	}
	for _, f := range glob("/tmp/demux-501/" + r.L + "-*") {
		os.Remove(f)
	}
}

func glob(pat string) []string { m, _ := filepath.Glob(pat); return m }

// envSansTmux strips TMUX/TMUX_PANE so nothing ever looks nested.
func envSansTmux() []string {
	var out []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "TMUX=") || strings.HasPrefix(e, "TMUX_PANE=") {
			continue
		}
		out = append(out, e)
	}
	return out
}

// T runs a tmux command against this rig's server; fatal on error.
func (r *Rig) T(args ...string) string {
	r.t.Helper()
	out, err := r.TQ(args...)
	if err != nil {
		r.t.Fatalf("tmux %v: %v\n%s", args, err, out)
	}
	return out
}

// TQ is T but quiet: returns the error instead of failing the test.
func (r *Rig) TQ(args ...string) (string, error) {
	cmd := exec.Command("tmux", append([]string{"-L", r.L}, args...)...)
	cmd.Env = envSansTmux()
	b, err := cmd.CombinedOutput()
	return strings.TrimRight(string(b), "\n"), err
}

// D runs the demuxd client binary against this rig's server. DEMUX_BENCH
// rides along so the (auto-spawned) daemon logs its µs instrumentation —
// free diagnostics whenever a rig fails.
func (r *Rig) D(args ...string) string {
	r.t.Helper()
	cmd := exec.Command(demuxdBin, append([]string{"-L", r.L}, args...)...)
	cmd.Env = append(envSansTmux(), "DEMUX_BENCH=1")
	b, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("demuxd %v: %v\n%s", args, err, b)
	}
	return strings.TrimRight(string(b), "\n")
}

func (r *Rig) setup() {
	r.t.Helper()
	r.teardown()
	sleep(300)
	r.T("-f", "/dev/null", "new-session", "-d", "-s", "work", "-x", "200", "-y", "50")
	r.W1 = r.T("display-message", "-p", "-t", "work:", "#{window_id}")
	r.T("split-window", "-h", "-t", r.W1, "while :; do echo MARKW1; sleep 2; done")
	r.T("new-window", "-t", "work:", "-n", "beta")
	r.T("new-window", "-t", "work:", "-n", "gamma")
	for _, ln := range strings.Split(r.T("list-windows", "-t", "work", "-F", "#{window_id} #{window_name}"), "\n") {
		f := strings.Fields(ln)
		if len(f) != 2 {
			continue
		}
		switch f[1] {
		case "beta":
			r.W2 = f[0]
		case "gamma":
			r.W3 = f[0]
		}
	}
	r.T("new-session", "-d", "-s", "play", "-x", "200", "-y", "50")
	r.P1 = r.T("display-message", "-p", "-t", "play:", "#{window_id}")
	r.T("new-window", "-t", "play:", "-n", "ptwo")
	r.T("select-window", "-t", r.W2)
	sleep(300)
	r.attachFakeClient()
	sleep(1000)
	// Baselines AFTER attach, each while ITS window is current — the
	// client's status line resizes only the current window (window-size
	// latest).
	r.T("select-window", "-t", r.W1)
	sleep(200)
	r.LW1 = r.T("display-message", "-p", "-t", r.W1, "#{window_layout}")
	r.T("select-window", "-t", r.W3)
	sleep(200)
	r.LW3 = r.T("display-message", "-p", "-t", r.W3, "#{window_layout}")
	r.T("select-window", "-t", r.W2)
	sleep(200)
	r.LW2 = r.T("display-message", "-p", "-t", r.W2, "#{window_layout}")
	for _, ln := range strings.Split(r.T("list-clients", "-F", "#{client_name} #{client_control_mode}"), "\n") {
		f := strings.Fields(ln)
		if len(f) == 2 && f[1] == "0" {
			r.CL = f[0]
			break
		}
	}
	if r.CL == "" {
		r.t.Fatal("no fake client attached")
	}
	r.D("ls")
	sleep(500)
	for _, ln := range strings.Split(r.D("sock"), "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "demux:") {
			r.Sock = strings.Fields(ln)[1]
		}
	}
}

// attachFakeClient attaches a real (non-control) 200x50 tmux client on a pty.
func (r *Rig) attachFakeClient() {
	r.t.Helper()
	master, slave, err := openPty(50, 200)
	if err != nil {
		r.t.Fatalf("openPty: %v", err)
	}
	cmd := exec.Command("tmux", "-L", r.L, "attach", "-t", "work")
	cmd.Env = append(envSansTmux(), "TERM=xterm-256color")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	cmd.SysProcAttr = sysProcAttrTTY()
	if err := cmd.Start(); err != nil {
		r.t.Fatalf("attach client: %v", err)
	}
	slave.Close()
	go func() { // drain so tmux never blocks writing to the client
		buf := make([]byte, 65536)
		for {
			if _, err := master.Read(buf); err != nil {
				return
			}
		}
	}()
	r.clientCmd, r.ptyMaster = cmd, master
}

// ---- assertions & helpers ----

func sleep(ms int) { time.Sleep(time.Duration(ms) * time.Millisecond) }

func (r *Rig) Chk(name string, cond bool) {
	r.t.Helper()
	if cond {
		r.t.Logf("ok - %s", name)
	} else {
		r.t.Errorf("FAIL - %s", name)
	}
}

// WaitUntil polls f at 10ms up to tries times.
func (r *Rig) WaitUntil(tries int, f func() bool) bool {
	for i := 0; i < tries; i++ {
		if f() {
			return true
		}
		sleep(10)
	}
	return false
}

// Sidebar is the demux TUI pane, wherever it currently lives.
type Sidebar struct {
	Pane, Win           string
	Left, Width, Active int
}

func (r *Rig) Side() Sidebar {
	out := r.T("list-panes", "-a", "-F",
		"#{pane_id} #{pane_current_command} #{window_id} #{pane_left} #{pane_width} #{pane_active}")
	for _, ln := range strings.Split(out, "\n") {
		f := strings.Fields(ln)
		if len(f) == 6 && strings.Contains(f[1], "demux") {
			left, _ := strconv.Atoi(f[3])
			width, _ := strconv.Atoi(f[4])
			active, _ := strconv.Atoi(f[5])
			return Sidebar{Pane: f[0], Win: f[2], Left: left, Width: width, Active: active}
		}
	}
	return Sidebar{}
}

func (r *Rig) ClientWin() string {
	for _, ln := range strings.Split(r.T("list-clients", "-F", "#{client_name} #{window_id}"), "\n") {
		f := strings.Fields(ln)
		if len(f) == 2 && f[0] == r.CL {
			return f[1]
		}
	}
	return ""
}

func (r *Rig) ClientSess() string {
	for _, ln := range strings.Split(r.T("list-clients", "-F", "#{client_name} #{session_name}"), "\n") {
		f := strings.Fields(ln)
		if len(f) == 2 && f[0] == r.CL {
			return f[1]
		}
	}
	return ""
}

func (r *Rig) Zoomed(win string) bool {
	return r.T("display-message", "-p", "-t", win, "#{window_zoomed_flag}") == "1"
}

// Spacers counts spacer panes server-wide (their distinctive start command;
// tmux 3.7 wraps #{pane_start_command} in double quotes).
func (r *Rig) Spacers() int {
	n := 0
	for _, ln := range strings.Split(r.T("list-panes", "-a", "-F", "#{pane_start_command}"), "\n") {
		if strings.Trim(ln, `"`) == "sleep 100000001" {
			n++
		}
	}
	return n
}

// Layout returns a window's layout with the checksum prefix stripped.
func (r *Rig) Layout(win string) string {
	l := r.T("display-message", "-p", "-t", win, "#{window_layout}")
	if i := strings.Index(l, ","); i >= 0 {
		return l[i+1:]
	}
	return l
}

func (r *Rig) SendKeys(pane string, keys ...string) {
	r.T(append([]string{"send-keys", "-t", pane}, keys...)...)
}

func (r *Rig) Capture(pane string) string {
	out, _ := r.TQ("capture-pane", "-p", "-t", pane)
	return out
}

// ShowOpt returns a session/window option's value ("" when unset — tmux
// errors on unset user options, which counts as unset here).
func (r *Rig) ShowOpt(flags ...string) string {
	out, err := r.TQ(append([]string{"show-options"}, flags...)...)
	if err != nil {
		return ""
	}
	return out
}

// LogHas reports whether the daemon log contains pat.
func (r *Rig) LogHas(pat string) bool {
	b, err := os.ReadFile(r.Sock + ".log")
	return err == nil && regexp.MustCompile(pat).Match(b)
}

// DemuxPanes counts demux TUI panes inside the given target's panes.
func (r *Rig) DemuxPanes(flags ...string) int {
	out, err := r.TQ(append(append([]string{"list-panes"}, flags...), "-F", "#{pane_current_command}")...)
	if err != nil {
		return 0
	}
	n := 0
	for _, ln := range strings.Split(out, "\n") {
		if strings.Contains(ln, "demux") {
			n++
		}
	}
	return n
}
