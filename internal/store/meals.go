package store

import (
	"database/sql"

	"caltui/internal/domain"
)

const mealItemColumns = `food_id, name_snapshot,
	kcal_per_unit, protein_per_unit, carbs_per_unit, fat_per_unit, quantity, unit`

func scanMealItem(sc scanner) (domain.SavedMealItem, int64, error) {
	var (
		it     domain.SavedMealItem
		mealID int64
		fid    sql.NullInt64
		unit   string
	)
	err := sc.Scan(&mealID, &fid, &it.Name,
		&it.PerUnit.Kcal, &it.PerUnit.Protein, &it.PerUnit.Carbs, &it.PerUnit.Fat,
		&it.Quantity, &unit)
	if err != nil {
		return domain.SavedMealItem{}, 0, err
	}
	it.FoodID = scanID(fid)
	it.Unit = domain.Unit(unit)
	return it, mealID, nil
}

// AddSavedMeal inserts a saved meal and its items in one transaction.
func (s *Store) AddSavedMeal(m domain.SavedMeal) (int64, error) {
	kind := m.Kind
	if kind == "" {
		kind = "meal"
	}
	servings := m.Servings
	if servings <= 0 {
		servings = 1
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(`INSERT INTO saved_meals (name, kind, servings) VALUES (?,?,?)`, m.Name, kind, servings)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	stmt, err := tx.Prepare(`
		INSERT INTO saved_meal_items (meal_id, food_id, name_snapshot,
			kcal_per_unit, protein_per_unit, carbs_per_unit, fat_per_unit, quantity, unit)
		VALUES (?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for _, it := range m.Items {
		if _, err := stmt.Exec(id, nullableID(it.FoodID), it.Name,
			it.PerUnit.Kcal, it.PerUnit.Protein, it.PerUnit.Carbs, it.PerUnit.Fat,
			it.Quantity, string(it.Unit)); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

// ListSavedMeals returns all saved meals with their items, ordered by name.
func (s *Store) ListSavedMeals() ([]domain.SavedMeal, error) {
	rows, err := s.db.Query(`SELECT id, name, kind, servings FROM saved_meals ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var meals []domain.SavedMeal
	idx := make(map[int64]int)
	for rows.Next() {
		var m domain.SavedMeal
		if err := rows.Scan(&m.ID, &m.Name, &m.Kind, &m.Servings); err != nil {
			return nil, err
		}
		idx[m.ID] = len(meals)
		meals = append(meals, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(meals) == 0 {
		return nil, nil
	}

	itemRows, err := s.db.Query(`SELECT meal_id, ` + mealItemColumns + ` FROM saved_meal_items ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer itemRows.Close()
	for itemRows.Next() {
		it, mealID, err := scanMealItem(itemRows)
		if err != nil {
			return nil, err
		}
		if i, ok := idx[mealID]; ok {
			meals[i].Items = append(meals[i].Items, it)
		}
	}
	return meals, itemRows.Err()
}

// LogSavedMeal logs every item of a saved meal into date/meal as diary entries
// (snapshotting macros). Returns the number of entries created.
func (s *Store) LogSavedMeal(id int64, date string, meal domain.Meal) (int, error) {
	rows, err := s.db.Query(`SELECT meal_id, `+mealItemColumns+` FROM saved_meal_items WHERE meal_id = ?`, id)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var items []domain.SavedMealItem
	for rows.Next() {
		it, _, err := scanMealItem(rows)
		if err != nil {
			return 0, err
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.Prepare(`
		INSERT INTO log_entries (date, meal, food_id, name_snapshot,
			kcal_per_unit, protein_per_unit, carbs_per_unit, fat_per_unit, quantity, unit)
		VALUES (?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for _, it := range items {
		if _, err := stmt.Exec(date, string(meal), nullableID(it.FoodID), it.Name,
			it.PerUnit.Kcal, it.PerUnit.Protein, it.PerUnit.Carbs, it.PerUnit.Fat,
			it.Quantity, string(it.Unit)); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(items), nil
}

// DeleteSavedMeal removes a saved meal; its items cascade.
func (s *Store) DeleteSavedMeal(id int64) error {
	_, err := s.db.Exec(`DELETE FROM saved_meals WHERE id = ?`, id)
	return err
}
