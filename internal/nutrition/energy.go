// Package nutrition holds pure nutrition/fitness math: BMR/TDEE, calorie
// targets, macro splits, and unit conversions. Every function is deterministic
// and free of I/O, so the package is exhaustively table-testable.
package nutrition

import (
	"math"
	"time"

	"caltui/internal/domain"
)

// EnergyPerKg approximates the energy content of a kilogram of body mass. It is
// the standard 7700 kcal/kg figure used to translate a weekly rate of weight
// change into a daily calorie delta. It is a linear approximation, hence the
// clamp + warning machinery in CalorieTarget.
const EnergyPerKg = 7700.0

// Sex-specific minimum daily calorie floors (kcal). Deficits never push the
// target below these.
const (
	floorMale   = 1500.0
	floorFemale = 1200.0
)

// minDeficitFraction caps a deficit at 25% below TDEE (target >= 0.75*TDEE).
const minDeficitFraction = 0.75

// maxSurplusFraction caps a surplus at 20% above TDEE.
const maxSurplusFraction = 1.20

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

// TargetResult is a calorie target plus an explanation of any safety clamp that
// changed it, so the UI can warn the user.
type TargetResult struct {
	Kcal    float64 // the (possibly clamped) daily calorie target
	Raw     float64 // the unclamped target, for reference
	Clamped bool
	Reason  string
}

// CalorieTarget converts a TDEE and a signed weekly weight-change rate (kg/week;
// negative = loss, positive = gain) into a daily calorie target, applying
// safety clamps. For deficits the target is held at or above max(0.75*TDEE,
// sex floor); for surpluses it is capped at 1.20*TDEE.
func CalorieTarget(tdee, rateKgPerWeek float64, sex domain.Sex) TargetResult {
	raw := tdee + rateKgPerWeek*EnergyPerKg/7.0
	res := TargetResult{Kcal: raw, Raw: raw}

	switch {
	case raw < tdee: // deficit
		floor := floorMale
		if sex == domain.Female {
			floor = floorFemale
		}
		lo := math.Max(floor, minDeficitFraction*tdee)
		if raw < lo {
			res.Kcal = lo
			res.Clamped = true
			if lo == floor {
				res.Reason = "raised to the minimum safe daily calories"
			} else {
				res.Reason = "limited to a 25% deficit"
			}
		}
	case raw > tdee: // surplus
		hi := maxSurplusFraction * tdee
		if raw > hi {
			res.Kcal = hi
			res.Clamped = true
			res.Reason = "limited to a 20% surplus"
		}
	}
	return res
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

// BirthYearToDate converts a known age into an approximate birth date (Jan 1 of
// the implied birth year), for storing a reproducible goal when the user enters
// an age rather than a birthday.
func BirthDateForAge(ageYears int, ref time.Time) string {
	return time.Date(ref.Year()-ageYears, time.January, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}
