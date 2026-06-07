package tui

import (
	"fmt"
	"strings"
	"testing"

	"caltui/internal/domain"
)

func TestSearchScrollsToCursor(t *testing.T) {
	s := testStore(t)
	for i := 0; i < 20; i++ {
		if _, err := s.InsertFood(domain.Food{
			Source: domain.SourceCustom, Name: fmt.Sprintf("Food %02d", i),
			Per100g: domain.Macros{Kcal: 100 + float64(i)},
		}); err != nil {
			t.Fatal(err)
		}
	}
	sm := newSearchModal(s, "2026-06-06", domain.Lunch, nil)
	sm.viewHeight = 24
	sm.focus()
	typeStr(sm, "food")
	sm.Update(sm.searchCmd()()) // load results

	rows := sm.rows()
	visible := sm.visibleRows()
	if len(rows) <= visible {
		t.Fatalf("need more rows (%d) than fit (%d) to exercise scrolling", len(rows), visible)
	}

	// Navigate to the last row.
	for i := 0; i < len(rows); i++ {
		sm.Update(press("down"))
	}
	if sm.cursor != len(rows)-1 {
		t.Fatalf("cursor = %d, want %d", sm.cursor, len(rows)-1)
	}
	// The cursor must stay within the visible window, and the window must have
	// scrolled (the original bug: it never scrolled, so far rows were unreachable).
	if sm.cursor < sm.scroll || sm.cursor >= sm.scroll+visible {
		t.Errorf("cursor %d outside window [%d,%d)", sm.cursor, sm.scroll, sm.scroll+visible)
	}
	if sm.scroll == 0 {
		t.Errorf("window should have scrolled for a long list (len=%d visible=%d)", len(rows), visible)
	}

	// The rendered view must show the selected (last) item and a scroll-up hint.
	out := sm.View(90, 24)
	if last := rows[len(rows)-1].food; last == nil || !strings.Contains(out, last.Name) {
		t.Errorf("scrolled view should show the selected last item")
	}
	if !strings.Contains(out, "more") {
		t.Errorf("expected a scroll indicator ('↑ N more')")
	}

	// Scrolling back to the top resets the window.
	for i := 0; i < len(rows); i++ {
		sm.Update(press("up"))
	}
	if sm.cursor != 0 || sm.scroll != 0 {
		t.Errorf("after scrolling up, cursor=%d scroll=%d, want 0/0", sm.cursor, sm.scroll)
	}
}

func TestVisibleRowsGrowsWithHeight(t *testing.T) {
	sm := &searchModal{}
	sm.viewHeight = 0
	if got := sm.visibleRows(); got != 8 {
		t.Errorf("default visibleRows = %d, want 8", got)
	}
	sm.viewHeight = 24
	small := sm.visibleRows()
	sm.viewHeight = 50
	big := sm.visibleRows()
	if big <= small {
		t.Errorf("taller terminal should show more rows: 24->%d, 50->%d", small, big)
	}
}
