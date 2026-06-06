package store

import (
	"database/sql"
	"errors"
	"strings"

	"caltui/internal/domain"
)

// foodColumns is the fixed SELECT column order consumed by scanFood. Columns
// are qualified with the alias "f" (the foods table must be aliased f) so they
// never collide with the foods_fts virtual table's name/brand columns in joins.
const foodColumns = `f.id, f.source, f.fdc_id, f.name, f.brand,
	f.kcal_100g, f.protein_100g, f.carbs_100g, f.fat_100g,
	f.serving_size, f.serving_unit, f.household, f.density`

type scanner interface{ Scan(...any) error }

func scanFood(sc scanner) (domain.Food, error) {
	var (
		f    domain.Food
		fdc  sql.NullInt64
		src  string
		unit string
	)
	err := sc.Scan(
		&f.ID, &src, &fdc, &f.Name, &f.Brand,
		&f.Per100g.Kcal, &f.Per100g.Protein, &f.Per100g.Carbs, &f.Per100g.Fat,
		&f.ServingSize, &unit, &f.Household, &f.Density,
	)
	if err != nil {
		return domain.Food{}, err
	}
	f.Source = domain.FoodSource(src)
	f.ServingUnit = domain.Unit(unit)
	f.FDCID = scanID(fdc)
	return f, nil
}

// InsertFood inserts a food and returns its new id. The FTS index is maintained
// automatically by triggers.
func (s *Store) InsertFood(f domain.Food) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO foods (source, fdc_id, name, brand,
			kcal_100g, protein_100g, carbs_100g, fat_100g,
			serving_size, serving_unit, household, density)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		string(f.Source), nullableID(f.FDCID), f.Name, f.Brand,
		f.Per100g.Kcal, f.Per100g.Protein, f.Per100g.Carbs, f.Per100g.Fat,
		f.ServingSize, string(f.ServingUnit), f.Household, f.Density,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetFood returns the food with the given id. The bool is false if not found.
func (s *Store) GetFood(id int64) (domain.Food, bool, error) {
	f, err := scanFood(s.db.QueryRow(`SELECT `+foodColumns+` FROM foods f WHERE f.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Food{}, false, nil
	}
	if err != nil {
		return domain.Food{}, false, err
	}
	return f, true, nil
}

// DeleteFood removes a food. Diary entries that referenced it keep their macro
// snapshot; their food_id becomes NULL (ON DELETE SET NULL).
func (s *Store) DeleteFood(id int64) error {
	_, err := s.db.Exec(`DELETE FROM foods WHERE id = ?`, id)
	return err
}

// FoodCount returns the number of rows in the foods table.
func (s *Store) FoodCount() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM foods`).Scan(&n)
	return n, err
}

// SearchFoods runs a prefix full-text search over food name + brand, ordered by
// FTS relevance. An empty/blank query returns no rows (callers should show
// recent/frequent instead).
func (s *Store) SearchFoods(query string, limit int) ([]domain.Food, error) {
	match := ftsPrefixQuery(query)
	if match == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 25
	}
	rows, err := s.db.Query(`
		SELECT `+foodColumns+`
		FROM foods_fts fts
		JOIN foods f ON f.id = fts.rowid
		WHERE foods_fts MATCH ?
		ORDER BY rank
		LIMIT ?`,
		match, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Food
	for rows.Next() {
		f, err := scanFood(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// ftsPrefixQuery turns free user text into a safe FTS5 prefix MATCH expression:
// each whitespace token becomes a quoted prefix term, e.g. `chick bre` ->
// `"chick"* "bre"*` (implicit AND). Quoting makes apostrophes/punctuation safe.
func ftsPrefixQuery(s string) string {
	var terms []string
	for _, tok := range strings.Fields(s) {
		// Keep only alphanumerics within a token; drop tokens that empty out.
		var b strings.Builder
		for _, r := range tok {
			if r == '\'' || r == '"' {
				continue
			}
			b.WriteRune(r)
		}
		t := strings.TrimSpace(b.String())
		if t == "" {
			continue
		}
		terms = append(terms, `"`+t+`"*`)
	}
	return strings.Join(terms, " ")
}
