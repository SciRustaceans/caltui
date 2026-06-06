package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"caltui/internal/domain"
)

// press builds a KeyPressMsg whose String() matches our key bindings.
func press(s string) tea.KeyPressMsg {
	switch s {
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	default:
		return tea.KeyPressMsg{Text: s, Code: []rune(s)[0]}
	}
}

func TestPressStringMatchesBindings(t *testing.T) {
	cases := map[string]string{"2": "2", "j": "j", "q": "q", "tab": "tab"}
	for in, want := range cases {
		if got := press(in).String(); got != want {
			t.Errorf("press(%q).String() = %q, want %q", in, got, want)
		}
	}
}

func sampleModel() Model {
	m := NewForDate(nil, "2026-06-06")
	m.width, m.height = 100, 30
	m.hasGoal = true
	m.goal = domain.Goal{Target: domain.Macros{Kcal: 2100, Protein: 140, Carbs: 210, Fat: 70}}
	m.totals = domain.Macros{Kcal: 1420, Protein: 95, Carbs: 150, Fat: 48}
	m.entries = []domain.LogEntry{
		{Date: "2026-06-06", Meal: domain.Breakfast, Name: "Oatmeal",
			PerUnit: domain.Macros{Kcal: 3.89, Protein: 0.13, Carbs: 0.66, Fat: 0.07}, Quantity: 100, Unit: domain.UnitGram},
		{Date: "2026-06-06", Meal: domain.Lunch, Name: "Chicken breast",
			PerUnit: domain.Macros{Kcal: 1.65, Protein: 0.31}, Quantity: 200, Unit: domain.UnitGram},
	}
	m.recent = []domain.Food{{Name: "Oatmeal"}, {Name: "Banana"}}
	m.weekKcal = []float64{1800, 2000, 1950, 2100, 1700, 2200, 1420}
	return m
}

// update applies a message and returns the concrete Model.
func update(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	nm, cmd := m.Update(msg)
	mm, ok := nm.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want tui.Model", nm)
	}
	return mm, cmd
}

func TestRenderDashboard(t *testing.T) {
	out := sampleModel().render()
	for _, want := range []string{"Dashboard", "Diary", "Trends", "Calories", "1420", "2100", "Protein", "Oatmeal", "avg"} {
		if !strings.Contains(out, want) {
			t.Errorf("dashboard render missing %q", want)
		}
	}
}

func TestTabSwitching(t *testing.T) {
	m := sampleModel()

	m, _ = update(t, m, press("2"))
	if m.active != tabDiary {
		t.Fatalf("after '2', active = %d, want diary", m.active)
	}
	out := m.render()
	if !strings.Contains(out, "BREAKFAST") || !strings.Contains(out, "Chicken breast") {
		t.Errorf("diary render missing meal/entry; got:\n%s", out)
	}

	m, _ = update(t, m, press("5"))
	if m.active != tabTrends {
		t.Errorf("after '5', active = %d, want trends", m.active)
	}

	// tab wraps from last back to dashboard.
	m, _ = update(t, m, press("tab"))
	if m.active != tabDashboard {
		t.Errorf("after tab from trends, active = %d, want dashboard", m.active)
	}
}

func TestDiaryCursor(t *testing.T) {
	m := sampleModel()
	m, _ = update(t, m, press("2")) // diary
	if m.diaryCursor != 0 {
		t.Fatalf("initial cursor = %d", m.diaryCursor)
	}
	m, _ = update(t, m, press("j"))
	if m.diaryCursor != 1 {
		t.Errorf("after j cursor = %d, want 1", m.diaryCursor)
	}
	m, _ = update(t, m, press("j")) // clamps at last
	if m.diaryCursor != 1 {
		t.Errorf("cursor should clamp at 1, got %d", m.diaryCursor)
	}
	m, _ = update(t, m, press("k"))
	m, _ = update(t, m, press("k")) // clamps at 0
	if m.diaryCursor != 0 {
		t.Errorf("cursor should clamp at 0, got %d", m.diaryCursor)
	}
	if e, ok := m.selectedEntry(); !ok || e.Name != "Oatmeal" {
		t.Errorf("selectedEntry = %+v ok=%v", e, ok)
	}
}

func TestQuit(t *testing.T) {
	m := sampleModel()
	_, cmd := update(t, m, press("q"))
	if cmd == nil {
		t.Fatal("quit returned nil cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("quit cmd did not produce QuitMsg")
	}
}

func TestHelpToggle(t *testing.T) {
	m := sampleModel()
	if m.fullHelp {
		t.Fatal("help should start collapsed")
	}
	m, _ = update(t, m, press("?"))
	if !m.fullHelp {
		t.Error("'?' should expand help")
	}
}

func TestTooSmall(t *testing.T) {
	m := sampleModel()
	m.width, m.height = 20, 5
	if !strings.Contains(m.render(), "larger terminal") {
		t.Error("expected too-small message")
	}
}
