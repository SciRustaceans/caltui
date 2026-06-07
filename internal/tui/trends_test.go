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

func TestGoalOptionsMirror(t *testing.T) {
	var minRate, maxRate float64
	has := map[float64]bool{}
	for _, o := range goalOptions {
		if o.rate < minRate {
			minRate = o.rate
		}
		if o.rate > maxRate {
			maxRate = o.rate
		}
		has[o.rate] = true
	}
	if minRate != -2.5 || maxRate != 2.5 {
		t.Errorf("rates should span ±2.5, got [%g, %g]", minRate, maxRate)
	}
	if has[-3] || has[3] {
		t.Errorf("3 kg/week options should have been removed")
	}
	if !has[-0.75] || !has[0.75] {
		t.Errorf("0.75 kg/week options should exist (lose %v, gain %v)", has[-0.75], has[0.75])
	}
	if goalOptions[defaultGoalIdx()].rate != 0 {
		t.Errorf("default goal option should be maintain (rate 0)")
	}
}
