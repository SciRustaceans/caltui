package nutrition

import (
	"testing"
	"time"

	"caltui/internal/domain"
)

func mustDate(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

// intake builds a map of n consecutive days ending at end, each with kcal.
func intakeDays(end string, n int, kcal float64) map[string]float64 {
	m := make(map[string]float64)
	t := mustDate(end)
	for i := 0; i < n; i++ {
		m[t.AddDate(0, 0, -i).Format("2006-01-02")] = kcal
	}
	return m
}

func TestEstimateMaintenanceLosing(t *testing.T) {
	// Ate 2000/day for 21 days and lost 1 kg over the 14-day weigh-in span.
	intake := intakeDays("2026-06-21", 21, 2000)
	weights := []domain.Weight{
		{Date: "2026-06-07", Kg: 82.0},
		{Date: "2026-06-21", Kg: 81.0},
	}
	got, ok := EstimateMaintenance(intake, weights, "2026-06-21", 21)
	if !ok {
		t.Fatal("expected a valid estimate")
	}
	// dailyImbalance = -1*7700/14 = -550; maintenance = 2000 - (-550) = 2550.
	if got < 2540 || got > 2560 {
		t.Errorf("maintenance = %g, want ~2550 (losing => maintenance > intake)", got)
	}
}

func TestEstimateMaintenanceGaining(t *testing.T) {
	intake := intakeDays("2026-06-21", 21, 3000)
	weights := []domain.Weight{
		{Date: "2026-06-07", Kg: 80.0},
		{Date: "2026-06-21", Kg: 81.4}, // +1.4 kg / 14 d
	}
	got, ok := EstimateMaintenance(intake, weights, "2026-06-21", 21)
	if !ok {
		t.Fatal("expected a valid estimate")
	}
	// +1.4*7700/14 = +770 surplus; maintenance = 3000 - 770 = 2230.
	if got < 2220 || got > 2240 {
		t.Errorf("maintenance = %g, want ~2230 (gaining => maintenance < intake)", got)
	}
}

func TestEstimateMaintenanceInsufficientData(t *testing.T) {
	// Too few logged days.
	if _, ok := EstimateMaintenance(intakeDays("2026-06-21", 5, 2000),
		[]domain.Weight{{Date: "2026-06-07", Kg: 82}, {Date: "2026-06-21", Kg: 81}}, "2026-06-21", 21); ok {
		t.Error("should be invalid with <10 logged days")
	}
	// Only one weigh-in.
	if _, ok := EstimateMaintenance(intakeDays("2026-06-21", 21, 2000),
		[]domain.Weight{{Date: "2026-06-21", Kg: 81}}, "2026-06-21", 21); ok {
		t.Error("should be invalid with a single weigh-in")
	}
	// Weigh-ins too close together (span < 7 days).
	if _, ok := EstimateMaintenance(intakeDays("2026-06-21", 21, 2000),
		[]domain.Weight{{Date: "2026-06-19", Kg: 81.1}, {Date: "2026-06-21", Kg: 81}}, "2026-06-21", 21); ok {
		t.Error("should be invalid with <7 day weigh-in span")
	}
}
