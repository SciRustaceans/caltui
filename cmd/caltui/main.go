// Command caltui is a terminal calorie & macro tracker.
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
)

// model is a placeholder root model; the real TUI is built in internal/tui.
type model struct{}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() tea.View {
	v := tea.NewView("caltui — coming soon.\n\nPress q to quit.")
	v.AltScreen = true
	return v
}

func main() {
	if _, err := tea.NewProgram(model{}).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "caltui: %v\n", err)
		os.Exit(1)
	}
}
