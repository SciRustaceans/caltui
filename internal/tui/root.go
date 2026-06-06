// Package tui implements the Bubble Tea v2 terminal UI: a tabbed shell
// (Dashboard / Diary / Goals / Weight / Trends) over the store and food
// providers, with an arrows+vim keymap and a help bar.
package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"caltui/internal/domain"
	"caltui/internal/store"
	"caltui/internal/tui/keys"
)

type tab int

const (
	tabDashboard tab = iota
	tabDiary
	tabGoals
	tabWeight
	tabTrends
	tabCount
)

var tabNames = []string{"Dashboard", "Diary", "Goals", "Weight", "Trends"}

// Model is the root Bubble Tea model.
type Model struct {
	store *store.Store
	keys  keys.KeyMap
	help  help.Model

	width, height int
	active        tab
	fullHelp      bool
	today         string

	// Cached data for the current day.
	goal     domain.Goal
	hasGoal  bool
	entries  []domain.LogEntry
	totals   domain.Macros
	recent   []domain.Food
	weekKcal []float64

	diaryCursor int
	err         error
}

// New builds a root model backed by store s, showing today.
func New(s *store.Store) Model {
	return NewForDate(s, time.Now().Format("2006-01-02"))
}

// NewForDate builds a root model for a specific date (used by tests).
func NewForDate(s *store.Store, date string) Model {
	return Model{
		store:  s,
		keys:   keys.Default(),
		help:   help.New(),
		active: tabDashboard,
		today:  date,
	}
}

// Init loads the current day's data.
func (m Model) Init() tea.Cmd { return m.loadDay() }

// dayLoadedMsg carries the day's data loaded off the Update loop.
type dayLoadedMsg struct {
	goal     domain.Goal
	hasGoal  bool
	entries  []domain.LogEntry
	totals   domain.Macros
	recent   []domain.Food
	weekKcal []float64
	err      error
}

// loadDay returns a command that reads everything the views need for m.today.
func (m Model) loadDay() tea.Cmd {
	s, date := m.store, m.today
	return func() tea.Msg {
		var msg dayLoadedMsg
		g, ok, err := s.CurrentGoal(date)
		if err != nil {
			msg.err = err
			return msg
		}
		msg.goal, msg.hasGoal = g, ok
		if msg.entries, err = s.EntriesForDate(date); err != nil {
			msg.err = err
			return msg
		}
		if msg.totals, err = s.DayTotals(date); err != nil {
			msg.err = err
			return msg
		}
		if msg.recent, err = s.RecentFoods(8); err != nil {
			msg.err = err
			return msg
		}
		if msg.weekKcal, err = weekSeries(s, date); err != nil {
			msg.err = err
			return msg
		}
		return msg
	}
}

func weekSeries(s *store.Store, today string) ([]float64, error) {
	t, err := time.Parse("2006-01-02", today)
	if err != nil {
		return nil, err
	}
	from := t.AddDate(0, 0, -6).Format("2006-01-02")
	series, err := s.CalorieSeries(from, today)
	if err != nil {
		return nil, err
	}
	out := make([]float64, 7)
	for i := 0; i < 7; i++ {
		d := t.AddDate(0, 0, -6+i).Format("2006-01-02")
		out[i] = series[d]
	}
	return out, nil
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.SetWidth(msg.Width)
		return m, nil
	case dayLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.goal, m.hasGoal = msg.goal, msg.hasGoal
		m.entries, m.totals = msg.entries, msg.totals
		m.recent, m.weekKcal = msg.recent, msg.weekKcal
		m.clampDiaryCursor()
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Help):
		m.fullHelp = !m.fullHelp
		return m, nil
	case key.Matches(msg, m.keys.Tabs):
		if n, err := strconv.Atoi(msg.String()); err == nil && n >= 1 && n <= int(tabCount) {
			m.active = tab(n - 1)
		}
		return m, nil
	case key.Matches(msg, m.keys.Tab), key.Matches(msg, m.keys.Right):
		m.active = (m.active + 1) % tabCount
		return m, nil
	case key.Matches(msg, m.keys.PrevTab), key.Matches(msg, m.keys.Left):
		m.active = (m.active - 1 + tabCount) % tabCount
		return m, nil
	case key.Matches(msg, m.keys.Up):
		if m.active == tabDiary {
			m.diaryCursor--
			m.clampDiaryCursor()
		}
		return m, nil
	case key.Matches(msg, m.keys.Down):
		if m.active == tabDiary {
			m.diaryCursor++
			m.clampDiaryCursor()
		}
		return m, nil
	}
	return m, nil
}

// View renders the whole screen.
func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

func (m Model) render() string {
	if m.width < 50 || m.height < 12 {
		return "caltui needs a larger terminal — please resize."
	}
	header := m.renderTabs()
	footer := m.renderFooter()
	bodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer)
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	body := lipgloss.NewStyle().Width(m.width).Height(bodyHeight).Render(m.renderBody())
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m Model) renderTabs() string {
	parts := make([]string, 0, len(tabNames))
	for i, name := range tabNames {
		label := fmt.Sprintf(" %d %s ", i+1, name)
		if tab(i) == m.active {
			parts = append(parts, lipgloss.NewStyle().
				Foreground(lipgloss.Color("231")).Background(colAccent).Bold(true).Render(label))
		} else {
			parts = append(parts, styleDim.Render(label))
		}
	}
	bar := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	return bar + "\n" + styleFaint.Render(strings.Repeat("─", m.width))
}

func (m Model) renderFooter() string {
	rule := styleFaint.Render(strings.Repeat("─", m.width))
	if m.err != nil {
		return rule + "\n" + styleWarn.Render("error: "+m.err.Error())
	}
	if m.fullHelp {
		return rule + "\n" + m.help.FullHelpView(m.keys.FullHelp())
	}
	return rule + "\n" + m.help.ShortHelpView(m.keys.ShortHelp())
}

func (m Model) renderBody() string {
	switch m.active {
	case tabDashboard:
		return m.viewDashboard(m.width)
	case tabDiary:
		return m.viewDiary(m.width)
	case tabGoals:
		return m.placeholder("Goals", "Daily calorie & macro targets and the TDEE setup wizard live here.")
	case tabWeight:
		return m.placeholder("Weight", "Log body weight and track progress toward a goal here.")
	case tabTrends:
		return m.placeholder("Trends", "Calorie and weight charts over time live here.")
	}
	return ""
}

func (m Model) placeholder(title, desc string) string {
	return styleTitle.Render(title) + "\n\n" + styleDim.Render(desc)
}

func (m Model) prettyDate() string {
	t, err := time.Parse("2006-01-02", m.today)
	if err != nil {
		return m.today
	}
	return t.Format("Mon Jan 2")
}
