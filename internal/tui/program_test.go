package tui

import (
	"bytes"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/teatest/v2"

	"caltui/internal/domain"
	"caltui/internal/nutrition"
)

// TestProgramSmoke runs the real Bubble Tea program through teatest: it
// exercises the full lifecycle (Init -> WindowSize -> render) and the actual
// input decoder (typed bytes -> KeyPressMsg -> handlers), which the direct
// Update tests bypass.
func TestProgramSmoke(t *testing.T) {
	s := testStore(t)
	today := time.Now().Format("2006-01-02")
	// Seed a goal so the first-run wizard does not auto-open.
	if _, err := s.AddGoal(domain.Goal{
		EffectiveDate: today,
		Target:        domain.Macros{Kcal: 2100, Protein: 140, Carbs: 210, Fat: 70},
		Manual:        true,
	}); err != nil {
		t.Fatal(err)
	}

	tm := teatest.NewTestModel(t, New(s, nil), teatest.WithInitialTermSize(100, 30))
	out := tm.Output()

	// Initial dashboard render.
	teatest.WaitFor(t, out, func(b []byte) bool {
		return bytes.Contains(b, []byte("Dashboard")) && bytes.Contains(b, []byte("Calories"))
	}, teatest.WithDuration(5*time.Second))

	// A real "2" keystroke must decode and switch to the Diary tab.
	tm.Type("2")
	teatest.WaitFor(t, out, func(b []byte) bool {
		return bytes.Contains(b, []byte("BREAKFAST"))
	}, teatest.WithDuration(5*time.Second))

	// "q" quits.
	tm.Type("q")
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

// TestWeightPaceUpdatesDashboardE2E drives the exact reported flow: open the
// weight-goal modal, set a "Lose 1 kg/week" pace, save, and confirm the
// dashboard's calorie target actually changes.
func TestWeightPaceUpdatesDashboardE2E(t *testing.T) {
	s := testStore(t)
	now := time.Now()
	today := now.Format("2006-01-02")
	if _, err := s.AddGoal(domain.Goal{
		EffectiveDate: today, Target: domain.Macros{Kcal: 2209, Protein: 144, Carbs: 230, Fat: 72},
		Sex: domain.Male, BirthDate: nutrition.BirthDateForAge(30, now),
		HeightCm: 180, WeightKg: 80, Activity: domain.ModeratelyActive, GoalRate: -0.5,
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertWeight(domain.Weight{Date: today, Kg: 80, Unit: "kg"}); err != nil {
		t.Fatal(err)
	}

	// After "Lose 1 kg/week": target = TDEE(BMR(male,80,180,30), moderate) - 1100.
	want := nutrition.CalorieTarget(nutrition.TDEE(nutrition.BMR(domain.Male, 80, 180, 30), domain.ModeratelyActive), -1.0).Kcal
	wantStr := fmt.Sprintf("%d", int(math.Round(want))) // 1659

	tm := teatest.NewTestModel(t, New(s, nil), teatest.WithInitialTermSize(120, 32))
	out := tm.Output()
	teatest.WaitFor(t, out, contains("2209"), teatest.WithDuration(5*time.Second)) // initial target

	tm.Send(press("4")) // Weight tab
	teatest.WaitFor(t, out, contains("Current:"), teatest.WithDuration(5*time.Second))
	tm.Send(press("e")) // open weight-goal modal
	teatest.WaitFor(t, out, contains("Weight goal"), teatest.WithDuration(5*time.Second))

	tm.Type("75")          // target weight
	tm.Send(press("tab"))  // -> unit
	tm.Send(press("tab"))  // -> pace
	tm.Send(press("left")) // maintain -> lose 0.25
	tm.Send(press("left")) // -> lose 0.5
	tm.Send(press("left")) // -> lose 0.75
	tm.Send(press("left")) // -> lose 1
	tm.Send(press("enter"))

	// Wait until the modal closes and the new weight goal is shown.
	teatest.WaitFor(t, out, contains("Goal: 75"), teatest.WithDuration(5*time.Second))

	tm.Send(press("1")) // Dashboard
	teatest.WaitFor(t, out, contains(wantStr), teatest.WithDuration(5*time.Second))

	tm.Send(press("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))

	fm := tm.FinalModel(t).(Model)
	if fm.goal.GoalRate != -1.0 {
		t.Errorf("final goal rate = %g, want -1.0", fm.goal.GoalRate)
	}
}

func contains(s string) func([]byte) bool {
	return func(b []byte) bool { return bytes.Contains(b, []byte(s)) }
}

// TestLogSavedMealE2E drives the real flow: open the food search, pick a saved
// meal, and confirm its calories land on the dashboard.
func TestLogSavedMealE2E(t *testing.T) {
	s := testStore(t)
	today := time.Now().Format("2006-01-02")
	if _, err := s.AddGoal(domain.Goal{EffectiveDate: today, Target: domain.Macros{Kcal: 2000}, Manual: true}); err != nil {
		t.Fatal(err)
	}
	// Saved meal totalling 525 kcal (200*1.65 + 150*1.30).
	if _, err := s.AddSavedMeal(domain.SavedMeal{Name: "Usual lunch", Items: []domain.SavedMealItem{
		{Name: "Chicken", PerUnit: domain.Macros{Kcal: 1.65}, Quantity: 200, Unit: domain.UnitGram},
		{Name: "Rice", PerUnit: domain.Macros{Kcal: 1.30}, Quantity: 150, Unit: domain.UnitGram},
	}}); err != nil {
		t.Fatal(err)
	}

	tm := teatest.NewTestModel(t, New(s, nil), teatest.WithInitialTermSize(120, 32))
	out := tm.Output()
	teatest.WaitFor(t, out, contains("Dashboard"), teatest.WithDuration(5*time.Second))

	tm.Send(press("a")) // open food search (saved meals listed first on blank query)
	teatest.WaitFor(t, out, contains("Usual lunch"), teatest.WithDuration(5*time.Second))
	tm.Send(press("enter")) // log the saved meal

	// Dashboard calorie total reflects the logged 525 kcal.
	teatest.WaitFor(t, out, contains("525"), teatest.WithDuration(5*time.Second))

	tm.Send(press("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
