package store

import (
	"path/filepath"
	"testing"

	"caltui/internal/domain"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func ptr(i int64) *int64 { return &i }

func TestMigrateAndPragmas(t *testing.T) {
	s := openTestStore(t)

	want, err := latestMigrationVersion()
	if err != nil {
		t.Fatal(err)
	}
	var uv int
	if err := s.DB().QueryRow("PRAGMA user_version").Scan(&uv); err != nil {
		t.Fatal(err)
	}
	if uv != want {
		t.Errorf("user_version = %d, want %d", uv, want)
	}

	var jm string
	if err := s.DB().QueryRow("PRAGMA journal_mode").Scan(&jm); err != nil {
		t.Fatal(err)
	}
	if jm != "wal" {
		t.Errorf("journal_mode = %q, want wal", jm)
	}

	var fk int
	if err := s.DB().QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}

	// Re-opening an existing (current) DB must be a no-op migration.
	if err := migrate(s.DB()); err != nil {
		t.Errorf("re-migrate should be no-op: %v", err)
	}
}

func seedFood(t *testing.T, s *Store, name string, m domain.Macros) int64 {
	t.Helper()
	id, err := s.InsertFood(domain.Food{Source: domain.SourceCustom, Name: name, Per100g: m})
	if err != nil {
		t.Fatalf("InsertFood(%s): %v", name, err)
	}
	return id
}

func TestFoodsCRUDAndFTS(t *testing.T) {
	s := openTestStore(t)
	chickenID := seedFood(t, s, "Chicken breast, cooked", domain.Macros{Kcal: 165, Protein: 31, Fat: 3.6})
	seedFood(t, s, "Chickpeas, canned", domain.Macros{Kcal: 139, Protein: 7.3, Carbs: 22.5, Fat: 2.6})
	seedFood(t, s, "Rice, white, cooked", domain.Macros{Kcal: 130, Protein: 2.7, Carbs: 28})

	if n, err := s.FoodCount(); err != nil || n != 3 {
		t.Fatalf("FoodCount = %d, %v; want 3", n, err)
	}

	got, ok, err := s.GetFood(chickenID)
	if err != nil || !ok {
		t.Fatalf("GetFood: ok=%v err=%v", ok, err)
	}
	if got.Name != "Chicken breast, cooked" || got.Per100g.Protein != 31 {
		t.Errorf("GetFood returned %+v", got)
	}
	if _, ok, _ := s.GetFood(9999); ok {
		t.Error("GetFood of missing id should be not-found")
	}

	// Prefix FTS: "chick" matches both chicken foods, not rice.
	res, err := s.SearchFoods("chick", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 {
		t.Fatalf("SearchFoods(chick) = %d results, want 2: %+v", len(res), res)
	}
	// Two-term prefix narrows to chicken breast.
	res, _ = s.SearchFoods("chick bre", 10)
	if len(res) != 1 || res[0].ID != chickenID {
		t.Errorf("SearchFoods(chick bre) = %+v, want only chicken breast", res)
	}
	// Blank query returns nothing.
	if res, _ := s.SearchFoods("   ", 10); res != nil {
		t.Errorf("blank query should return nil, got %+v", res)
	}

	// Deleting a food keeps FTS consistent.
	if err := s.DeleteFood(chickenID); err != nil {
		t.Fatal(err)
	}
	if res, _ := s.SearchFoods("chick bre", 10); len(res) != 0 {
		t.Errorf("after delete, search should not find chicken breast: %+v", res)
	}
}

func TestDiary(t *testing.T) {
	s := openTestStore(t)
	chick := seedFood(t, s, "Chicken", domain.Macros{Kcal: 165, Protein: 31})
	rice := seedFood(t, s, "Rice", domain.Macros{Kcal: 130, Carbs: 28})

	const day = "2026-06-06"
	// 150 g chicken at lunch (per-unit = per gram).
	if _, err := s.AddEntry(domain.LogEntry{
		Date: day, Meal: domain.Lunch, FoodID: ptr(chick), Name: "Chicken",
		PerUnit: domain.Macros{Kcal: 1.65, Protein: 0.31}, Quantity: 150, Unit: domain.UnitGram,
	}); err != nil {
		t.Fatal(err)
	}
	bID, err := s.AddEntry(domain.LogEntry{
		Date: day, Meal: domain.Breakfast, FoodID: ptr(rice), Name: "Rice",
		PerUnit: domain.Macros{Kcal: 1.30, Carbs: 0.28}, Quantity: 100, Unit: domain.UnitGram,
	})
	if err != nil {
		t.Fatal(err)
	}

	entries, err := s.EntriesForDate(day)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("EntriesForDate = %d, want 2", len(entries))
	}
	// Breakfast must sort before lunch.
	if entries[0].Meal != domain.Breakfast || entries[1].Meal != domain.Lunch {
		t.Errorf("meal ordering wrong: %q then %q", entries[0].Meal, entries[1].Meal)
	}

	tot, err := s.DayTotals(day)
	if err != nil {
		t.Fatal(err)
	}
	// 150*1.65 + 100*1.30 = 247.5 + 130 = 377.5
	if tot.Kcal != 377.5 {
		t.Errorf("DayTotals kcal = %g, want 377.5", tot.Kcal)
	}

	// Update breakfast rice to 200 g.
	bEntry := entries[0]
	bEntry.Quantity = 200
	if err := s.UpdateEntry(bEntry); err != nil {
		t.Fatal(err)
	}
	tot, _ = s.DayTotals(day)
	if tot.Kcal != 247.5+260 {
		t.Errorf("after update kcal = %g, want %g", tot.Kcal, 247.5+260)
	}

	// Recent and frequent.
	recent, _ := s.RecentFoods(10)
	if len(recent) != 2 {
		t.Errorf("RecentFoods = %d, want 2", len(recent))
	}
	// Log chicken again so it becomes most frequent.
	_, _ = s.AddEntry(domain.LogEntry{Date: day, Meal: domain.Dinner, FoodID: ptr(chick), Name: "Chicken",
		PerUnit: domain.Macros{Kcal: 1.65}, Quantity: 100, Unit: domain.UnitGram})
	freq, _ := s.FrequentFoods(10)
	if len(freq) == 0 || freq[0].ID != chick {
		t.Errorf("FrequentFoods[0] = %+v, want chicken", freq)
	}

	// Copy the whole day forward.
	n, err := s.CopyDay(day, "2026-06-07")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("CopyDay copied %d, want 3", n)
	}
	if e2, _ := s.EntriesForDate("2026-06-07"); len(e2) != 3 {
		t.Errorf("copied day has %d entries, want 3", len(e2))
	}

	// Copy a single meal.
	n, _ = s.CopyMeal(day, "2026-06-08", domain.Lunch)
	if n != 1 {
		t.Errorf("CopyMeal copied %d, want 1", n)
	}

	// Calorie series spans logged days.
	series, _ := s.CalorieSeries("2026-06-01", "2026-06-30")
	if series[day] == 0 {
		t.Errorf("CalorieSeries missing %s", day)
	}

	// Delete an entry.
	if err := s.DeleteEntry(bID); err != nil {
		t.Fatal(err)
	}
}

func TestOnDeleteSetNull(t *testing.T) {
	s := openTestStore(t)
	fid := seedFood(t, s, "Temp food", domain.Macros{Kcal: 100})
	eid, err := s.AddEntry(domain.LogEntry{
		Date: "2026-06-06", Meal: domain.Snacks, FoodID: ptr(fid), Name: "Temp food",
		PerUnit: domain.Macros{Kcal: 1}, Quantity: 50, Unit: domain.UnitGram,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteFood(fid); err != nil {
		t.Fatal(err)
	}
	var fk any
	if err := s.DB().QueryRow("SELECT food_id FROM log_entries WHERE id = ?", eid).Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if fk != nil {
		t.Errorf("food_id = %v, want NULL after food delete (ON DELETE SET NULL)", fk)
	}
	// The snapshot survives.
	var name string
	_ = s.DB().QueryRow("SELECT name_snapshot FROM log_entries WHERE id = ?", eid).Scan(&name)
	if name != "Temp food" {
		t.Errorf("snapshot lost: %q", name)
	}
}

func TestGoals(t *testing.T) {
	s := openTestStore(t)
	if ok, _ := s.HasGoal(); ok {
		t.Error("fresh DB should have no goal")
	}
	if _, ok, _ := s.CurrentGoal("2026-06-06"); ok {
		t.Error("CurrentGoal should be not-found on fresh DB")
	}

	_, _ = s.AddGoal(domain.Goal{
		EffectiveDate: "2026-01-01", Target: domain.Macros{Kcal: 2000, Protein: 150, Carbs: 200, Fat: 60},
		Sex: domain.Male, Activity: domain.ModeratelyActive, GoalRate: -0.5,
	})
	_, _ = s.AddGoal(domain.Goal{
		EffectiveDate: "2026-05-01", Target: domain.Macros{Kcal: 2200, Protein: 160, Carbs: 220, Fat: 70},
		Sex: domain.Male, Activity: domain.VeryActive, GoalRate: 0, Manual: true,
	})

	g, ok, err := s.CurrentGoal("2026-06-06")
	if err != nil || !ok {
		t.Fatalf("CurrentGoal: ok=%v err=%v", ok, err)
	}
	if g.Target.Kcal != 2200 {
		t.Errorf("CurrentGoal kcal = %g, want 2200 (latest effective)", g.Target.Kcal)
	}
	if g.Activity != domain.VeryActive {
		t.Errorf("activity round-trip = %q, want very_active", g.Activity)
	}
	if !g.Manual {
		t.Error("Manual flag did not round-trip")
	}
	// A date before the second goal returns the first.
	g, _, _ = s.CurrentGoal("2026-04-01")
	if g.Target.Kcal != 2000 {
		t.Errorf("CurrentGoal(2026-04-01) kcal = %g, want 2000", g.Target.Kcal)
	}
	if ok, _ := s.HasGoal(); !ok {
		t.Error("HasGoal should be true")
	}
}

func TestWeight(t *testing.T) {
	s := openTestStore(t)
	if _, ok, _ := s.LatestWeight(); ok {
		t.Error("no weight expected on fresh DB")
	}
	if err := s.UpsertWeight(domain.Weight{Date: "2026-06-05", Kg: 82.5}); err != nil {
		t.Fatal(err)
	}
	// Same-day upsert replaces, not duplicates.
	if err := s.UpsertWeight(domain.Weight{Date: "2026-06-05", Kg: 82.1}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertWeight(domain.Weight{Date: "2026-06-06", Kg: 81.9}); err != nil {
		t.Fatal(err)
	}
	series, _ := s.WeightSeries("2026-06-01", "2026-06-30")
	if len(series) != 2 {
		t.Fatalf("WeightSeries = %d, want 2 (upsert dedup)", len(series))
	}
	if series[0].Kg != 82.1 {
		t.Errorf("upserted weight = %g, want 82.1", series[0].Kg)
	}
	latest, ok, _ := s.LatestWeight()
	if !ok || latest.Date != "2026-06-06" {
		t.Errorf("LatestWeight = %+v", latest)
	}

	if _, ok, _ := s.GetWeightGoal(); ok {
		t.Error("no weight goal expected initially")
	}
	if err := s.SetWeightGoal(domain.WeightGoal{TargetKg: 78, RatePerWeek: -0.5, StartDate: "2026-06-01", StartKg: 82.5}); err != nil {
		t.Fatal(err)
	}
	// Upsert again (single-row constraint).
	if err := s.SetWeightGoal(domain.WeightGoal{TargetKg: 77, RatePerWeek: -0.4, StartDate: "2026-06-01", StartKg: 82.5}); err != nil {
		t.Fatal(err)
	}
	wg, ok, _ := s.GetWeightGoal()
	if !ok || wg.TargetKg != 77 {
		t.Errorf("GetWeightGoal = %+v, want target 77", wg)
	}
}
