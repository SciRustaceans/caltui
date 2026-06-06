-- Mutable application tables: the diary, saved meals/recipes, goals, and weight.

CREATE TABLE log_entries (
    id               INTEGER PRIMARY KEY,
    date             TEXT NOT NULL,
    meal             TEXT NOT NULL CHECK (meal IN ('breakfast','lunch','dinner','snacks')),
    food_id          INTEGER REFERENCES foods(id) ON DELETE SET NULL,
    name_snapshot    TEXT NOT NULL,
    kcal_per_unit    REAL NOT NULL,
    protein_per_unit REAL NOT NULL,
    carbs_per_unit   REAL NOT NULL,
    fat_per_unit     REAL NOT NULL,
    quantity         REAL NOT NULL,
    unit             TEXT NOT NULL,
    created_at       TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_log_date ON log_entries (date);
CREATE INDEX idx_log_date_meal ON log_entries (date, meal);
CREATE INDEX idx_log_food ON log_entries (food_id);

CREATE TABLE saved_meals (
    id         INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    kind       TEXT NOT NULL DEFAULT 'meal' CHECK (kind IN ('meal','recipe')),
    servings   REAL NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE saved_meal_items (
    id               INTEGER PRIMARY KEY,
    meal_id          INTEGER NOT NULL REFERENCES saved_meals(id) ON DELETE CASCADE,
    food_id          INTEGER REFERENCES foods(id) ON DELETE SET NULL,
    name_snapshot    TEXT NOT NULL,
    kcal_per_unit    REAL NOT NULL,
    protein_per_unit REAL NOT NULL,
    carbs_per_unit   REAL NOT NULL,
    fat_per_unit     REAL NOT NULL,
    quantity         REAL NOT NULL,
    unit             TEXT NOT NULL
);
CREATE INDEX idx_smi_meal ON saved_meal_items (meal_id);

CREATE TABLE goals (
    id                 INTEGER PRIMARY KEY,
    effective_date     TEXT NOT NULL,
    kcal_target        REAL NOT NULL,
    protein_g          REAL NOT NULL,
    carbs_g            REAL NOT NULL,
    fat_g              REAL NOT NULL,
    sex                TEXT NOT NULL DEFAULT '',
    birth_date         TEXT NOT NULL DEFAULT '',
    height_cm          REAL NOT NULL DEFAULT 0,
    weight_kg          REAL NOT NULL DEFAULT 0,
    activity_factor    REAL NOT NULL DEFAULT 0,
    goal_rate          REAL NOT NULL DEFAULT 0,
    is_manual_override INTEGER NOT NULL DEFAULT 0,
    created_at         TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_goals_eff ON goals (effective_date);

CREATE TABLE weight_log (
    id         INTEGER PRIMARY KEY,
    date       TEXT NOT NULL UNIQUE,
    weight_kg  REAL NOT NULL,
    unit       TEXT NOT NULL DEFAULT 'kg',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE weight_goal (
    id           INTEGER PRIMARY KEY CHECK (id = 1),
    target_kg    REAL NOT NULL,
    unit         TEXT NOT NULL DEFAULT 'kg',
    rate_per_week REAL NOT NULL DEFAULT 0,
    start_date   TEXT NOT NULL DEFAULT '',
    start_kg     REAL NOT NULL DEFAULT 0
);
