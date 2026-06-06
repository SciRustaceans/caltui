-- Foods reference table + full-text search. This migration is the schema the
-- bundled USDA seed database is built against; a fresh user DB without a seed
-- gets the same (empty) tables. Macros are stored per 100 g (canonical basis).

CREATE TABLE foods (
    id           INTEGER PRIMARY KEY,
    source       TEXT NOT NULL CHECK (source IN ('usda_offline','usda_online','custom')),
    fdc_id       INTEGER,
    name         TEXT NOT NULL,
    brand        TEXT NOT NULL DEFAULT '',
    kcal_100g    REAL NOT NULL,
    protein_100g REAL NOT NULL,
    carbs_100g   REAL NOT NULL,
    fat_100g     REAL NOT NULL,
    serving_size REAL NOT NULL DEFAULT 0,
    serving_unit TEXT NOT NULL DEFAULT '',
    household    TEXT NOT NULL DEFAULT '',
    density      REAL NOT NULL DEFAULT 0,
    created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_foods_name ON foods (name COLLATE NOCASE);
CREATE UNIQUE INDEX idx_foods_fdc ON foods (fdc_id) WHERE fdc_id IS NOT NULL;

-- External-content FTS5 index over name+brand. Kept in sync with the foods
-- table via triggers (the canonical FTS5 external-content pattern).
CREATE VIRTUAL TABLE foods_fts USING fts5(name, brand, content='foods', content_rowid='id');

CREATE TRIGGER foods_ai AFTER INSERT ON foods BEGIN
    INSERT INTO foods_fts(rowid, name, brand) VALUES (new.id, new.name, new.brand);
END;

CREATE TRIGGER foods_ad AFTER DELETE ON foods BEGIN
    INSERT INTO foods_fts(foods_fts, rowid, name, brand) VALUES ('delete', old.id, old.name, old.brand);
END;

CREATE TRIGGER foods_au AFTER UPDATE ON foods BEGIN
    INSERT INTO foods_fts(foods_fts, rowid, name, brand) VALUES ('delete', old.id, old.name, old.brand);
    INSERT INTO foods_fts(rowid, name, brand) VALUES (new.id, new.name, new.brand);
END;
