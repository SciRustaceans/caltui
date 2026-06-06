// Package nutrition holds pure nutrition/fitness math: BMR/TDEE, calorie
// targets, macro splits, and unit conversions. Every function is deterministic
// and free of I/O, so the package is exhaustively table-testable.
package nutrition

import (
	"time"

	"caltui/internal/domain"
)

// EnergyPerKg approximates the energy content of a kilogram of body mass. It is
// the standard 7700 kcal/kg figure used to translate a weekly rate of weight
// change into a daily calorie delta (a linear approximation).
const EnergyPerKg = 7700.0

// BMR computes basal metabolic rate (kcal/day) using the Mifflin-St Jeor
// equation. weightKg and heightCm are in metric units; ageYears is whole years.
// Unspecified/other sex uses the male constant (the wizard lets the user pick).
func BMR(sex domain.Sex, weightKg, heightCm float64, ageYears int) float64 {
	base := 10*weightKg + 6.25*heightCm - 5*float64(ageYears)
	if sex == domain.Female {
		return base - 161
	}
	return base + 5
}

// TDEE is total daily energy expenditure: BMR scaled by an activity multiplier.
func TDEE(bmr float64, a domain.ActivityLevel) float64 {
	return bmr * a.Multiplier()
}

// TargetResult is a daily calorie target. The target is the raw value implied
// by TDEE and the chosen rate (no safety floor/cap), clamped only to stay
// non-negative.
type TargetResult struct {
	Kcal float64 // daily calorie target (>= 0)
	Raw  float64 // unclamped target, for reference
}

// CalorieTarget converts a TDEE and a signed weekly weight-change rate (kg/week;
// negative = loss, positive = gain) into a daily calorie target:
// target = TDEE + rate * 7700 / 7. No artificial floor or cap is applied; the
// only guard is that the target cannot go negative.
func CalorieTarget(tdee, rateKgPerWeek float64) TargetResult {
	raw := tdee + rateKgPerWeek*EnergyPerKg/7.0
	target := raw
	if target < 0 {
		target = 0
	}
	return TargetResult{Kcal: target, Raw: raw}
}

// AgeFromDate returns whole years between a YYYY-MM-DD birth date and ref.
func AgeFromDate(birthDate string, ref time.Time) (int, error) {
	b, err := time.Parse("2006-01-02", birthDate)
	if err != nil {
		return 0, err
	}
	years := ref.Year() - b.Year()
	// Subtract a year if this year's birthday hasn't happened yet.
	if ref.Month() < b.Month() || (ref.Month() == b.Month() && ref.Day() < b.Day()) {
		years--
	}
	if years < 0 {
		years = 0
	}
	return years, nil
}

// BirthDateForAge converts a known age into an approximate birth date (Jan 1 of
// the implied birth year), for storing a reproducible goal when the user enters
// an age rather than a birthday.
func BirthDateForAge(ageYears int, ref time.Time) string {
	return time.Date(ref.Year()-ageYears, time.January, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}
