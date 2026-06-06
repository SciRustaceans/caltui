// Package config resolves caltui's data directory and loads/saves the optional
// user config (TOML). The data directory is XDG_CONFIG_HOME/tuitracker when set,
// otherwise ~/.config/tuitracker on every platform (including macOS) so the
// location is predictable for a terminal app, rather than ~/Library/Application
// Support as os.UserConfigDir would give.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// AppDir is the per-user subdirectory name.
const AppDir = "tuitracker"

// envAPIKey is the environment variable that overrides the config-file API key.
const envAPIKey = "FDC_API_KEY"

// Config is the persisted user configuration.
type Config struct {
	// FDCAPIKey enables online USDA FoodData Central lookups. Empty means
	// offline-only. An FDC_API_KEY environment variable overrides this.
	FDCAPIKey string `toml:"fdc_api_key"`
	// WeightUnit is the preferred display unit for body weight ("kg" or "lb").
	WeightUnit string `toml:"weight_unit"`
}

func defaults() Config { return Config{WeightUnit: "kg"} }

// Dir returns the data directory, creating it (0700) if necessary.
func Dir() (string, error) {
	var base string
	if x := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); x != "" {
		base = x
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	dir := filepath.Join(base, AppDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// Path returns the config file path (ensuring the directory exists).
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

// DBPath returns the SQLite database path (ensuring the directory exists).
func DBPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tuitracker.db"), nil
}

// Load reads the config file (tolerating a missing file by returning defaults)
// and applies the FDC_API_KEY environment override.
func Load() (Config, error) {
	c := defaults()
	p, err := Path()
	if err != nil {
		return c, err
	}
	data, err := os.ReadFile(p)
	switch {
	case err == nil:
		if err := toml.Unmarshal(data, &c); err != nil {
			return c, fmt.Errorf("parsing %s: %w", p, err)
		}
	case os.IsNotExist(err):
		// keep defaults
	default:
		return c, err
	}
	if k := strings.TrimSpace(os.Getenv(envAPIKey)); k != "" {
		c.FDCAPIKey = k
	}
	if c.WeightUnit == "" {
		c.WeightUnit = "kg"
	}
	return c, nil
}

// Save writes the config file with 0600 permissions.
func Save(c Config) error {
	p, err := Path()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := toml.NewEncoder(f).Encode(c); err != nil {
		return err
	}
	return f.Close()
}

// HasAPIKey reports whether an online API key is configured.
func (c Config) HasAPIKey() bool { return strings.TrimSpace(c.FDCAPIKey) != "" }

// LoadDotEnv reads simple KEY=VALUE lines from a .env file in the current
// directory (a developer convenience when running from the project) and sets
// them in the process environment, without overriding variables already set.
// A missing .env is not an error.
func LoadDotEnv() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if key != "" && os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}
