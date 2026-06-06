package charts

import (
	"strings"
	"testing"
)

func TestSparkline(t *testing.T) {
	if got := Sparkline(nil); got != "" {
		t.Errorf("empty input = %q, want empty", got)
	}
	s := Sparkline([]float64{1, 2, 3, 4})
	if r := []rune(s); len(r) != 4 {
		t.Fatalf("len = %d, want 4", len(r))
	} else if r[0] >= r[3] {
		t.Errorf("expected ascending sparkline, got %q", s)
	}
	flat := []rune(Sparkline([]float64{5, 5, 5}))
	if flat[0] != flat[1] || flat[1] != flat[2] {
		t.Errorf("flat data should be uniform, got %q", string(flat))
	}
}

func TestLineChart(t *testing.T) {
	if got := LineChart([]float64{1}, 5, 20, 0); got != "" {
		t.Errorf("single point should render empty, got %q", got)
	}
	out := LineChart([]float64{1, 2, 3, 4, 5}, 5, 20, 0)
	if out == "" || !strings.Contains(out, "\n") {
		t.Errorf("expected a multi-line chart, got %q", out)
	}
}
