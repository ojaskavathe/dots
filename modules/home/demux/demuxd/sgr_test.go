package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestSelfContainCarry(t *testing.T) {
	lines := []string{
		"\x1b[48;2;20;20;20m", // fill row, bg left open
		"plain row",           // relies on carry
		"done\x1b[0m",
		"after reset",
	}
	selfContain(lines)
	if !strings.HasPrefix(lines[1], "\x1b[48;2;20;20;20m") {
		t.Errorf("carry not prepended: %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "\x1b[48;2;20;20;20m") {
		t.Errorf("carry lost before reset: %q", lines[2])
	}
	if lines[3] != "after reset" {
		t.Errorf("carry survived a reset: %q", lines[3])
	}
}

// The regression that garbled dense frames: hundreds of color changes above
// a row must fold to ONE fg + ONE bg — never a partial compound.
func TestSgrStateBounded(t *testing.T) {
	var st sgrState
	for i := 0; i < 300; i++ {
		st.fold(fmt.Sprintf("x\x1b[38;2;%d;1;2m\x1b[48;2;%d;3;4my\x1b[1m", i, i))
	}
	seq := st.seq()
	if seq != "\x1b[1;38;2;299;1;2;48;2;299;3;4m" {
		t.Errorf("state not minimal: %q", seq)
	}
}

func TestSgrFlagsAndDefaults(t *testing.T) {
	var st sgrState
	st.fold("\x1b[1m\x1b[4m\x1b[31m\x1b[45m")
	st.fold("\x1b[22m\x1b[49m") // bold off, bg default
	seq := st.seq()
	if strings.Contains(seq, "1;") || strings.Contains(seq, "45") {
		t.Errorf("cancelled attrs kept: %q", seq)
	}
	if !strings.Contains(seq, "4") || !strings.Contains(seq, "31") {
		t.Errorf("live attrs dropped: %q", seq)
	}
}

func TestSgrColonForms(t *testing.T) {
	var st sgrState
	st.fold("\x1b[4:3m\x1b[38:2:10:20:30m")
	seq := st.seq()
	if !strings.Contains(seq, "4:3") || !strings.Contains(seq, "38:2:10:20:30") {
		t.Errorf("colon forms lost: %q", seq)
	}
	st.fold("\x1b[24m\x1b[39m")
	if got := st.seq(); got != "" {
		t.Errorf("colon forms not cancelled: %q", got)
	}
}

func TestSgrMalformedIntroducer(t *testing.T) {
	var st sgrState
	// truncated compounds must not panic or absorb params into a color;
	// the standalone 9 after the skipped introducer is a real strike flag
	st.fold("\x1b[38m\x1b[48;9m")
	if got := st.seq(); got != "\x1b[9m" {
		t.Errorf("malformed handling: %q", got)
	}
}
