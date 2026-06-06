package tui

import (
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"caltui/internal/domain"
	"caltui/internal/store"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func typeStr(sm *searchModal, s string) {
	for _, r := range s {
		sm.Update(press(string(r)))
	}
}

func TestSearchModalAddFlow(t *testing.T) {
	s := testStore(t)
	if _, err := s.InsertFood(domain.Food{
		Source: domain.SourceCustom, Name: "Banana",
		Per100g:     domain.Macros{Kcal: 89, Protein: 1.1, Carbs: 22.8, Fat: 0.3},
		ServingSize: 118, ServingUnit: domain.UnitGram,
	}); err != nil {
		t.Fatal(err)
	}

	sm := newSearchModal(s, "2026-06-06", domain.Breakfast, nil)
	sm.focus()
	typeStr(sm, "banana")
	// Run the search command and feed its results back.
	sm.Update(sm.searchCmd()())
	if len(sm.results) == 0 {
		t.Fatalf("expected search results for 'banana'")
	}

	sm.Update(press("enter")) // pick first result -> detail step
	if sm.step != stepDetail {
		t.Fatalf("expected detail step, got %d", sm.step)
	}
	// Banana has a serving size, so the default unit is serving, qty 1.
	if sm.currentUnit() != domain.UnitServing {
		t.Errorf("default unit = %q, want serving", sm.currentUnit())
	}

	_, cmd := sm.Update(press("enter")) // submit
	if cmd == nil {
		t.Fatal("submit produced no command")
	}
	save, ok := cmd().(saveEntryMsg)
	if !ok {
		t.Fatalf("expected saveEntryMsg")
	}
	e := save.entry
	if e.Name != "Banana" || e.Meal != domain.Breakfast || e.ID != 0 {
		t.Errorf("entry = %+v", e)
	}
	// 1 serving = 118 g -> 89 * 1.18 ≈ 105 kcal.
	if k := e.Total().Kcal; k < 103 || k > 107 {
		t.Errorf("total kcal = %g, want ~105", k)
	}
}

func TestSearchModalMealCycle(t *testing.T) {
	s := testStore(t)
	_, _ = s.InsertFood(domain.Food{Source: domain.SourceCustom, Name: "Rice", Per100g: domain.Macros{Kcal: 130}})
	sm := newSearchModal(s, "2026-06-06", domain.Breakfast, nil)
	sm.focus()
	typeStr(sm, "rice")
	sm.Update(sm.searchCmd()())
	sm.Update(press("enter")) // detail
	// Move focus to meal (qty -> unit -> meal) and cycle it.
	sm.Update(press("tab"))
	sm.Update(press("tab"))
	if sm.detailFocus != 2 {
		t.Fatalf("focus = %d, want meal", sm.detailFocus)
	}
	sm.Update(press("right"))
	if sm.meal != domain.Lunch {
		t.Errorf("meal = %q, want lunch after cycle", sm.meal)
	}
}

func TestQuickAddFlow(t *testing.T) {
	s := testStore(t)
	sm := newSearchModal(s, "2026-06-06", domain.Snacks, nil)
	sm.Update(press("ctrl+a"))
	if sm.step != stepQuick {
		t.Fatalf("expected quick step")
	}
	typeStr(sm, "Shake")    // name (field 0, focused)
	sm.Update(press("tab")) // -> calories
	typeStr(sm, "200")      // calories
	var cmd tea.Cmd
	for i := 0; i < 4; i++ { // advance through carbs/fat, then submit
		_, cmd = sm.Update(press("enter"))
	}
	save, ok := cmd().(saveEntryMsg)
	if !ok {
		t.Fatalf("expected saveEntryMsg from quick add")
	}
	e := save.entry
	if e.Name != "Shake" || e.Meal != domain.Snacks || e.PerUnit.Kcal != 200 || e.Unit != domain.UnitServing {
		t.Errorf("quick entry = %+v", e)
	}
}

func TestRootSaveAndDeleteWiring(t *testing.T) {
	s := testStore(t)
	m := NewForDate(s, "2026-06-06")
	m.width, m.height = 100, 30

	entry := domain.LogEntry{Date: "2026-06-06", Meal: domain.Lunch, Name: "Test",
		PerUnit: domain.Macros{Kcal: 100}, Quantity: 2, Unit: domain.UnitServing}
	m, cmd := update(t, m, saveEntryMsg{entry: entry})
	if cmd == nil {
		t.Fatal("save produced no command")
	}
	done, ok := cmd().(mutationDoneMsg)
	if !ok || done.err != nil {
		t.Fatalf("save did not complete: %+v", done)
	}
	entries, _ := s.EntriesForDate("2026-06-06")
	if len(entries) != 1 {
		t.Fatalf("expected 1 stored entry, got %d", len(entries))
	}

	// mutationDoneMsg closes any open modal and reloads.
	m.modal = newSearchModal(s, "2026-06-06", domain.Lunch, nil)
	m, _ = update(t, m, mutationDoneMsg{})
	if m.modal != nil {
		t.Error("mutationDoneMsg should close the modal")
	}

	// Delete via the diary 'd' key.
	m.active = tabDiary
	m.entries = entries
	m.diaryCursor = 0
	m, cmd = update(t, m, press("d"))
	if cmd == nil {
		t.Fatal("delete produced no command")
	}
	if d, ok := cmd().(mutationDoneMsg); !ok || d.err != nil {
		t.Fatalf("delete failed: %+v", d)
	}
	if entries, _ := s.EntriesForDate("2026-06-06"); len(entries) != 0 {
		t.Errorf("expected entry deleted, %d remain", len(entries))
	}
}

func TestOpenSearchModalKey(t *testing.T) {
	s := testStore(t)
	m := NewForDate(s, "2026-06-06")
	m.width, m.height = 100, 30
	m, _ = update(t, m, press("a"))
	if m.modal == nil {
		t.Fatal("'a' should open the search modal")
	}
	// esc closes it.
	_, cmd := m.modal.Update(press("esc"))
	if cmd == nil {
		t.Fatal("esc should emit a close command")
	}
	if _, ok := cmd().(closeModalMsg); !ok {
		t.Errorf("esc did not produce closeModalMsg")
	}
}
