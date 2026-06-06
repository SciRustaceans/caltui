package domain

import "math"

// Macros is a bundle of energy + the three tracked macronutrients. It is the
// central value type: per-100g food data, per-unit log snapshots, daily totals,
// and goal targets are all Macros.
type Macros struct {
	Kcal    float64
	Protein float64 // grams
	Carbs   float64 // grams
	Fat     float64 // grams
}

// Add returns the element-wise sum.
func (m Macros) Add(o Macros) Macros {
	return Macros{m.Kcal + o.Kcal, m.Protein + o.Protein, m.Carbs + o.Carbs, m.Fat + o.Fat}
}

// Scale multiplies every field by f.
func (m Macros) Scale(f float64) Macros {
	return Macros{m.Kcal * f, m.Protein * f, m.Carbs * f, m.Fat * f}
}

// ComputedKcal is energy implied by the macros using the 4/4/9 Atwater factors.
// It can differ from Kcal (label energy) due to fiber, alcohol, or rounding.
func (m Macros) ComputedKcal() float64 {
	return 4*m.Protein + 4*m.Carbs + 9*m.Fat
}

// Round rounds energy to the nearest kcal and macros to one decimal gram, for
// display and stable storage.
func (m Macros) Round() Macros {
	r1 := func(x float64) float64 { return math.Round(x*10) / 10 }
	return Macros{math.Round(m.Kcal), r1(m.Protein), r1(m.Carbs), r1(m.Fat)}
}

// Food is a nutrition reference item. Macros are stored per 100 grams, the
// canonical basis. ServingSize/ServingUnit/Household describe a default serving
// for ergonomic logging; Density (g per ml) supports volume units.
type Food struct {
	ID          int64
	Source      FoodSource
	FDCID       *int64 // USDA FoodData Central id, when applicable
	Name        string
	Brand       string
	Per100g     Macros
	ServingSize float64 // amount of one serving, expressed in ServingUnit
	ServingUnit Unit
	Household   string  // e.g. "1 medium (118 g)"
	Density     float64 // grams per ml; 0 means unknown (callers assume 1.0)
}

// LogEntry is one logged food in the diary. It stores an immutable per-unit
// macro snapshot so editing or deleting the source Food never rewrites history.
// FoodID (nullable) survives for recent/frequent/copy features.
type LogEntry struct {
	ID       int64
	Date     string // YYYY-MM-DD, local
	Meal     Meal
	FoodID   *int64
	Name     string  // snapshot of the food name at log time
	PerUnit  Macros  // macros for one Unit
	Quantity float64 // number of Units
	Unit     Unit
}

// Total is the macro contribution of this entry (PerUnit * Quantity).
func (e LogEntry) Total() Macros { return e.PerUnit.Scale(e.Quantity) }

// Goal is a dated set of daily targets plus the inputs that produced them, so a
// suggestion can always be recomputed/explained. The latest goal whose
// EffectiveDate is on or before a given day is the one in force.
type Goal struct {
	ID            int64
	EffectiveDate string // YYYY-MM-DD
	Target        Macros
	// Inputs (zero-valued when the goal was set purely manually):
	Sex       Sex
	BirthDate string // YYYY-MM-DD
	HeightCm  float64
	WeightKg  float64
	Activity  ActivityLevel
	GoalRate  float64 // kg per week; negative = loss, positive = gain, 0 = maintain
	// Manual reports the user overrode the calculated targets by hand.
	Manual bool
}

// Weight is a single body-weight measurement. Internally weight is kilograms;
// Unit records how the user entered it for display round-tripping.
type Weight struct {
	ID   int64
	Date string // YYYY-MM-DD, unique per day
	Kg   float64
	Unit string // "kg" or "lb" (display preference)
}

// WeightGoal is the single-row target for body weight.
type WeightGoal struct {
	TargetKg    float64
	Unit        string  // display preference
	RatePerWeek float64 // kg per week toward target (signed)
	StartDate   string
	StartKg     float64
}

// SavedMeal is a reusable bundle of food items logged together with one action.
type SavedMeal struct {
	ID       int64
	Name     string
	Kind     string // "meal" or "recipe"
	Servings float64
	Items    []SavedMealItem
}

// Total sums the macros of every item in the saved meal.
func (s SavedMeal) Total() Macros {
	var t Macros
	for _, it := range s.Items {
		t = t.Add(it.Total())
	}
	return t
}

// SavedMealItem is one component of a SavedMeal, snapshotted like a LogEntry.
type SavedMealItem struct {
	ID       int64
	FoodID   *int64
	Name     string
	PerUnit  Macros
	Quantity float64
	Unit     Unit
}

// Total is the macro contribution of this saved item.
func (i SavedMealItem) Total() Macros { return i.PerUnit.Scale(i.Quantity) }
