package store

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"

	"caltui/internal/seed"
)

// SeedIfMissing writes the bundled food database to dbPath when no database
// exists there yet and a seed is embedded, returning true if it seeded. The
// copied file already contains the foods table + FTS index; Open's migrator then
// brings it up to the latest schema (a no-op when the seed is current). When no
// seed is bundled it is a no-op and Open will create an empty schema instead.
func SeedIfMissing(dbPath string) (bool, error) {
	if _, err := os.Stat(dbPath); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}

	gz := seed.Gzip()
	if len(gz) == 0 {
		return false, nil
	}

	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return false, fmt.Errorf("reading embedded seed: %w", err)
	}
	defer zr.Close()

	// Write to a temp file then rename so a partial write never looks like a
	// valid database.
	tmp := dbPath + ".seed.tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return false, err
	}
	if _, err := io.Copy(f, zr); err != nil { //nolint:gosec // bounded embedded asset
		_ = f.Close()
		_ = os.Remove(tmp)
		return false, err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return false, err
	}
	if err := os.Rename(tmp, dbPath); err != nil {
		_ = os.Remove(tmp)
		return false, err
	}
	return true, nil
}
