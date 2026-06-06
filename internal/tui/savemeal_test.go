package tui

import (
	"testing"

	"caltui/internal/domain"
)

func TestSaveMealFromDiary(t *testing.T) {
	m := sampleModel() // entries: Oatmeal (breakfast), Chicken breast (lunch)
	m.active = tabDiary
	m.diaryCursor = 1 // chicken breast, lunch

	nm, _ := m.openSaveMeal()
	m = nm.(Model)
	mm, ok := m.modal.(*saveMealModal)
	if !ok {
		t.Fatalf("expected saveMealModal, got %T", m.modal)
	}
	if mm.meal != domain.Lunch || len(mm.items) != 1 || mm.items[0].Name != "Chicken breast" {
		t.Fatalf("save-meal built wrong items: meal=%q items=%+v", mm.meal, mm.items)
	}

	mm.focus()
	typeMM(mm, "My lunch")
	_, cmd := mm.Update(press("enter"))
	save, ok := cmd().(saveMealMsg)
	if !ok {
		t.Fatal("expected saveMealMsg")
	}
	if save.meal.Name != "My lunch" || len(save.meal.Items) != 1 {
		t.Errorf("saved meal = %+v", save.meal)
	}
}

func TestLogSavedMealFromSearch(t *testing.T) {
	s := testStore(t)
	sm := newSearchModal(s, "2026-06-06", domain.Dinner, nil)
	sm.savedMeals = []domain.SavedMeal{{
		ID: 7, Name: "Usual",
		Items: []domain.SavedMealItem{{Name: "x", PerUnit: domain.Macros{Kcal: 100}, Quantity: 1, Unit: domain.UnitServing}},
	}}
	sm.focus()
	// Blank query → saved meals listed first; cursor at 0.
	_, cmd := sm.Update(press("enter"))
	msg, ok := cmd().(logSavedMealMsg)
	if !ok {
		t.Fatalf("expected logSavedMealMsg, got %T", cmd())
	}
	if msg.id != 7 || msg.meal != domain.Dinner {
		t.Errorf("log saved meal msg = %+v", msg)
	}

	// ctrl+d on the saved meal emits a delete.
	_, cmd = sm.Update(press("ctrl+d"))
	if del, ok := cmd().(deleteSavedMealMsg); !ok || del.id != 7 {
		t.Errorf("ctrl+d should delete saved meal 7, got %v", cmd())
	}
}

func TestRootSavedMealWiring(t *testing.T) {
	s := testStore(t)
	m := New(s, nil)
	date := m.today

	meal := domain.SavedMeal{Name: "Recipe", Items: []domain.SavedMealItem{
		{Name: "a", PerUnit: domain.Macros{Kcal: 100}, Quantity: 2, Unit: domain.UnitServing},
		{Name: "b", PerUnit: domain.Macros{Kcal: 50}, Quantity: 1, Unit: domain.UnitServing},
	}}
	_, cmd := update(t, m, saveMealMsg{meal: meal})
	if d, ok := cmd().(mutationDoneMsg); !ok || d.err != nil {
		t.Fatalf("save meal failed: %+v", d)
	}
	meals, _ := s.ListSavedMeals()
	if len(meals) != 1 {
		t.Fatalf("expected 1 saved meal, got %d", len(meals))
	}

	_, cmd = update(t, m, logSavedMealMsg{id: meals[0].ID, meal: domain.Lunch})
	if d, ok := cmd().(mutationDoneMsg); !ok || d.err != nil {
		t.Fatalf("log saved meal failed: %+v", d)
	}
	if entries, _ := s.EntriesForDate(date); len(entries) != 2 {
		t.Errorf("expected 2 logged entries from recipe, got %d", len(entries))
	}

	_, cmd = update(t, m, deleteSavedMealMsg{id: meals[0].ID})
	cmd()
	if meals, _ := s.ListSavedMeals(); len(meals) != 0 {
		t.Errorf("saved meal should be deleted, %d remain", len(meals))
	}
}
