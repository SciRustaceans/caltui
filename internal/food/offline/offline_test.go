package offline

import (
	"context"
	"path/filepath"
	"testing"

	"caltui/internal/store"
)

// TestSeededOfflineSearch exercises the full Phase 3 path: write the embedded
// seed to a fresh DB, open it, and run offline searches against real USDA data.
func TestSeededOfflineSearch(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tuitracker.db")

	seeded, err := store.SeedIfMissing(dbPath)
	if err != nil {
		t.Fatalf("SeedIfMissing: %v", err)
	}
	if !seeded {
		t.Fatal("expected to seed from embedded data (run `make etl` if this fails)")
	}
	// Seeding is idempotent: a second call is a no-op because the file exists.
	if again, _ := store.SeedIfMissing(dbPath); again {
		t.Error("should not re-seed an existing database")
	}

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	n, err := s.FoodCount()
	if err != nil {
		t.Fatal(err)
	}
	if n < 5000 {
		t.Fatalf("seed has %d foods, expected several thousand", n)
	}

	p := New(s)
	if p.Name() != "offline" {
		t.Errorf("Name = %q", p.Name())
	}

	for _, q := range []string{"chicken breast", "banana", "rice white", "egg"} {
		res, err := p.Search(context.Background(), q, 10)
		if err != nil {
			t.Fatalf("Search(%q): %v", q, err)
		}
		if len(res) == 0 {
			t.Errorf("Search(%q) returned no results", q)
			continue
		}
		// Sanity: a returned food should carry real per-100g energy.
		if res[0].Per100g.Kcal <= 0 {
			t.Errorf("Search(%q)[0] %q has non-positive kcal", q, res[0].Name)
		}
	}

	// Blank query yields nothing.
	if res, _ := p.Search(context.Background(), "   ", 10); len(res) != 0 {
		t.Errorf("blank query returned %d results", len(res))
	}
}
