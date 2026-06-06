// Command caltui is a terminal calorie & macro tracker.
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"caltui/internal/config"
	"caltui/internal/store"
	"caltui/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "caltui: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	dbPath, err := config.DBPath()
	if err != nil {
		return fmt.Errorf("resolving data directory: %w", err)
	}
	if _, err := store.SeedIfMissing(dbPath); err != nil {
		return fmt.Errorf("seeding food database: %w", err)
	}
	st, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	if _, err := tea.NewProgram(tui.New(st)).Run(); err != nil {
		return err
	}
	return nil
}
