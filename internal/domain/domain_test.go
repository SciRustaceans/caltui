package domain

import "testing"

func TestMacrosArithmetic(t *testing.T) {
	a := Macros{Kcal: 100, Protein: 10, Carbs: 20, Fat: 5}
	b := Macros{Kcal: 50, Protein: 5, Carbs: 0, Fat: 1}
	sum := a.Add(b)
	if sum != (Macros{150, 15, 20, 6}) {
		t.Errorf("Add = %+v", sum)
	}
	scaled := a.Scale(2)
	if scaled != (Macros{200, 20, 40, 10}) {
		t.Errorf("Scale = %+v", scaled)
	}
	if got := (Macros{Protein: 10, Carbs: 20, Fat: 5}).ComputedKcal(); got != 165 {
		t.Errorf("ComputedKcal = %g, want 165", got)
	}
	r := Macros{Kcal: 165.4, Protein: 10.04, Carbs: 19.96, Fat: 5.55}.Round()
	if r != (Macros{165, 10, 20, 5.6}) {
		t.Errorf("Round = %+v", r)
	}
}

func TestMeal(t *testing.T) {
	if Breakfast.Title() != "Breakfast" {
		t.Errorf("Title = %q", Breakfast.Title())
	}
	m, err := ParseMeal("  LUNCH ")
	if err != nil || m != Lunch {
		t.Errorf("ParseMeal = %q, %v", m, err)
	}
	if _, err := ParseMeal("brunch"); err == nil {
		t.Error("expected error for unknown meal")
	}
	cases := map[int]Meal{6: Breakfast, 12: Lunch, 18: Dinner, 22: Snacks, 0: Breakfast}
	for hour, want := range cases {
		if got := MealForHour(hour); got != want {
			t.Errorf("MealForHour(%d) = %q, want %q", hour, got, want)
		}
	}
	if len(MealsInOrder) != 4 {
		t.Errorf("MealsInOrder len = %d", len(MealsInOrder))
	}
}

func TestActivityLevel(t *testing.T) {
	a, ok := ActivityLevelForMultiplier(1.55)
	if !ok || a != ModeratelyActive {
		t.Errorf("ActivityLevelForMultiplier(1.55) = %q, %v", a, ok)
	}
	if _, ok := ActivityLevelForMultiplier(1.4); ok {
		t.Error("expected no match for 1.4")
	}
	if !ModeratelyActive.Valid() || ActivityLevel("nope").Valid() {
		t.Error("Valid() wrong")
	}
}

func TestEntryTotals(t *testing.T) {
	e := LogEntry{PerUnit: Macros{Kcal: 50, Protein: 2}, Quantity: 3}
	if got := e.Total(); got != (Macros{Kcal: 150, Protein: 6}) {
		t.Errorf("Total = %+v", got)
	}
	sm := SavedMeal{Items: []SavedMealItem{
		{PerUnit: Macros{Kcal: 100}, Quantity: 1},
		{PerUnit: Macros{Kcal: 50}, Quantity: 2},
	}}
	if got := sm.Total(); got.Kcal != 200 {
		t.Errorf("SavedMeal.Total kcal = %g, want 200", got.Kcal)
	}
}
