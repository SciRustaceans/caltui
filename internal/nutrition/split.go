package nutrition

import "caltui/internal/domain"

// Default macro-split parameters.
const (
	DefaultProteinPerKg = 1.8 // grams protein per kg bodyweight
	DefaultFatPerKg     = 0.9 // grams fat per kg bodyweight
	essentialFatPerKg   = 0.5 // floor when carbs would otherwise go negative

	// Default percentage split (protein/carbs/fat), used by the %-based mode.
	DefaultProteinPct = 30.0
	DefaultCarbsPct   = 40.0
	DefaultFatPct     = 30.0
)

// Split is a computed macro target plus any note about adjustments made to keep
// the numbers physically sensible.
type Split struct {
	Macros   domain.Macros
	Adjusted bool
	Note     string
}

// MacroSplitPerKg derives target grams from a calorie target and bodyweight
// using a grams-per-kg approach: protein and fat are set from bodyweight and
// carbs absorb the remaining calories. If carbs would be negative, fat is
// reduced toward an essential minimum; if still negative, protein is held and
// carbs are pinned to zero (with a warning note).
func MacroSplitPerKg(targetKcal, weightKg, proteinPerKg, fatPerKg float64) Split {
	protein := proteinPerKg * weightKg
	fat := fatPerKg * weightKg
	carbs := (targetKcal - protein*4 - fat*9) / 4

	s := Split{}
	if carbs < 0 {
		s.Adjusted = true
		fat = essentialFatPerKg * weightKg
		carbs = (targetKcal - protein*4 - fat*9) / 4
		if carbs < 0 {
			carbs = 0
			s.Note = "calorie target is too low for this protein level; protein kept, carbs set to 0"
		} else {
			s.Note = "fat reduced to the essential minimum so carbs stay non-negative"
		}
	}
	s.Macros = domain.Macros{Kcal: targetKcal, Protein: protein, Carbs: carbs, Fat: fat}
	return s
}

// DefaultMacroSplit applies MacroSplitPerKg with the default per-kg targets.
func DefaultMacroSplit(targetKcal, weightKg float64) Split {
	return MacroSplitPerKg(targetKcal, weightKg, DefaultProteinPerKg, DefaultFatPerKg)
}

// MacroSplitPercent derives target grams from a calorie target and a
// percentage split. Percentages should sum to 100; the result is never
// negative. Energy uses 4/4/9 kcal per gram for protein/carbs/fat.
func MacroSplitPercent(targetKcal, proteinPct, carbsPct, fatPct float64) domain.Macros {
	return domain.Macros{
		Kcal:    targetKcal,
		Protein: targetKcal * proteinPct / 100.0 / 4.0,
		Carbs:   targetKcal * carbsPct / 100.0 / 4.0,
		Fat:     targetKcal * fatPct / 100.0 / 9.0,
	}
}

// MacroPercentages returns the share of energy from each macro (protein, carbs,
// fat) for a given set of macros, using 4/4/9 factors against ComputedKcal.
// Returns zeros if there is no energy.
func MacroPercentages(m domain.Macros) (proteinPct, carbsPct, fatPct float64) {
	total := m.ComputedKcal()
	if total <= 0 {
		return 0, 0, 0
	}
	return 4 * m.Protein / total * 100,
		4 * m.Carbs / total * 100,
		9 * m.Fat / total * 100
}
