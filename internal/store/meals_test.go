package store

import (
	"testing"

	"caltui/internal/domain"
)

func TestSavedMeals(t *testing.T) {
	s := openTestStore(t)

	id, err := s.AddSavedMeal(domain.SavedMeal{
		Name: "Usual lunch", Kind: "meal",
		Items: []domain.SavedMealItem{
			{Name: "Chicken", PerUnit: domain.Macros{Kcal: 1.65, Protein: 0.31}, Quantity: 200, Unit: domain.UnitGram},
			{Name: "Rice", PerUnit: domain.Macros{Kcal: 1.30, Carbs: 0.28}, Quantity: 150, Unit: domain.UnitGram},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	meals, err := s.ListSavedMeals()
	if err != nil {
		t.Fatal(err)
	}
	if len(meals) != 1 || meals[0].Name != "Usual lunch" || len(meals[0].Items) != 2 {
		t.Fatalf("ListSavedMeals = %+v", meals)
	}
	// 200*1.65 + 150*1.30 = 330 + 195 = 525.
	if k := meals[0].Total().Kcal; k != 525 {
		t.Errorf("saved meal total = %g, want 525", k)
	}

	n, err := s.LogSavedMeal(id, "2026-06-06", domain.Dinner)
	if err != nil || n != 2 {
		t.Fatalf("LogSavedMeal = %d, %v; want 2", n, err)
	}
	entries, _ := s.EntriesForDate("2026-06-06")
	if len(entries) != 2 {
		t.Fatalf("expected 2 logged entries, got %d", len(entries))
	}
	if tot, _ := s.DayTotals("2026-06-06"); tot.Kcal != 525 {
		t.Errorf("day totals after logging meal = %g, want 525", tot.Kcal)
	}
	for _, e := range entries {
		if e.Meal != domain.Dinner {
			t.Errorf("logged entry meal = %q, want dinner", e.Meal)
		}
	}

	if err := s.DeleteSavedMeal(id); err != nil {
		t.Fatal(err)
	}
	if meals, _ := s.ListSavedMeals(); len(meals) != 0 {
		t.Errorf("expected no saved meals after delete, got %d", len(meals))
	}
	// Items cascade away.
	var itemCount int
	_ = s.DB().QueryRow(`SELECT COUNT(*) FROM saved_meal_items`).Scan(&itemCount)
	if itemCount != 0 {
		t.Errorf("saved_meal_items should cascade-delete, %d remain", itemCount)
	}
}
