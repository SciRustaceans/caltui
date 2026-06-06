package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"caltui/internal/domain"
)

// modalModel is an overlay (search, add/edit, wizard) that captures input while
// open. Its Update returns the (possibly updated) modal and a command; returning
// a closeModalMsg command tells the root to dismiss it.
type modalModel interface {
	Update(tea.Msg) (modalModel, tea.Cmd)
	View(width, height int) string
}

// Messages exchanged between modals and the root.
type closeModalMsg struct{}

// saveEntryMsg asks the root to persist an entry: ID == 0 inserts, else updates.
type saveEntryMsg struct{ entry domain.LogEntry }

// saveGoalMsg asks the root to persist a goal.
type saveGoalMsg struct{ goal domain.Goal }

// mutationDoneMsg signals a store mutation finished; the root closes any modal,
// records an error, and reloads the day.
type mutationDoneMsg struct{ err error }

func closeModalCmd() tea.Msg { return closeModalMsg{} }

// saveEntryCmd inserts or updates a diary entry.
func (m Model) saveEntryCmd(e domain.LogEntry) tea.Cmd {
	s := m.store
	return func() tea.Msg {
		var err error
		if e.ID == 0 {
			_, err = s.AddEntry(e)
		} else {
			err = s.UpdateEntry(e)
		}
		return mutationDoneMsg{err: err}
	}
}

// saveGoalCmd persists a goal (a new dated row).
func (m Model) saveGoalCmd(g domain.Goal) tea.Cmd {
	s := m.store
	return func() tea.Msg {
		_, err := s.AddGoal(g)
		return mutationDoneMsg{err: err}
	}
}

// deleteEntryCmd removes a diary entry.
func (m Model) deleteEntryCmd(id int64) tea.Cmd {
	s := m.store
	return func() tea.Msg { return mutationDoneMsg{err: s.DeleteEntry(id)} }
}

// copyYesterdayCmd copies yesterday's entries into today.
func (m Model) copyYesterdayCmd() tea.Cmd {
	s, today := m.store, m.today
	return func() tea.Msg {
		y, err := prevDate(today)
		if err != nil {
			return mutationDoneMsg{err: err}
		}
		_, err = s.CopyDay(y, today)
		return mutationDoneMsg{err: err}
	}
}

func prevDate(date string) (string, error) {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", err
	}
	return t.AddDate(0, 0, -1).Format("2006-01-02"), nil
}
