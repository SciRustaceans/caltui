package tui

import (
	"context"
	"strings"
	"testing"

	"caltui/internal/domain"
)

type fakeProvider struct{ results []domain.Food }

func (f fakeProvider) Name() string { return "fake" }
func (f fakeProvider) Search(_ context.Context, _ string, _ int) ([]domain.Food, error) {
	return f.results, nil
}

func i64(v int64) *int64 { return &v }

func TestOnlineResultsMergeAndCache(t *testing.T) {
	s := testStore(t)
	if _, err := s.InsertFood(domain.Food{Source: domain.SourceCustom, Name: "Cheese, cheddar", Per100g: domain.Macros{Kcal: 400}}); err != nil {
		t.Fatal(err)
	}

	sm := newSearchModal(s, "2026-06-06", domain.Lunch, nil)
	sm.online = fakeProvider{results: []domain.Food{
		{Source: domain.SourceOnlineUSDA, Name: "Cheez-It", Per100g: domain.Macros{Kcal: 500}, FDCID: i64(999)},
	}}
	sm.focus()
	typeStr(sm, "chee")
	sm.Update(sm.searchCmd()()) // offline results
	if len(sm.results) == 0 {
		t.Fatal("expected offline results for 'chee'")
	}

	// Debounce tick triggers the online fetch; run it and feed the results back.
	_, cmd := sm.Update(onlineTickMsg{gen: sm.gen})
	if cmd == nil {
		t.Fatal("tick should trigger an online search command")
	}
	sm.Update(cmd())
	if sm.searching {
		t.Error("searching flag should clear after results arrive")
	}

	rows := sm.rows()
	var haveOffline, haveOnline bool
	for _, r := range rows {
		if r.food == nil {
			continue
		}
		if r.food.Name == "Cheese, cheddar" {
			haveOffline = true
		}
		if r.food.Name == "Cheez-It" {
			haveOnline = true
		}
	}
	if !haveOffline || !haveOnline {
		t.Errorf("merged list missing entries (offline=%v online=%v): %d rows", haveOffline, haveOnline, len(rows))
	}

	// Picking the online result caches it locally (gets a real id).
	online := sm.onlineResults[0]
	sm.pickFood(online)
	if sm.food == nil || sm.food.ID == 0 {
		t.Errorf("online food should be cached with an id, got %+v", sm.food)
	}
	if got, _ := s.SearchFoods("cheez", 5); len(got) == 0 {
		t.Error("cached online food should now be offline-searchable")
	}
}

func TestStaleOnlineResultsIgnored(t *testing.T) {
	sm := newSearchModal(testStore(t), "2026-06-06", domain.Lunch, nil)
	sm.gen = 5
	// A result tagged with an older generation must be ignored.
	sm.Update(onlineResultsMsg{gen: 3, results: []domain.Food{{Name: "stale"}}})
	if len(sm.onlineResults) != 0 {
		t.Errorf("stale online results should be ignored, got %d", len(sm.onlineResults))
	}
	sm.Update(onlineResultsMsg{gen: 5, results: []domain.Food{{Name: "fresh"}}})
	if len(sm.onlineResults) != 1 || !strings.Contains(sm.onlineResults[0].Name, "fresh") {
		t.Errorf("current-gen results should apply, got %+v", sm.onlineResults)
	}
}
