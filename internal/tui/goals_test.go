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
	exp := nutrition.CalorieTarget(tdee, -0.5)

	target, _, ok := w.compute()
	if !ok {
		t.Fatal("compute should be valid")
	}
	if target.Kcal != exp.Kcal {
		t.Errorf("wizard target = %g, want %g", target.Kcal, exp.Kcal)
	}

	_, cmd := w.Update(press("enter"))
	save, ok := cmd().(saveSetupMsg)
	if !ok {
		t.Fatal("expected saveSetupMsg")
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
	// The entered weight is logged as a weigh-in during setup.
	if save.weight.Kg != 82 || save.weight.Unit != "kg" || save.weight.Date != "2026-06-06" {
		t.Errorf("setup should record the entered weight, got %+v", save.weight)
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
	// Step 1: a day load with no goal pops the TDEE wizard.
	m, _ = update(t, m, dayLoadedMsg{hasGoal: false})
	if _, ok := m.modal.(*wizardModal); !ok {
		t.Fatalf("expected wizard modal on first run, got %T", m.modal)
	}
	// Step 2: after the wizard is dismissed, onboarding offers the API-key modal
	// (not the wizard again).
	m, _ = update(t, m, closeModalMsg{})
	m, _ = update(t, m, dayLoadedMsg{hasGoal: false})
	if _, ok := m.modal.(*apiKeyModal); !ok {
		t.Fatalf("expected API-key modal as onboarding step 2, got %T", m.modal)
	}
	// Once both have been offered, nothing re-pops.
	m, _ = update(t, m, closeModalMsg{})
	m, _ = update(t, m, dayLoadedMsg{hasGoal: false})
	if m.modal != nil {
		t.Errorf("onboarding should be complete, got %T", m.modal)
	}
}

// TestDashboardReflectsGoalUpdate verifies the dashboard's "calories left"
// updates after the goal changes, via the real save -> mutationDone -> reload
// message chain.
func TestDashboardReflectsGoalUpdate(t *testing.T) {
	s := testStore(t)
	m := New(s, nil) // today = now
	m.width, m.height = 100, 30
	date := m.today

	// 1500 kcal logged today.
	if _, err := s.AddEntry(domain.LogEntry{
		Date: date, Meal: domain.Lunch, Name: "x",
		PerUnit: domain.Macros{Kcal: 1}, Quantity: 1500, Unit: domain.UnitGram,
	}); err != nil {
		t.Fatal(err)
	}

	// runChain applies a saveGoalMsg and drives the async commands it spawns
	// (saveGoalCmd -> mutationDoneMsg -> loadDay -> dayLoadedMsg).
	runChain := func(m Model, target float64) Model {
		m, cmd := update(t, m, saveGoalMsg{goal: domain.Goal{
			EffectiveDate: date, Target: domain.Macros{Kcal: target}, Manual: true,
		}})
		m, cmd = update(t, m, cmd()) // mutationDoneMsg
		m, _ = update(t, m, cmd())   // dayLoadedMsg
		return m
	}

	m = runChain(m, 2000)
	if !strings.Contains(m.render(), "500 kcal left") {
		t.Errorf("after goal 2000 (1500 logged), expected '500 kcal left'")
	}
	m = runChain(m, 1800)
	if !strings.Contains(m.render(), "300 kcal left") {
		t.Errorf("after goal update to 1800, expected '300 kcal left' (dashboard did not refresh)")
	}
}

func TestApplyMaintenance(t *testing.T) {
	s := testStore(t)
	m := New(s, nil)
	date := m.today
	now := time.Now()
	tt, _ := time.Parse("2006-01-02", date)

	m.goal = domain.Goal{
		EffectiveDate: date, Target: domain.Macros{Kcal: 2200},
		Sex: domain.Male, BirthDate: nutrition.BirthDateForAge(30, now),
		HeightCm: 180, WeightKg: 80, Activity: domain.ModeratelyActive, GoalRate: -0.5,
	}
	m.hasGoal = true
	m.intakeByDay = map[string]float64{}
	for i := 0; i < 21; i++ {
		m.intakeByDay[tt.AddDate(0, 0, -i).Format("2006-01-02")] = 2000
	}
	m.weights = []domain.Weight{
		{Date: tt.AddDate(0, 0, -14).Format("2006-01-02"), Kg: 82, Unit: "kg"},
		{Date: date, Kg: 81, Unit: "kg"},
	}

	_, cmd := m.applyMaintenance()
	if cmd == nil {
		t.Fatal("applyMaintenance produced no command")
	}
	if d, ok := cmd().(mutationDoneMsg); !ok || d.err != nil {
		t.Fatalf("apply failed: %+v", d)
	}

	// est maintenance = 2000 - (-1*7700/14) = 2550; target at -0.5 = 2000.
	want := nutrition.CalorieTarget(2550, -0.5).Kcal
	g, _, _ := s.CurrentGoal(date)
	if math.Abs(g.Target.Kcal-math.Round(want)) > 1 {
		t.Errorf("applied target = %g, want ~%g", g.Target.Kcal, math.Round(want))
	}

	// Goals view surfaces the estimate.
	if !strings.Contains(m.viewGoals(100), "Est. maintenance") {
		t.Error("goals view should show the maintenance estimate when data is sufficient")
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
