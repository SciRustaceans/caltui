package store

import (
	"database/sql"
	"errors"

	"caltui/internal/domain"
)

// UpsertWeight records a weigh-in, replacing any existing entry for the same
// date (one weigh-in per day).
func (s *Store) UpsertWeight(w domain.Weight) error {
	unit := w.Unit
	if unit == "" {
		unit = "kg"
	}
	_, err := s.db.Exec(`
		INSERT INTO weight_log (date, weight_kg, unit) VALUES (?,?,?)
		ON CONFLICT(date) DO UPDATE SET weight_kg = excluded.weight_kg, unit = excluded.unit`,
		w.Date, w.Kg, unit)
	return err
}

// DeleteWeight removes the weigh-in for a date, if any.
func (s *Store) DeleteWeight(date string) error {
	_, err := s.db.Exec(`DELETE FROM weight_log WHERE date = ?`, date)
	return err
}

func scanWeight(sc scanner) (domain.Weight, error) {
	var w domain.Weight
	err := sc.Scan(&w.ID, &w.Date, &w.Kg, &w.Unit)
	return w, err
}

// LatestWeight returns the most recent weigh-in. The bool is false if none.
func (s *Store) LatestWeight() (domain.Weight, bool, error) {
	w, err := scanWeight(s.db.QueryRow(`
		SELECT id, date, weight_kg, unit FROM weight_log ORDER BY date DESC LIMIT 1`))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Weight{}, false, nil
	}
	if err != nil {
		return domain.Weight{}, false, err
	}
	return w, true, nil
}

// WeightSeries returns weigh-ins between from and to (inclusive), oldest first.
func (s *Store) WeightSeries(from, to string) ([]domain.Weight, error) {
	rows, err := s.db.Query(`
		SELECT id, date, weight_kg, unit FROM weight_log
		WHERE date BETWEEN ? AND ? ORDER BY date`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Weight
	for rows.Next() {
		w, err := scanWeight(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// GetWeightGoal returns the single weight goal row. The bool is false if unset.
func (s *Store) GetWeightGoal() (domain.WeightGoal, bool, error) {
	var g domain.WeightGoal
	err := s.db.QueryRow(`
		SELECT target_kg, unit, rate_per_week, start_date, start_kg
		FROM weight_goal WHERE id = 1`).
		Scan(&g.TargetKg, &g.Unit, &g.RatePerWeek, &g.StartDate, &g.StartKg)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.WeightGoal{}, false, nil
	}
	if err != nil {
		return domain.WeightGoal{}, false, err
	}
	return g, true, nil
}

// SetWeightGoal upserts the single weight goal row.
func (s *Store) SetWeightGoal(g domain.WeightGoal) error {
	unit := g.Unit
	if unit == "" {
		unit = "kg"
	}
	_, err := s.db.Exec(`
		INSERT INTO weight_goal (id, target_kg, unit, rate_per_week, start_date, start_kg)
		VALUES (1, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			target_kg = excluded.target_kg, unit = excluded.unit,
			rate_per_week = excluded.rate_per_week,
			start_date = excluded.start_date, start_kg = excluded.start_kg`,
		g.TargetKg, unit, g.RatePerWeek, g.StartDate, g.StartKg)
	return err
}
