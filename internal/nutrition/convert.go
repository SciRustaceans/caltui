package nutrition

import (
	"fmt"
	"math"

	"caltui/internal/domain"
)

// Physical constants for unit conversion.
const (
	GramsPerOunce = 28.3495
	KgPerPound    = 0.45359237
	KcalPerKJ     = 1.0 / 4.184 // 1 kcal = 4.184 kJ

	// KcalReconcileTolerance is the relative gap between label energy and
	// 4/4/9-computed energy above which we flag a food for review.
	KcalReconcileTolerance = 0.15
)

// servingGrams returns the mass in grams of one serving of f, or 0 if it cannot
// be determined (e.g. a serving defined only as a "piece" with no gram weight).
func servingGrams(f domain.Food) float64 {
	switch f.ServingUnit {
	case domain.UnitGram:
		return f.ServingSize
	case domain.UnitOunce:
		return f.ServingSize * GramsPerOunce
	case domain.UnitMilliliter:
		return f.ServingSize * density(f)
	default:
		return 0
	}
}

func density(f domain.Food) float64 {
	if f.Density > 0 {
		return f.Density
	}
	return 1.0 // water-like default
}

// ToGrams converts a quantity in the given unit to grams, using the food's
// density (for ml) or serving size (for serving/piece) where required.
func ToGrams(qty float64, unit domain.Unit, f domain.Food) (float64, error) {
	switch unit {
	case domain.UnitGram:
		return qty, nil
	case domain.UnitOunce:
		return qty * GramsPerOunce, nil
	case domain.UnitMilliliter:
		return qty * density(f), nil
	case domain.UnitServing, domain.UnitPiece:
		g := servingGrams(f)
		if g <= 0 {
			return 0, fmt.Errorf("food %q has no serving weight in grams; log it in grams", f.Name)
		}
		return qty * g, nil
	default:
		return 0, fmt.Errorf("unknown unit %q", unit)
	}
}

// MacrosFor computes the macros contributed by qty units of food f. Quantity
// must be positive.
func MacrosFor(f domain.Food, qty float64, unit domain.Unit) (domain.Macros, error) {
	if qty <= 0 {
		return domain.Macros{}, fmt.Errorf("quantity must be positive, got %g", qty)
	}
	grams, err := ToGrams(qty, unit, f)
	if err != nil {
		return domain.Macros{}, err
	}
	return f.Per100g.Scale(grams / 100.0), nil
}

// PerUnitMacros computes the macros for exactly one unit of f, the snapshot
// stored on a LogEntry so totals are just PerUnit * Quantity.
func PerUnitMacros(f domain.Food, unit domain.Unit) (domain.Macros, error) {
	return MacrosFor(f, 1, unit)
}

// Per100gFromServing builds a per-100g Macros from per-serving values and a
// serving size, used when normalizing branded foods that report per serving.
// servingSizeGrams must be a positive gram weight.
func Per100gFromServing(perServing domain.Macros, servingSizeGrams float64) (domain.Macros, error) {
	if servingSizeGrams <= 0 {
		return domain.Macros{}, fmt.Errorf("serving size must be a positive gram weight, got %g", servingSizeGrams)
	}
	return perServing.Scale(100.0 / servingSizeGrams), nil
}

// KcalMismatch reports whether label energy and 4/4/9-computed energy differ by
// more than KcalReconcileTolerance. It also returns the computed energy. When
// they disagree, callers should trust the label Kcal but may surface a warning.
func KcalMismatch(m domain.Macros) (mismatch bool, computed float64) {
	computed = m.ComputedKcal()
	if m.Kcal <= 0 {
		return false, computed
	}
	return math.Abs(m.Kcal-computed)/m.Kcal > KcalReconcileTolerance, computed
}

// LbToKg converts pounds to kilograms.
func LbToKg(lb float64) float64 { return lb * KgPerPound }

// KgToLb converts kilograms to pounds.
func KgToLb(kg float64) float64 { return kg / KgPerPound }

// KJToKcal converts kilojoules to kilocalories (for energy values that arrive
// in kJ from a data source).
func KJToKcal(kj float64) float64 { return kj * KcalPerKJ }
