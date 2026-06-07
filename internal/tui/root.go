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
	"caltui/internal/food"
	"caltui/internal/food/fdc"
	"caltui/internal/nutrition"
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
	store  *store.Store
	online food.Provider
	keys   keys.KeyMap
	help   help.Model

	width, height int
	active        tab
	fullHelp      bool
	today         string

	// Cached data for the current day.
	goal      domain.Goal
	hasGoal   bool
	entries   []domain.LogEntry
	totals    domain.Macros
	recent    []domain.Food
	weekKcal  []float64
	trendKcal []float64

	weights       []domain.Weight
	weightGoal    domain.WeightGoal
	hasWeightGoal bool
	intakeByDay   map[string]float64 // recent daily calories, for the maintenance estimate
	savedMeals    []domain.SavedMeal

	diaryCursor  int
	modal        modalModel
	firstRun     bool // no goal existed at startup (drives onboarding)
	promptedGoal bool // first-run wizard has been offered (don't re-pop)
	promptedKey  bool // first-run API-key prompt has been offered
	err          error
}

// New builds a root model backed by store s and an optional online provider
// (nil for offline-only), showing today.
func New(s *store.Store, online food.Provider) Model {
	m := NewForDate(s, time.Now().Format("2006-01-02"))
	m.online = online
	return m
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
	goal      domain.Goal
	hasGoal   bool
	entries   []domain.LogEntry
	totals    domain.Macros
	recent    []domain.Food
	weekKcal  []float64
	trendKcal []float64

	weights       []domain.Weight
	weightGoal    domain.WeightGoal
	hasWeightGoal bool
	intakeByDay   map[string]float64
	savedMeals    []domain.SavedMeal

	err error
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
		if msg.weekKcal, err = kcalWindow(s, date, 7); err != nil {
			msg.err = err
			return msg
		}
		if msg.trendKcal, err = kcalWindow(s, date, trendDays); err != nil {
			msg.err = err
			return msg
		}
		if msg.weights, err = weightWindow(s, date, 90); err != nil {
			msg.err = err
			return msg
		}
		if msg.weightGoal, msg.hasWeightGoal, err = s.GetWeightGoal(); err != nil {
			msg.err = err
			return msg
		}
		if t, e := time.Parse("2006-01-02", date); e == nil {
			from := t.AddDate(0, 0, -(nutrition.MaintenanceWindowDays - 1)).Format("2006-01-02")
			if msg.intakeByDay, err = s.CalorieSeries(from, date); err != nil {
				msg.err = err
				return msg
			}
		}
		if msg.savedMeals, err = s.ListSavedMeals(); err != nil {
			msg.err = err
			return msg
		}
		return msg
	}
}

// weightWindow loads weigh-ins for the last n days ending at date.
func weightWindow(s *store.Store, date string, n int) ([]domain.Weight, error) {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, err
	}
	from := t.AddDate(0, 0, -(n - 1)).Format("2006-01-02")
	return s.WeightSeries(from, date)
}

// trendDays is the window for the trends screen's calorie chart/table.
const trendDays = 30

// kcalWindow returns total calories per day for the last n days ending at today,
// oldest first, with unlogged days as 0.
func kcalWindow(s *store.Store, today string, n int) ([]float64, error) {
	t, err := time.Parse("2006-01-02", today)
	if err != nil {
		return nil, err
	}
	from := t.AddDate(0, 0, -(n - 1)).Format("2006-01-02")
	series, err := s.CalorieSeries(from, today)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		d := t.AddDate(0, 0, -(n-1)+i).Format("2006-01-02")
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
		if m.modal != nil {
			var cmd tea.Cmd
			m.modal, cmd = m.modal.Update(msg)
			return m, cmd
		}
		return m, nil
	case dayLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.goal, m.hasGoal = msg.goal, msg.hasGoal
		m.entries, m.totals = msg.entries, msg.totals
		m.recent, m.weekKcal = msg.recent, msg.weekKcal
		m.trendKcal = msg.trendKcal
		m.weights = msg.weights
		m.weightGoal, m.hasWeightGoal = msg.weightGoal, msg.hasWeightGoal
		m.intakeByDay = msg.intakeByDay
		m.savedMeals = msg.savedMeals
		m.clampDiaryCursor()
		// Onboarding, step 1: no goal yet -> TDEE wizard (once).
		if !m.hasGoal && m.modal == nil && !m.promptedGoal {
			m.promptedGoal = true
			m.firstRun = true
			wm := newWizardModal(m.today, time.Now(), nil)
			m.modal = wm
			return m, wm.focusActive()
		}
		// Onboarding, step 2 (first run only): offer the API-key setup once.
		if m.firstRun && m.online == nil && m.modal == nil && !m.promptedKey {
			m.promptedKey = true
			km := newAPIKeyModal()
			m.modal = km
			return m, km.focus()
		}
		return m, nil
	case closeModalMsg:
		m.modal = nil
		return m, nil
	case saveEntryMsg:
		return m, m.saveEntryCmd(msg.entry)
	case saveGoalMsg:
		return m, m.saveGoalCmd(msg.goal)
	case saveSetupMsg:
		return m, m.saveSetupCmd(msg.goal, msg.weight)
	case saveWeightMsg:
		return m, m.saveWeightCmd(msg.weight)
	case saveWeightGoalMsg:
		return m, m.saveWeightGoalCmd(msg.goal)
	case onlineEnabledMsg:
		m.online = fdc.New(msg.key)
		return m, nil
	case saveMealMsg:
		return m, m.saveMealCmd(msg.meal)
	case logSavedMealMsg:
		return m, m.logSavedMealCmd(msg.id, msg.meal)
	case deleteSavedMealMsg:
		return m, m.deleteSavedMealCmd(msg.id)
	case mutationDoneMsg:
		m.modal = nil
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		return m, m.loadDay()
	case tea.KeyPressMsg:
		if m.modal != nil {
			var cmd tea.Cmd
			m.modal, cmd = m.modal.Update(msg)
			return m, cmd
		}
		return m.handleKey(msg)
	default:
		if m.modal != nil {
			var cmd tea.Cmd
			m.modal, cmd = m.modal.Update(msg)
			return m, cmd
		}
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
	case key.Matches(msg, m.keys.Add):
		switch m.active {
		case tabGoals:
			return m.openWizard()
		case tabWeight:
			return m.openWeightEntry()
		default:
			return m.openFoodSearch()
		}
	case key.Matches(msg, m.keys.Search):
		if m.active == tabDashboard || m.active == tabDiary {
			return m.openFoodSearch()
		}
		return m, nil
	case key.Matches(msg, m.keys.Edit):
		switch m.active {
		case tabDiary:
			if e, ok := m.selectedEntry(); ok {
				sm := newEditModal(m.today, e)
				m.modal = sm
				return m, sm.focus()
			}
		case tabGoals:
			if m.hasGoal {
				return m.openManualGoal()
			}
			return m.openWizard()
		case tabWeight:
			return m.openWeightGoal()
		}
		return m, nil
	case key.Matches(msg, m.keys.Delete):
		if m.active == tabDiary {
			if e, ok := m.selectedEntry(); ok {
				return m, m.deleteEntryCmd(e.ID)
			}
		}
		return m, nil
	case key.Matches(msg, m.keys.Copy):
		if m.active == tabDiary {
			return m, m.copyYesterdayCmd()
		}
		return m, nil
	case key.Matches(msg, m.keys.Apply):
		if m.active == tabGoals {
			return m.applyMaintenance()
		}
		return m, nil
	case key.Matches(msg, m.keys.APIKey):
		if m.active == tabGoals {
			km := newAPIKeyModal()
			m.modal = km
			return m, km.focus()
		}
		return m, nil
	case key.Matches(msg, m.keys.Save):
		if m.active == tabDiary {
			return m.openSaveMeal()
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
	var body string
	if m.modal != nil {
		body = lipgloss.Place(m.width, bodyHeight, lipgloss.Center, lipgloss.Center, m.modal.View(m.width, bodyHeight))
	} else {
		body = lipgloss.NewStyle().Width(m.width).Height(bodyHeight).Render(m.renderBody(bodyHeight))
	}
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
	if m.modal != nil {
		return rule + "\n" + styleDim.Render("esc cancel")
	}
	if m.fullHelp {
		return rule + "\n" + m.help.FullHelpView(m.keys.FullHelp())
	}
	return rule + "\n" + m.help.ShortHelpView(m.keys.ShortHelp())
}

func (m Model) renderBody(height int) string {
	switch m.active {
	case tabDashboard:
		return m.viewDashboard(m.width)
	case tabDiary:
		return m.viewDiary(m.width, height)
	case tabGoals:
		return m.viewGoals(m.width)
	case tabWeight:
		return m.viewWeight(m.width)
	case tabTrends:
		return m.viewTrends(m.width)
	}
	return ""
}

// openFoodSearch opens the food search/add modal with the meal defaulted by the
// time of day.
func (m Model) openFoodSearch() (tea.Model, tea.Cmd) {
	meal := domain.MealForHour(time.Now().Hour())
	sm := newSearchModal(m.store, m.today, meal, m.recent)
	sm.online = m.online
	sm.savedMeals = m.savedMeals
	sm.viewHeight = m.height
	m.modal = sm
	return m, sm.focus()
}

func (m Model) prettyDate() string {
	t, err := time.Parse("2006-01-02", m.today)
	if err != nil {
		return m.today
	}
	return t.Format("Mon Jan 2")
}
