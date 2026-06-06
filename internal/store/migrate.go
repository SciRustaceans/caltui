package store

import (
	"database/sql"
	"embed"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type migration struct {
	version int
	name    string
	sql     string
}

// loadMigrations reads and orders the embedded migration files. File names must
// start with a zero-padded version number followed by '_', e.g. 0001_foods.sql.
func loadMigrations() ([]migration, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	var migs []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		prefix, _, ok := strings.Cut(e.Name(), "_")
		if !ok {
			return nil, fmt.Errorf("migration %q missing version prefix", e.Name())
		}
		v, err := strconv.Atoi(prefix)
		if err != nil {
			return nil, fmt.Errorf("migration %q has non-numeric version: %w", e.Name(), err)
		}
		body, err := migrationsFS.ReadFile(path.Join("migrations", e.Name()))
		if err != nil {
			return nil, err
		}
		migs = append(migs, migration{version: v, name: e.Name(), sql: string(body)})
	}
	sort.Slice(migs, func(i, j int) bool { return migs[i].version < migs[j].version })
	for i := 1; i < len(migs); i++ {
		if migs[i].version == migs[i-1].version {
			return nil, fmt.Errorf("duplicate migration version %d", migs[i].version)
		}
	}
	return migs, nil
}

// latestMigrationVersion returns the highest embedded migration version.
func latestMigrationVersion() (int, error) {
	migs, err := loadMigrations()
	if err != nil {
		return 0, err
	}
	if len(migs) == 0 {
		return 0, nil
	}
	return migs[len(migs)-1].version, nil
}

// schemaVersion reads PRAGMA user_version.
func schemaVersion(q interface {
	QueryRow(string, ...any) *sql.Row
}) (int, error) {
	var v int
	if err := q.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return 0, err
	}
	return v, nil
}

// migrate applies every migration whose version is greater than the database's
// current user_version, in order, each in its own transaction. It is a no-op
// when the database is already current (e.g. a freshly copied seed).
func migrate(db *sql.DB) error {
	migs, err := loadMigrations()
	if err != nil {
		return err
	}
	current, err := schemaVersion(db)
	if err != nil {
		return fmt.Errorf("reading schema version: %w", err)
	}
	for _, m := range migs {
		if m.version <= current {
			continue
		}
		if err := applyMigration(db, m); err != nil {
			return fmt.Errorf("migration %s: %w", m.name, err)
		}
	}
	return nil
}

func applyMigration(db *sql.DB, m migration) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(m.sql); err != nil {
		return err
	}
	// PRAGMA user_version does not accept bound parameters; the version is an
	// integer parsed from a trusted file name, so formatting is safe.
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", m.version)); err != nil {
		return err
	}
	return tx.Commit()
}
