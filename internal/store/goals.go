package store

import (
	"database/sql"
	"errors"

	"caltui/internal/domain"
)

// AddGoal inserts a dated goal and returns its id. The activity level is stored
// as its numeric multiplier (activity_factor).
func (s *Store) AddGoal(g domain.Goal) (int64, error) {
	manual := 0
	if g.Manual {
		manual = 1
	}
	res, err := s.db.Exec(`
		INSERT INTO goals (effective_date, kcal_target, protein_g, carbs_g, fat_g,
			sex, birth_date, height_cm, weight_kg, activity_factor, goal_rate, is_manual_override)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		g.EffectiveDate, g.Target.Kcal, g.Target.Protein, g.Target.Carbs, g.Target.Fat,
		string(g.Sex), g.BirthDate, g.HeightCm, g.WeightKg, g.Activity.Multiplier(), g.GoalRate, manual,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// CurrentGoal returns the goal in force on date: the one with the greatest
// effective_date that is on or before date. The bool is false if none exists.
func (s *Store) CurrentGoal(date string) (domain.Goal, bool, error) {
	var (
		g      domain.Goal
		sex    string
		factor float64
		manual int
	)
	err := s.db.QueryRow(`
		SELECT id, effective_date, kcal_target, protein_g, carbs_g, fat_g,
			sex, birth_date, height_cm, weight_kg, activity_factor, goal_rate, is_manual_override
		FROM goals
		WHERE effective_date <= ?
		ORDER BY effective_date DESC, id DESC
		LIMIT 1`, date).
		Scan(&g.ID, &g.EffectiveDate, &g.Target.Kcal, &g.Target.Protein, &g.Target.Carbs, &g.Target.Fat,
			&sex, &g.BirthDate, &g.HeightCm, &g.WeightKg, &factor, &g.GoalRate, &manual)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Goal{}, false, nil
	}
	if err != nil {
		return domain.Goal{}, false, err
	}
	g.Sex = domain.Sex(sex)
	g.Manual = manual != 0
	if lvl, ok := domain.ActivityLevelForMultiplier(factor); ok {
		g.Activity = lvl
	}
	return g, true, nil
}

// HasGoal reports whether any goal exists (used for first-run detection).
func (s *Store) HasGoal() (bool, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM goals`).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}
