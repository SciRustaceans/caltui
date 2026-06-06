// Package domain holds the core data types and small value-object behavior for
// caltui. It has no I/O and no dependencies on other internal packages, so it
// can be imported freely by the store, nutrition, food, and tui layers.
package domain

import (
	"fmt"
	"strings"
)

// Sex is the biological sex used by the Mifflin-St Jeor BMR equation.
type Sex string

const (
	Male   Sex = "male"
	Female Sex = "female"
)

// Valid reports whether s is a known sex.
func (s Sex) Valid() bool { return s == Male || s == Female }

// ActivityLevel is a named TDEE activity multiplier bucket.
type ActivityLevel string

const (
	Sedentary        ActivityLevel = "sedentary"
	LightlyActive    ActivityLevel = "lightly_active"
	ModeratelyActive ActivityLevel = "moderately_active"
	VeryActive       ActivityLevel = "very_active"
	ExtraActive      ActivityLevel = "extra_active"
)

// ActivityLevels lists the levels in ascending order, for menus.
var ActivityLevels = []ActivityLevel{
	Sedentary, LightlyActive, ModeratelyActive, VeryActive, ExtraActive,
}

// Multiplier returns the factor applied to BMR to get TDEE.
func (a ActivityLevel) Multiplier() float64 {
	switch a {
	case Sedentary:
		return 1.2
	case LightlyActive:
		return 1.375
	case ModeratelyActive:
		return 1.55
	case VeryActive:
		return 1.725
	case ExtraActive:
		return 1.9
	default:
		return 0
	}
}

// Label returns a human-friendly description for menus.
func (a ActivityLevel) Label() string {
	switch a {
	case Sedentary:
		return "Sedentary (little/no exercise)"
	case LightlyActive:
		return "Lightly active (1-3 days/week)"
	case ModeratelyActive:
		return "Moderately active (3-5 days/week)"
	case VeryActive:
		return "Very active (6-7 days/week)"
	case ExtraActive:
		return "Extra active (hard daily/physical job)"
	default:
		return string(a)
	}
}

// Valid reports whether a is a known activity level.
func (a ActivityLevel) Valid() bool { return a.Multiplier() != 0 }

// ActivityLevelForMultiplier reverse-maps a stored factor to its level. It is
// used when loading goals, which persist the float factor. The second return is
// false if no level matches exactly.
func ActivityLevelForMultiplier(m float64) (ActivityLevel, bool) {
	for _, a := range ActivityLevels {
		if a.Multiplier() == m {
			return a, true
		}
	}
	return "", false
}

// Meal is a section of the daily diary.
type Meal string

const (
	Breakfast Meal = "breakfast"
	Lunch     Meal = "lunch"
	Dinner    Meal = "dinner"
	Snacks    Meal = "snacks"
)

// MealsInOrder lists meals in the order they appear in the diary.
var MealsInOrder = []Meal{Breakfast, Lunch, Dinner, Snacks}

// Title returns a capitalized label, e.g. "Breakfast".
func (m Meal) Title() string {
	if m == "" {
		return ""
	}
	return strings.ToUpper(string(m[:1])) + string(m[1:])
}

// Valid reports whether m is a known meal.
func (m Meal) Valid() bool {
	switch m {
	case Breakfast, Lunch, Dinner, Snacks:
		return true
	default:
		return false
	}
}

// ParseMeal parses a meal name case-insensitively.
func ParseMeal(s string) (Meal, error) {
	m := Meal(strings.ToLower(strings.TrimSpace(s)))
	if !m.Valid() {
		return "", fmt.Errorf("unknown meal %q", s)
	}
	return m, nil
}

// MealForHour picks a sensible default meal slot for a 24h clock hour, so the
// add-food modal can pre-select where the user is most likely logging.
func MealForHour(hour int) Meal {
	switch {
	case hour < 11:
		return Breakfast
	case hour < 15:
		return Lunch
	case hour < 21:
		return Dinner
	default:
		return Snacks
	}
}

// FoodSource records where a food came from.
type FoodSource string

const (
	SourceOfflineUSDA FoodSource = "usda_offline"
	SourceOnlineUSDA  FoodSource = "usda_online"
	SourceCustom      FoodSource = "custom"
)

// Unit is a logging unit. Grams are the canonical mass unit; everything is
// converted to grams (using a food's serving size / density when needed) before
// computing macros.
type Unit string

const (
	UnitGram       Unit = "g"
	UnitOunce      Unit = "oz"
	UnitMilliliter Unit = "ml"
	UnitServing    Unit = "serving"
	UnitPiece      Unit = "piece"
)

// Valid reports whether u is a known unit.
func (u Unit) Valid() bool {
	switch u {
	case UnitGram, UnitOunce, UnitMilliliter, UnitServing, UnitPiece:
		return true
	default:
		return false
	}
}
