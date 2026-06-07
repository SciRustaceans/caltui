package tui

import (
	"fmt"
	"strings"
	"testing"
)

func TestWindowLines(t *testing.T) {
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = fmt.Sprintf("line%d", i)
	}

	// Short list (fits) is returned unchanged.
	if got := windowLines(lines[:6], 0, 10); len(got) != 6 {
		t.Errorf("short list should be unchanged, got %d", len(got))
	}

	check := func(focus, height int) {
		out := windowLines(lines, focus, height)
		if len(out) != height {
			t.Errorf("focus=%d: window size %d, want %d", focus, len(out), height)
		}
		if !strings.Contains(strings.Join(out, "\n"), fmt.Sprintf("line%d", focus)) {
			t.Errorf("focus=%d: focused line must stay visible:\n%s", focus, strings.Join(out, "\n"))
		}
	}
	check(0, 10)  // top
	check(15, 10) // middle
	check(29, 10) // bottom

	// Middle window shows both scroll indicators.
	mid := strings.Join(windowLines(lines, 15, 10), "\n")
	if !strings.Contains(mid, "↑") || !strings.Contains(mid, "↓") {
		t.Errorf("middle window should show up and down indicators:\n%s", mid)
	}
}
