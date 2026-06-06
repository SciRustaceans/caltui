// Package store is the persistence layer: a thin repository over a local SQLite
// database (pure-Go modernc.org/sqlite driver). It owns the schema, migrations,
// and all queries. All macro values are stored as plain REAL columns.
package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver
)

// Store wraps the database handle and exposes repository methods.
type Store struct {
	db *sql.DB
}

// dsn builds a modernc.org/sqlite DSN with the pragmas we want on every
// connection: WAL journaling, enforced foreign keys, a busy timeout, and
// NORMAL synchronous mode (modernc uses the _pragma=name(value) form).
//
// The raw OS path is used WITHOUT a "file:" prefix: modernc only treats the DSN
// as a URI when it starts with "file:", and a file: URI built from a Windows
// path (drive letter + backslashes) is invalid. Without the prefix modernc
// passes the path to SQLite verbatim and still applies the _pragma params, which
// is correct on Windows, macOS, and Linux alike.
func dsn(path string) string {
	return path +
		"?_pragma=journal_mode(wal)" +
		"&_pragma=foreign_keys(1)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=synchronous(1)"
}

// Open opens (creating if needed) the database at path, applies any pending
// migrations, and returns a ready Store. Access is serialized to a single
// connection to avoid SQLite writer contention in a single-user app.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrating database: %w", err)
	}
	return &Store{db: db}, nil
}

// OpenDB wraps an already-open *sql.DB (used by the ETL tool, which manages its
// own connection) and runs migrations on it.
func OpenDB(db *sql.DB) (*Store, error) {
	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrating database: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the underlying database.
func (s *Store) Close() error { return s.db.Close() }

// DB exposes the raw handle for callers that need it (e.g. the ETL tool).
func (s *Store) DB() *sql.DB { return s.db }

// Checkpoint truncates the WAL into the main database file, producing a single
// self-contained file (used by the ETL before gzipping the seed).
func (s *Store) Checkpoint() error {
	_, err := s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	return err
}

// --- small helpers shared by the repository files ---

// nullableID converts a *int64 to a SQL-friendly value.
func nullableID(id *int64) any {
	if id == nil {
		return nil
	}
	return *id
}

// scanID reads a nullable INTEGER id column into a *int64.
func scanID(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}
