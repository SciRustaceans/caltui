package tui

import (
	"math"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"caltui/internal/domain"
	"caltui/internal/nutrition"
)

func typeMM(u modalModel, s string) {
	for _, r := range s {
		u.Update(press(string(r)))
	}
}

func TestWizardComputeAndSave(t *testing.T) {
	ref := time.Date(2026, time.June, 6, 0, 0, 0, 0, time.UTC)
	w := newWizardModal("2026-06-06", ref, nil)
	w.focusActive()

	w.Update(press("right")) // sex: male -> female
	w.Update(press("tab"))   // -> age
	typeMM(w, "30")
	w.Update(press("tab")) // -> height
	typeMM(w, "180")
	w.Update(press("tab")) // -> weight
	typeMM(w, "82")
	w.Update(press("tab")) // -> activity (default moderate)
	w.Update(press("tab")) // -> goal (default maintain)
	w.Update(press("left"))
	w.Update(press("left")) // maintain -> lose 0.25 -> lose 0.5

	// Expected via the nutrition package directly.
	bmr := nutrition.BMR(domain.Female, 82, 180, 30)
	tdee := nutrition.TDEE(bmr, domain.ModeratelyActive)
	exp := nutrition.CalorieTarget(tdee, -0.5, domain.Female)

	target, _, ok := w.compute()
	if !ok {
		t.Fatal("compute should be valid")
	}
	if target.Kcal != exp.Kcal {
		t.Errorf("wizard target = %g, want %g", target.Kcal, exp.Kcal)
	}

	_, cmd := w.Update(press("enter"))
	save, ok := cmd().(saveGoalMsg)
	if !ok {
		t.Fatal("expected saveGoalMsg")
	}
	g := save.goal
	if g.Sex != domain.Female || g.WeightKg != 82 || g.HeightCm != 180 || g.GoalRate != -0.5 {
		t.Errorf("goal inputs wrong: %+v", g)
	}
	if g.Activity != domain.ModeratelyActive || g.Manual {
		t.Errorf("goal activity/manual wrong: %+v", g)
	}
	if g.Target.Kcal != math.Round(exp.Kcal) {
		t.Errorf("goal target = %g, want %g (rounded)", g.Target.Kcal, math.Round(exp.Kcal))
	}
}

func TestManualGoalSave(t *testing.T) {
	mm := newManualGoalModal("2026-06-06", domain.Goal{})
	mm.focus()
	typeMM(mm, "2000") // calories (field 0 focused)
	var cmd tea.Cmd
	for i := 0; i < 4; i++ {
		_, cmd = mm.Update(press("enter"))
	}
	save, ok := cmd().(saveGoalMsg)
	if !ok {
		t.Fatal("expected saveGoalMsg")
	}
	if save.goal.Target.Kcal != 2000 || !save.goal.Manual {
		t.Errorf("manual goal = %+v", save.goal)
	}
}

func TestRootSaveGoalWiring(t *testing.T) {
	s := testStore(t)
	m := NewForDate(s, "2026-06-06")
	m.width, m.height = 100, 30
	goal := domain.Goal{EffectiveDate: "2026-06-06",
		Target: domain.Macros{Kcal: 2000, Protein: 150, Carbs: 200, Fat: 60}, Manual: true}
	_, cmd := update(t, m, saveGoalMsg{goal: goal})
	if cmd == nil {
		t.Fatal("no save command")
	}
	if d, ok := cmd().(mutationDoneMsg); !ok || d.err != nil {
		t.Fatalf("save goal failed: %+v", d)
	}
	g, ok, _ := s.CurrentGoal("2026-06-06")
	if !ok || g.Target.Kcal != 2000 {
		t.Errorf("goal not persisted: %+v ok=%v", g, ok)
	}
}

func TestFirstRunOpensWizard(t *testing.T) {
	s := testStore(t)
	m := NewForDate(s, "2026-06-06")
	m.width, m.height = 100, 30
	// A day load with no goal should pop the wizard.
	m, _ = update(t, m, dayLoadedMsg{hasGoal: false})
	if _, ok := m.modal.(*wizardModal); !ok {
		t.Errorf("expected wizard modal on first run, got %T", m.modal)
	}
	// Dismiss it; a later reload with still-no-goal must NOT re-pop the wizard.
	m, _ = update(t, m, closeModalMsg{})
	m, _ = update(t, m, dayLoadedMsg{hasGoal: false})
	if m.modal != nil {
		t.Errorf("wizard should not re-open after being dismissed once, got %T", m.modal)
	}
}

func TestGoalsView(t *testing.T) {
	m := sampleModel()
	m.active = tabGoals
	out := m.viewGoals(100)
	for _, want := range []string{"Goals", "Daily target", "2100", "Protein", "Carbs", "Fat"} {
		if !strings.Contains(out, want) {
			t.Errorf("goals view missing %q", want)
		}
	}
}
