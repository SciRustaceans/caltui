package store

import (
	"database/sql"

	"caltui/internal/domain"
)

// mealOrder maps a meal to its diary sort position.
func mealOrder() string {
	return `CASE meal WHEN 'breakfast' THEN 0 WHEN 'lunch' THEN 1 WHEN 'dinner' THEN 2 ELSE 3 END`
}

func scanEntry(sc scanner) (domain.LogEntry, error) {
	var (
		e    domain.LogEntry
		fid  sql.NullInt64
		meal string
		unit string
	)
	err := sc.Scan(
		&e.ID, &e.Date, &meal, &fid, &e.Name,
		&e.PerUnit.Kcal, &e.PerUnit.Protein, &e.PerUnit.Carbs, &e.PerUnit.Fat,
		&e.Quantity, &unit,
	)
	if err != nil {
		return domain.LogEntry{}, err
	}
	e.Meal = domain.Meal(meal)
	e.Unit = domain.Unit(unit)
	e.FoodID = scanID(fid)
	return e, nil
}

const entryColumns = `id, date, meal, food_id, name_snapshot,
	kcal_per_unit, protein_per_unit, carbs_per_unit, fat_per_unit, quantity, unit`

// AddEntry inserts a diary entry and returns its id.
func (s *Store) AddEntry(e domain.LogEntry) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO log_entries (date, meal, food_id, name_snapshot,
			kcal_per_unit, protein_per_unit, carbs_per_unit, fat_per_unit, quantity, unit)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		e.Date, string(e.Meal), nullableID(e.FoodID), e.Name,
		e.PerUnit.Kcal, e.PerUnit.Protein, e.PerUnit.Carbs, e.PerUnit.Fat,
		e.Quantity, string(e.Unit),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateEntry updates the mutable fields of a diary entry (meal, quantity, unit,
// and the macro snapshot/name).
func (s *Store) UpdateEntry(e domain.LogEntry) error {
	_, err := s.db.Exec(`
		UPDATE log_entries SET
			date = ?, meal = ?, food_id = ?, name_snapshot = ?,
			kcal_per_unit = ?, protein_per_unit = ?, carbs_per_unit = ?, fat_per_unit = ?,
			quantity = ?, unit = ?
		WHERE id = ?`,
		e.Date, string(e.Meal), nullableID(e.FoodID), e.Name,
		e.PerUnit.Kcal, e.PerUnit.Protein, e.PerUnit.Carbs, e.PerUnit.Fat,
		e.Quantity, string(e.Unit), e.ID,
	)
	return err
}

// DeleteEntry removes a diary entry by id.
func (s *Store) DeleteEntry(id int64) error {
	_, err := s.db.Exec(`DELETE FROM log_entries WHERE id = ?`, id)
	return err
}

// EntriesForDate returns all diary entries for a date, ordered by meal then
// insertion order.
func (s *Store) EntriesForDate(date string) ([]domain.LogEntry, error) {
	rows, err := s.db.Query(`SELECT `+entryColumns+`
		FROM log_entries WHERE date = ?
		ORDER BY `+mealOrder()+`, id`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.LogEntry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// DayTotals sums the macro totals (quantity * per-unit) for a date.
func (s *Store) DayTotals(date string) (domain.Macros, error) {
	var m domain.Macros
	err := s.db.QueryRow(`
		SELECT
			COALESCE(SUM(quantity * kcal_per_unit), 0),
			COALESCE(SUM(quantity * protein_per_unit), 0),
			COALESCE(SUM(quantity * carbs_per_unit), 0),
			COALESCE(SUM(quantity * fat_per_unit), 0)
		FROM log_entries WHERE date = ?`, date).
		Scan(&m.Kcal, &m.Protein, &m.Carbs, &m.Fat)
	return m, err
}

// CalorieSeries returns total calories per day for dates in [from, to]
// inclusive, as a map keyed by YYYY-MM-DD. Missing days are absent (callers fill
// with zero as needed).
func (s *Store) CalorieSeries(from, to string) (map[string]float64, error) {
	rows, err := s.db.Query(`
		SELECT date, COALESCE(SUM(quantity * kcal_per_unit), 0)
		FROM log_entries WHERE date BETWEEN ? AND ?
		GROUP BY date`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]float64)
	for rows.Next() {
		var d string
		var kcal float64
		if err := rows.Scan(&d, &kcal); err != nil {
			return nil, err
		}
		out[d] = kcal
	}
	return out, rows.Err()
}

// RecentFoods returns distinct foods most recently logged (food_id not null),
// for quick re-logging.
func (s *Store) RecentFoods(limit int) ([]domain.Food, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(`
		SELECT `+foodColumns+`
		FROM foods f
		JOIN (
			SELECT food_id, MAX(id) AS last_id
			FROM log_entries WHERE food_id IS NOT NULL
			GROUP BY food_id
		) le ON le.food_id = f.id
		ORDER BY le.last_id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectFoods(rows)
}

// FrequentFoods returns foods ordered by how often they have been logged.
func (s *Store) FrequentFoods(limit int) ([]domain.Food, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(`
		SELECT `+foodColumns+`
		FROM foods f
		JOIN (
			SELECT food_id, COUNT(*) AS n
			FROM log_entries WHERE food_id IS NOT NULL
			GROUP BY food_id
		) le ON le.food_id = f.id
		ORDER BY le.n DESC, f.name COLLATE NOCASE
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectFoods(rows)
}

func collectFoods(rows *sql.Rows) ([]domain.Food, error) {
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

// CopyDay copies every entry from one date to another, preserving meals and
// snapshots. Returns the number of entries copied.
func (s *Store) CopyDay(from, to string) (int, error) {
	return s.copyEntries(from, to, "")
}

// CopyMeal copies the entries of a single meal from one date to another.
func (s *Store) CopyMeal(from, to string, meal domain.Meal) (int, error) {
	return s.copyEntries(from, to, meal)
}

func (s *Store) copyEntries(from, to string, meal domain.Meal) (int, error) {
	q := `
		INSERT INTO log_entries (date, meal, food_id, name_snapshot,
			kcal_per_unit, protein_per_unit, carbs_per_unit, fat_per_unit, quantity, unit)
		SELECT ?, meal, food_id, name_snapshot,
			kcal_per_unit, protein_per_unit, carbs_per_unit, fat_per_unit, quantity, unit
		FROM log_entries WHERE date = ?`
	args := []any{to, from}
	if meal != "" {
		q += ` AND meal = ?`
		args = append(args, string(meal))
	}
	res, err := s.db.Exec(q, args...)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
