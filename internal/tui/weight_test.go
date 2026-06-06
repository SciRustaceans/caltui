package tui

import (
	"math"
	"strings"
	"testing"
	"time"

	"caltui/internal/domain"
	"caltui/internal/nutrition"
)

func TestWeightEntryKg(t *testing.T) {
	wm := newWeightEntryModal("2026-06-06", "kg", 0)
	wm.focus()
	typeMM(wm, "82.1")
	_, cmd := wm.Update(press("enter"))
	save, ok := cmd().(saveWeightMsg)
	if !ok {
		t.Fatal("expected saveWeightMsg")
	}
	if math.Abs(save.weight.Kg-82.1) > 1e-6 || save.weight.Unit != "kg" {
		t.Errorf("weight = %+v", save.weight)
	}
}

func TestWeightEntryLbConverts(t *testing.T) {
	wm := newWeightEntryModal("2026-06-06", "lb", 0)
	wm.focus()
	typeMM(wm, "180")
	_, cmd := wm.Update(press("enter"))
	save := cmd().(saveWeightMsg)
	if math.Abs(save.weight.Kg-nutrition.LbToKg(180)) > 1e-6 {
		t.Errorf("lb not converted: %g", save.weight.Kg)
	}
	if save.weight.Unit != "lb" {
		t.Errorf("unit = %q, want lb", save.weight.Unit)
	}
}

func TestWeightGoalSaves(t *testing.T) {
	gm := newWeightGoalModal("2026-06-06", "kg", 82.5, domain.WeightGoal{}, false)
	gm.focus()
	typeMM(gm, "78")
	// default pace is maintain (idx 2); cycle to lose 0.5 (idx 0).
	gm.Update(press("tab")) // -> unit
	gm.Update(press("tab")) // -> pace
	gm.Update(press("left"))
	gm.Update(press("left"))
	_, cmd := gm.Update(press("enter"))
	save, ok := cmd().(saveWeightGoalMsg)
	if !ok {
		t.Fatal("expected saveWeightGoalMsg")
	}
	g := save.goal
	if g.TargetKg != 78 || g.RatePerWeek != -0.5 || g.StartKg != 82.5 {
		t.Errorf("weight goal = %+v", g)
	}
}

func TestRootWeightWiring(t *testing.T) {
	s := testStore(t)
	m := NewForDate(s, "2026-06-06")
	m.width, m.height = 100, 30

	_, cmd := update(t, m, saveWeightMsg{weight: domain.Weight{Date: "2026-06-06", Kg: 81.9, Unit: "kg"}})
	if d, ok := cmd().(mutationDoneMsg); !ok || d.err != nil {
		t.Fatalf("save weight failed: %+v", d)
	}
	if w, ok, _ := s.LatestWeight(); !ok || w.Kg != 81.9 {
		t.Errorf("weight not persisted: %+v ok=%v", w, ok)
	}

	_, cmd = update(t, m, saveWeightGoalMsg{goal: domain.WeightGoal{TargetKg: 78, RatePerWeek: -0.5, StartKg: 82}})
	if d, ok := cmd().(mutationDoneMsg); !ok || d.err != nil {
		t.Fatalf("save weight goal failed: %+v", d)
	}
	if g, ok, _ := s.GetWeightGoal(); !ok || g.TargetKg != 78 {
		t.Errorf("weight goal not persisted: %+v", g)
	}
}

// TestWeightPaceUpdatesCalories verifies that setting the weight-goal pace
// recomputes the daily calorie target from the goal's TDEE inputs.
func TestWeightPaceUpdatesCalories(t *testing.T) {
	s := testStore(t)
	m := New(s, nil)
	m.width, m.height = 100, 30
	date := m.today
	now := time.Now()

	// An existing wizard-style goal with full TDEE inputs (male, 90 kg, 180 cm, 30y).
	m.goal = domain.Goal{
		EffectiveDate: date, Target: domain.Macros{Kcal: 2700},
		Sex: domain.Male, BirthDate: nutrition.BirthDateForAge(30, now),
		HeightCm: 180, WeightKg: 90, Activity: domain.ModeratelyActive, GoalRate: -0.25,
	}
	m.hasGoal = true

	_, cmd := update(t, m, saveWeightGoalMsg{goal: domain.WeightGoal{TargetKg: 80, RatePerWeek: -1.0, StartKg: 90}})
	if d, ok := cmd().(mutationDoneMsg); !ok || d.err != nil {
		t.Fatalf("save weight goal failed: %+v", d)
	}

	g, ok, _ := s.CurrentGoal(date)
	if !ok {
		t.Fatal("no current goal")
	}
	bmr := nutrition.BMR(domain.Male, 90, 180, 30)
	want := nutrition.CalorieTarget(nutrition.TDEE(bmr, domain.ModeratelyActive), -1.0).Kcal
	if g.GoalRate != -1.0 {
		t.Errorf("goal rate = %g, want -1.0 (pace not applied to calories)", g.GoalRate)
	}
	if math.Abs(g.Target.Kcal-math.Round(want)) > 0.5 {
		t.Errorf("calorie target = %g, want ~%g after pace change", g.Target.Kcal, math.Round(want))
	}
}

func TestWeightView(t *testing.T) {
	m := sampleModel()
	m.active = tabWeight
	m.weights = []domain.Weight{
		{Date: "2026-06-04", Kg: 82.5, Unit: "kg"},
		{Date: "2026-06-05", Kg: 82.2, Unit: "kg"},
		{Date: "2026-06-06", Kg: 81.9, Unit: "kg"},
	}
	m.hasWeightGoal = true
	m.weightGoal = domain.WeightGoal{TargetKg: 78, Unit: "kg", RatePerWeek: -0.5, StartKg: 82.5}
	out := m.viewWeight(100)
	for _, want := range []string{"Weight", "Current", "81.9", "Goal", "78", "to go"} {
		if !strings.Contains(out, want) {
			t.Errorf("weight view missing %q", want)
		}
	}
}

func TestWeightTabKeysOpenModals(t *testing.T) {
	s := testStore(t)
	m := NewForDate(s, "2026-06-06")
	m.width, m.height = 100, 30
	m.active = tabWeight
	m, _ = update(t, m, press("a"))
	if _, ok := m.modal.(*weightEntryModal); !ok {
		t.Errorf("'a' on weight tab should open entry modal, got %T", m.modal)
	}
	m.modal = nil
	m, _ = update(t, m, press("e"))
	if _, ok := m.modal.(*weightGoalModal); !ok {
		t.Errorf("'e' on weight tab should open goal modal, got %T", m.modal)
	}
}
