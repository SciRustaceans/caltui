package tui

import (
	"math"
	"testing"

	"caltui/internal/domain"
)

func macrosKcal(mm *manualGoalModal) (p, c, f, total float64) {
	p = parseF(mm.form.Value(1))
	c = parseF(mm.form.Value(2))
	f = parseF(mm.form.Value(3))
	return p, c, f, 4*p + 4*c + 9*f
}

func TestManualGoalRebalanceMacro(t *testing.T) {
	// 4*100 + 4*300 + 9*44 = 1996 ≈ 2000.
	mm := newManualGoalModal("2026-06-06", domain.Goal{
		Target: domain.Macros{Kcal: 2000, Protein: 100, Carbs: 300, Fat: 44},
	})
	mm.form.fields[1].ti.SetValue("150") // bump protein
	mm.balance(1)

	p, c, f, total := macrosKcal(mm)
	if p != 150 {
		t.Errorf("edited protein should stay 150, got %g", p)
	}
	if c == 300 || f == 44 {
		t.Errorf("other macros should rebalance, got C=%g F=%g", c, f)
	}
	if math.Abs(total-2000) > 12 { // within integer-gram rounding
		t.Errorf("total kcal = %g, want ~2000", total)
	}
}

func TestManualGoalRescaleOnCalorieChange(t *testing.T) {
	mm := newManualGoalModal("2026-06-06", domain.Goal{
		Target: domain.Macros{Kcal: 2000, Protein: 150, Carbs: 200, Fat: 55.6},
	})
	mm.form.fields[0].ti.SetValue("2200") // raise calorie target
	mm.balance(0)

	_, _, _, total := macrosKcal(mm)
	if math.Abs(total-2200) > 12 {
		t.Errorf("rescaled total kcal = %g, want ~2200", total)
	}
}

func TestManualGoalDefaultSplitFromCalories(t *testing.T) {
	mm := newManualGoalModal("2026-06-06", domain.Goal{}) // no macros
	mm.form.fields[0].ti.SetValue("2000")
	mm.balance(0)
	p, c, f, total := macrosKcal(mm)
	if p == 0 || c == 0 || f == 0 {
		t.Errorf("default split should populate all macros, got P=%g C=%g F=%g", p, c, f)
	}
	if math.Abs(total-2000) > 12 {
		t.Errorf("default-split total = %g, want ~2000", total)
	}
}

func TestManualGoalRebalanceViaKeys(t *testing.T) {
	mm := newManualGoalModal("2026-06-06", domain.Goal{
		Target: domain.Macros{Kcal: 2000, Protein: 100, Carbs: 300, Fat: 44},
	})
	mm.focus()              // field 0 (calories)
	mm.Update(press("tab")) // -> protein (calories unchanged, no rebalance)
	mm.form.fields[1].ti.SetValue("150")
	mm.Update(press("tab")) // leaving protein (changed) -> rebalance carbs+fat

	_, c, f, total := macrosKcal(mm)
	if c == 300 || f == 44 {
		t.Errorf("tabbing off an edited macro should rebalance: C=%g F=%g", c, f)
	}
	if math.Abs(total-2000) > 12 {
		t.Errorf("total kcal after key-driven rebalance = %g, want ~2000", total)
	}
}
