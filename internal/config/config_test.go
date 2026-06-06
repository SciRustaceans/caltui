package config

import (
	"os"
	"path/filepath"
	"testing"
)

// useTempConfig points the config dir at a temp directory for the duration of
// the test via XDG_CONFIG_HOME.
func useTempConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("FDC_API_KEY", "") // ensure a clean env override baseline
	return dir
}

func TestPaths(t *testing.T) {
	dir := useTempConfig(t)
	p, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, AppDir, "config.toml")
	if p != want {
		t.Errorf("Path = %q, want %q", p, want)
	}
	dbp, _ := DBPath()
	if dbp != filepath.Join(dir, AppDir, "tuitracker.db") {
		t.Errorf("DBPath = %q", dbp)
	}
	// Dir is created.
	if _, err := os.Stat(filepath.Join(dir, AppDir)); err != nil {
		t.Errorf("data dir not created: %v", err)
	}
}

func TestLoadDefaultsWhenMissing(t *testing.T) {
	useTempConfig(t)
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.WeightUnit != "kg" {
		t.Errorf("default WeightUnit = %q, want kg", c.WeightUnit)
	}
	if c.HasAPIKey() {
		t.Error("no API key expected by default")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	useTempConfig(t)
	in := Config{FDCAPIKey: "abc123", WeightUnit: "lb"}
	if err := Save(in); err != nil {
		t.Fatal(err)
	}
	// File should be 0600.
	p, _ := Path()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config perms = %o, want 600", perm)
	}
	out, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if out.FDCAPIKey != "abc123" || out.WeightUnit != "lb" {
		t.Errorf("round trip = %+v", out)
	}
}

func TestEnvOverridesAPIKey(t *testing.T) {
	useTempConfig(t)
	if err := Save(Config{FDCAPIKey: "file-key", WeightUnit: "kg"}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FDC_API_KEY", "env-key")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.FDCAPIKey != "env-key" {
		t.Errorf("env did not override: got %q", c.FDCAPIKey)
	}
}
