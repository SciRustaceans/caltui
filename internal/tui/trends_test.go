package tui

import (
	"strings"
	"testing"

	"caltui/internal/domain"
)

func TestTrendsView(t *testing.T) {
	m := sampleModel()
	m.active = tabTrends
	m.trendKcal = make([]float64, trendDays)
	for i := range m.trendKcal {
		m.trendKcal[i] = 1800 + float64(i*10)
	}
	m.weights = []domain.Weight{
		{Date: "2026-06-04", Kg: 82.5, Unit: "kg"},
		{Date: "2026-06-05", Kg: 82.2, Unit: "kg"},
		{Date: "2026-06-06", Kg: 81.9, Unit: "kg"},
	}
	out := m.viewTrends(100)
	for _, want := range []string{"Trends", "Calories", "Weight", "Recent days"} {
		if !strings.Contains(out, want) {
			t.Errorf("trends view missing %q", want)
		}
	}
}

func TestTrimLeadingZeros(t *testing.T) {
	got := trimLeadingZeros([]float64{0, 0, 3, 4, 0})
	if len(got) != 3 || got[0] != 3 {
		t.Errorf("trimLeadingZeros = %v", got)
	}
	if len(trimLeadingZeros([]float64{0, 0})) != 0 {
		t.Errorf("all zeros should trim to empty")
	}
}

func TestHistoryTable(t *testing.T) {
	m := sampleModel()
	m.trendKcal = make([]float64, trendDays)
	m.trendKcal[trendDays-1] = 2300 // today, over the 2100 goal
	out := m.historyTable()
	if !strings.Contains(out, "over") {
		t.Errorf("history table should flag an over-goal day:\n%s", out)
	}
}
