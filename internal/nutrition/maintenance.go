package nutrition

import (
	"time"

	"caltui/internal/domain"
)

// MaintenanceWindowDays is the default look-back for the adaptive estimate.
const MaintenanceWindowDays = 21

// EstimateMaintenance infers maintenance calories from logged intake versus the
// observed weight trend over the last windowDays ending at today.
//
// Energy balance: over the window the user ate avgIntake/day and changed weight
// by Δkg; that change represents a daily surplus/deficit of Δkg*7700/days, so
// maintenance = avgIntake - dailyImbalance. ok is false when there isn't enough
// data to trust the estimate (too few logged days, too few weigh-ins, or too
// short a span).
func EstimateMaintenance(intakeByDay map[string]float64, weights []domain.Weight, today string, windowDays int) (float64, bool) {
	end, err := time.Parse("2006-01-02", today)
	if err != nil {
		return 0, false
	}
	start := end.AddDate(0, 0, -(windowDays - 1))
	inWindow := func(date string) bool {
		d, err := time.Parse("2006-01-02", date)
		if err != nil {
			return false
		}
		return !d.Before(start) && !d.After(end)
	}

	// Average intake over logged (positive) days in the window.
	var sum float64
	var logged int
	for date, kcal := range intakeByDay {
		if kcal > 0 && inWindow(date) {
			sum += kcal
			logged++
		}
	}
	if logged < 10 {
		return 0, false
	}
	avgIntake := sum / float64(logged)

	// First and last weigh-ins within the window (weights are oldest-first).
	var first, last domain.Weight
	var haveFirst bool
	for _, w := range weights {
		if !inWindow(w.Date) {
			continue
		}
		if !haveFirst {
			first, haveFirst = w, true
		}
		last = w
	}
	if !haveFirst || first.Date == last.Date {
		return 0, false
	}
	d1, _ := time.Parse("2006-01-02", first.Date)
	d2, _ := time.Parse("2006-01-02", last.Date)
	daysSpan := d2.Sub(d1).Hours() / 24
	if daysSpan < 7 {
		return 0, false
	}

	dailyImbalance := (last.Kg - first.Kg) * EnergyPerKg / daysSpan
	maintenance := avgIntake - dailyImbalance
	if maintenance <= 0 {
		return 0, false
	}
	return maintenance, true
}
