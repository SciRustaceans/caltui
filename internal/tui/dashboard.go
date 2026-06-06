package tui

import (
	"fmt"
	"math"
	"strings"

	"charm.land/lipgloss/v2"

	"caltui/internal/charts"
)

const (
	barCal   = 20
	barMacro = 14
)

// viewDashboard renders the home dashboard: a calorie gauge, macro bars, a week
// sparkline, and recent foods.
func (m *Model) viewDashboard(width int) string {
	cal := panel("Calories", m.caloriePanel(), 0)
	mac := panel("Macros", m.macroPanel(), 0)
	week := panel("This week", m.weekPanel(), 0)
	recent := panel("Recent", m.recentPanel(), 0)

	// Two columns when there's room, otherwise stack.
	if width >= 80 {
		top := lipgloss.JoinHorizontal(lipgloss.Top, cal, " ", mac)
		bottom := lipgloss.JoinHorizontal(lipgloss.Top, week, " ", recent)
		return lipgloss.JoinVertical(lipgloss.Left, top, bottom)
	}
	return lipgloss.JoinVertical(lipgloss.Left, cal, mac, week, recent)
}

func (m *Model) caloriePanel() string {
	if !m.hasGoal || m.goal.Target.Kcal <= 0 {
		return styleDim.Render("No goal set yet.\nPress 3 to set up goals.")
	}
	consumed := m.totals.Kcal
	target := m.goal.Target.Kcal
	ratio := consumed / target

	fill := colCalorie
	if consumed > target {
		fill = colWarn
	}
	big := styleText.Bold(true).Render(fmt.Sprintf("%s / %s kcal", fmtInt(consumed), fmtInt(target)))
	bar := renderBar(barCal, ratio, fill) + fmt.Sprintf(" %d%%", int(math.Round(ratio*100)))

	rem := target - consumed
	var note string
	if rem >= 0 {
		note = styleGood.Render(fmtInt(rem) + " kcal left")
	} else {
		note = styleWarn.Render(fmtInt(-rem) + " kcal over")
	}
	return strings.Join([]string{big, bar, note}, "\n")
}

func (m *Model) macroPanel() string {
	if !m.hasGoal {
		t := m.totals
		return strings.Join([]string{
			macroRow("Protein", t.Protein, 0, barMacro, colProtein),
			macroRow("Carbs", t.Carbs, 0, barMacro, colCarbs),
			macroRow("Fat", t.Fat, 0, barMacro, colFat),
		}, "\n")
	}
	t, g := m.totals, m.goal.Target
	return strings.Join([]string{
		macroRow("Protein", t.Protein, g.Protein, barMacro, colProtein),
		macroRow("Carbs", t.Carbs, g.Carbs, barMacro, colCarbs),
		macroRow("Fat", t.Fat, g.Fat, barMacro, colFat),
	}, "\n")
}

func (m *Model) weekPanel() string {
	if len(m.weekKcal) == 0 {
		return styleDim.Render("No history yet.")
	}
	spark := lipgloss.NewStyle().Foreground(colCalorie).Render(charts.Sparkline(m.weekKcal))
	var sum, n float64
	for _, v := range m.weekKcal {
		if v > 0 {
			sum += v
			n++
		}
	}
	avg := "—"
	if n > 0 {
		avg = fmtInt(sum/n) + " kcal"
	}
	return spark + "\n" + styleDim.Render("avg "+avg)
}

func (m *Model) recentPanel() string {
	if len(m.recent) == 0 {
		return styleDim.Render("Nothing logged yet.\nPress 2 then a to add food.")
	}
	limit := len(m.recent)
	if limit > 6 {
		limit = 6
	}
	var lines []string
	for _, f := range m.recent[:limit] {
		lines = append(lines, styleText.Render(truncate(f.Name, 34)))
	}
	return strings.Join(lines, "\n")
}

// truncate shortens s to limit runes, appending an ellipsis when cut.
func truncate(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	if limit <= 1 {
		return string(r[:limit])
	}
	return string(r[:limit-1]) + "…"
}
