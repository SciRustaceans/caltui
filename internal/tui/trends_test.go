package tui

import (
	"strings"
	"testing"

	"caltui/internal/domain"
)

func TestTrendsView(t *testing.T) {
	m := sampleModel()
	m.active = tabTrends
	m.weekKcal = []float64{1800, 2000, 1950, 2100, 1700, 2200, 1420}
	m.trendKcal = make([]float64, trendDays)
	for i := range m.trendKcal {
		m.trendKcal[i] = 1800 + float64(i*10)
	}
	m.weights = []domain.Weight{
		{Date: "2026-05-30", Kg: 83.0, Unit: "kg"},
		{Date: "2026-06-02", Kg: 82.4, Unit: "kg"},
		{Date: "2026-06-06", Kg: 81.6, Unit: "kg"},
	}
	out := m.viewTrends(100)
	for _, want := range []string{"Trends", "Calories — this week", "Weight", "Recent days", "Mon", "projected"} {
		if !strings.Contains(out, want) {
			t.Errorf("trends view missing %q", want)
		}
	}
}

func TestWeeklyBarsColorByGoal(t *testing.T) {
	m := sampleModel() // goal 2100 kcal
	m.weekKcal = []float64{1500, 2500, 0, 2100, 1800, 2300, 1900}
	out := m.weeklyBars()
	// 7 day labels + a goal note line.
	if lines := strings.Count(out, "\n"); lines < 7 {
		t.Errorf("expected at least 7 bar lines, got %d", lines)
	}
	if !strings.Contains(out, "goal 2100") {
		t.Errorf("missing goal annotation:\n%s", out)
	}
}

func TestWeightProjection(t *testing.T) {
	m := sampleModel()
	// Losing ~0.5 kg over 7 days = 0.5 kg/week.
	m.weights = []domain.Weight{
		{Date: "2026-05-30", Kg: 82.4, Unit: "kg"},
		{Date: "2026-06-06", Kg: 81.9, Unit: "kg"},
	}
	m.hasWeightGoal = true
	m.weightGoal = domain.WeightGoal{TargetKg: 78, Unit: "kg", RatePerWeek: -0.5, StartKg: 82.4}

	proj, weekly, weeks, ok := m.weightProjection()
	if !ok {
		t.Fatal("expected a projection")
	}
	if weekly >= 0 {
		t.Errorf("weekly rate should be negative (losing), got %g", weekly)
	}
	if len(proj) == 0 || proj[len(proj)-1] >= proj[0] {
		t.Errorf("projection should trend downward: %v", proj)
	}
	if weeks <= 0 {
		t.Errorf("expected positive weeks-to-goal, got %g", weeks)
	}
}

func TestWeightProjectionFlatSkipped(t *testing.T) {
	m := sampleModel()
	m.weights = []domain.Weight{
		{Date: "2026-05-30", Kg: 82.0, Unit: "kg"},
		{Date: "2026-06-06", Kg: 82.0, Unit: "kg"},
	}
	if _, _, _, ok := m.weightProjection(); ok {
		t.Error("flat weight should not produce a projection")
	}
}

func TestGoalOptionsMirrorTo3(t *testing.T) {
	var minRate, maxRate float64
	for _, o := range goalOptions {
		if o.rate < minRate {
			minRate = o.rate
		}
		if o.rate > maxRate {
			maxRate = o.rate
		}
	}
	if minRate != -3.0 {
		t.Errorf("expected a -3 kg/week loss option, min = %g", minRate)
	}
	if maxRate != 3.0 {
		t.Errorf("gain should mirror loss to +3 kg/week, max = %g", maxRate)
	}
	if goalOptions[defaultGoalIdx()].rate != 0 {
		t.Errorf("default goal option should be maintain (rate 0)")
	}
	// Loss and gain must mirror at 1, 1.5, 2, 2.5, 3.
	want := map[float64]bool{
		-1: false, -1.5: false, -2: false, -2.5: false, -3: false,
		1: false, 1.5: false, 2: false, 2.5: false, 3: false,
	}
	for _, o := range goalOptions {
		if _, ok := want[o.rate]; ok {
			want[o.rate] = true
		}
	}
	for r, present := range want {
		if !present {
			t.Errorf("missing option for %g kg/week", r)
		}
	}
}
