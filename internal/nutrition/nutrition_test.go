package nutrition

import (
	"math"
	"testing"
	"time"

	"caltui/internal/domain"
)

func approx(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-4 {
		t.Errorf("%s = %g, want %g", name, got, want)
	}
}

func TestBMR(t *testing.T) {
	cases := []struct {
		name string
		sex  domain.Sex
		w, h float64
		age  int
		want float64
	}{
		// 10*68 + 6.25*165 - 5*35 - 161 = 1375.25
		{"female worked example", domain.Female, 68, 165, 35, 1375.25},
		// 10*82 + 6.25*180 - 5*30 + 5 = 1800
		{"male worked example", domain.Male, 82, 180, 30, 1800},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			approx(t, "BMR", BMR(c.sex, c.w, c.h, c.age), c.want)
		})
	}
}

func TestTDEE(t *testing.T) {
	cases := []struct {
		a    domain.ActivityLevel
		mult float64
	}{
		{domain.Sedentary, 1.2},
		{domain.LightlyActive, 1.375},
		{domain.ModeratelyActive, 1.55},
		{domain.VeryActive, 1.725},
		{domain.ExtraActive, 1.9},
	}
	for _, c := range cases {
		t.Run(string(c.a), func(t *testing.T) {
			approx(t, "TDEE", TDEE(1800, c.a), 1800*c.mult)
			approx(t, "Multiplier", c.a.Multiplier(), c.mult)
		})
	}
}

func TestCalorieTarget(t *testing.T) {
	cases := []struct {
		name        string
		tdee, rate  float64
		sex         domain.Sex
		wantKcal    float64
		wantClamped bool
	}{
		// 0.5 kg/wk loss => 550 deficit, comfortably above floors.
		{"moderate deficit, no clamp", 2790, -0.5, domain.Male, 2240, false},
		// Maintenance.
		{"maintain", 2200, 0, domain.Female, 2200, false},
		// Small surplus, under the cap.
		{"small surplus, no clamp", 2000, 0.2, domain.Male, 2220, false},
		// Sex floor binds: raw = 1800 - 1100 = 700 -> floor 1500.
		{"floor binds (male)", 1800, -1.0, domain.Male, 1500, true},
		// 25% rule binds: raw = 2500 - 1100 = 1400; max(1500, 1875) = 1875.
		{"deficit fraction binds", 2500, -1.0, domain.Male, 1875, true},
		// Surplus cap binds: raw = 2000 + 1100 = 3100; cap 1.2*2000 = 2400.
		{"surplus cap binds", 2000, 1.0, domain.Male, 2400, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := CalorieTarget(c.tdee, c.rate, c.sex)
			approx(t, "Kcal", got.Kcal, c.wantKcal)
			if got.Clamped != c.wantClamped {
				t.Errorf("Clamped = %v, want %v (reason %q)", got.Clamped, c.wantClamped, got.Reason)
			}
			if got.Clamped && got.Reason == "" {
				t.Errorf("clamped result must have a reason")
			}
		})
	}
}

func TestMacroSplitPerKg(t *testing.T) {
	// Normal case: plenty of room for carbs.
	s := MacroSplitPerKg(2240, 82, DefaultProteinPerKg, DefaultFatPerKg)
	if s.Adjusted {
		t.Errorf("did not expect adjustment, note=%q", s.Note)
	}
	approx(t, "protein", s.Macros.Protein, 1.8*82)
	approx(t, "fat", s.Macros.Fat, 0.9*82)
	approx(t, "carbs", s.Macros.Carbs, (2240-1.8*82*4-0.9*82*9)/4)

	// Negative-carb fallback: fat reduced to essential, carbs become positive.
	s2 := MacroSplitPerKg(1500, 90, 2.0, 1.0)
	if !s2.Adjusted {
		t.Fatalf("expected adjustment for low target / high protein")
	}
	approx(t, "fat reduced to essential", s2.Macros.Fat, essentialFatPerKg*90)
	if s2.Macros.Carbs < 0 {
		t.Errorf("carbs must not be negative after fallback, got %g", s2.Macros.Carbs)
	}
	approx(t, "carbs after fallback", s2.Macros.Carbs, (1500-2.0*90*4-essentialFatPerKg*90*9)/4)

	// Extreme: protein alone exceeds target -> carbs pinned to 0.
	s3 := MacroSplitPerKg(700, 90, 2.0, 1.0)
	if !s3.Adjusted || s3.Macros.Carbs != 0 {
		t.Errorf("expected carbs pinned to 0 for impossible target, got carbs=%g adjusted=%v", s3.Macros.Carbs, s3.Adjusted)
	}
}

func TestMacroSplitPercent(t *testing.T) {
	m := MacroSplitPercent(2000, 30, 40, 30)
	approx(t, "protein", m.Protein, 2000*0.30/4)
	approx(t, "carbs", m.Carbs, 2000*0.40/4)
	approx(t, "fat", m.Fat, 2000*0.30/9)

	p, c, f := MacroPercentages(domain.Macros{Protein: 150, Carbs: 200, Fat: 66.6667})
	approx(t, "protein pct", p, 30)
	approx(t, "carbs pct", c, 40)
	approx(t, "fat pct", f, 30)
}

func chicken() domain.Food {
	return domain.Food{
		Name:    "Chicken breast, cooked",
		Per100g: domain.Macros{Kcal: 165, Protein: 31, Carbs: 0, Fat: 3.6},
	}
}

func TestMacrosFor(t *testing.T) {
	f := chicken()

	m, err := MacrosFor(f, 150, domain.UnitGram)
	if err != nil {
		t.Fatal(err)
	}
	approx(t, "150g kcal", m.Kcal, 165*1.5)
	approx(t, "150g protein", m.Protein, 31*1.5)

	// 4 oz = 113.398 g.
	m, err = MacrosFor(f, 4, domain.UnitOunce)
	if err != nil {
		t.Fatal(err)
	}
	approx(t, "4oz grams->kcal", m.Kcal, 165*(4*GramsPerOunce)/100)

	// Serving expressed in grams.
	banana := domain.Food{
		Name:        "Banana",
		Per100g:     domain.Macros{Kcal: 89, Protein: 1.1, Carbs: 22.8, Fat: 0.3},
		ServingSize: 118, ServingUnit: domain.UnitGram, Household: "1 medium (118 g)",
	}
	m, err = MacrosFor(banana, 1, domain.UnitServing)
	if err != nil {
		t.Fatal(err)
	}
	approx(t, "1 serving (118g) kcal", m.Kcal, 89*1.18)

	// ml uses density (default 1.0).
	milk := domain.Food{Name: "Milk", Per100g: domain.Macros{Kcal: 61}, Density: 1.03}
	m, err = MacrosFor(milk, 250, domain.UnitMilliliter)
	if err != nil {
		t.Fatal(err)
	}
	approx(t, "250ml milk kcal", m.Kcal, 61*(250*1.03)/100)
}

func TestMacrosForErrors(t *testing.T) {
	f := chicken()
	if _, err := MacrosFor(f, 0, domain.UnitGram); err == nil {
		t.Error("expected error for zero quantity")
	}
	if _, err := MacrosFor(f, -5, domain.UnitGram); err == nil {
		t.Error("expected error for negative quantity")
	}
	// A serving/piece with no gram weight cannot be converted.
	piece := domain.Food{Name: "Mystery", ServingUnit: domain.UnitPiece, ServingSize: 1}
	if _, err := MacrosFor(piece, 1, domain.UnitServing); err == nil {
		t.Error("expected error converting serving with no gram weight")
	}
}

func TestPer100gFromServing(t *testing.T) {
	// 30 g serving with 120 kcal -> 400 kcal/100g.
	per100, err := Per100gFromServing(domain.Macros{Kcal: 120, Protein: 3, Carbs: 24, Fat: 1.5}, 30)
	if err != nil {
		t.Fatal(err)
	}
	approx(t, "per100 kcal", per100.Kcal, 400)
	approx(t, "per100 protein", per100.Protein, 10)
	if _, err := Per100gFromServing(domain.Macros{Kcal: 100}, 0); err == nil {
		t.Error("expected error for zero serving size")
	}
}

func TestKcalMismatch(t *testing.T) {
	// computed = 40+80+45 = 165; |200-165|/200 = 0.175 > 0.15 -> mismatch.
	mismatch, computed := KcalMismatch(domain.Macros{Kcal: 200, Protein: 10, Carbs: 20, Fat: 5})
	approx(t, "computed", computed, 165)
	if !mismatch {
		t.Error("expected mismatch")
	}
	// Within tolerance.
	mismatch, _ = KcalMismatch(domain.Macros{Kcal: 170, Protein: 10, Carbs: 20, Fat: 5})
	if mismatch {
		t.Error("did not expect mismatch within tolerance")
	}
}

func TestWeightConversions(t *testing.T) {
	approx(t, "154lb->kg", LbToKg(154), 154*0.45359237)
	approx(t, "roundtrip", KgToLb(LbToKg(80)), 80)
	approx(t, "kJ->kcal", KJToKcal(418.4), 100)
}

func TestAgeFromDate(t *testing.T) {
	ref := time.Date(2026, time.June, 6, 0, 0, 0, 0, time.UTC)
	// Birthday June 10 hasn't happened yet on June 6.
	age, err := AgeFromDate("1990-06-10", ref)
	if err != nil {
		t.Fatal(err)
	}
	if age != 35 {
		t.Errorf("age = %d, want 35", age)
	}
	// On/after the birthday.
	age, _ = AgeFromDate("1990-06-06", ref)
	if age != 36 {
		t.Errorf("age = %d, want 36", age)
	}
	if _, err := AgeFromDate("not-a-date", ref); err == nil {
		t.Error("expected parse error")
	}
}
