package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"caltui/internal/domain"
	"caltui/internal/nutrition"
)

// viewGoals renders the current goal: target calories, today's progress, the
// macro split, and (for wizard goals) the inputs it was derived from.
func (m *Model) viewGoals(_ int) string {
	if !m.hasGoal {
		return styleTitle.Render("Goals") + "\n\n" +
			styleDim.Render("No goal set yet.\nPress a to run the setup wizard, or e to enter targets manually.")
	}
	g := m.goal
	var b strings.Builder
	source := "from TDEE wizard"
	if g.Manual {
		source = "manual"
	}
	b.WriteString(styleTitle.Render("Goals") + "  " + styleDim.Render("("+source+")") + "\n\n")

	consumed := m.totals.Kcal
	rem := g.Target.Kcal - consumed
	remStr := styleGood.Render(fmtInt(rem) + " left")
	if rem < 0 {
		remStr = styleWarn.Render(fmtInt(-rem) + " over")
	}
	b.WriteString(styleText.Bold(true).Render(fmt.Sprintf("Daily target: %s kcal", fmtInt(g.Target.Kcal))) + "\n")
	ratio := 0.0
	if g.Target.Kcal > 0 {
		ratio = consumed / g.Target.Kcal
	}
	fill := colCalorie
	if consumed > g.Target.Kcal {
		fill = colWarn
	}
	b.WriteString("Today  " + renderBar(barCal, ratio, fill) +
		fmt.Sprintf("  %s / %s  ", fmtInt(consumed), fmtInt(g.Target.Kcal)) + remStr + "\n\n")

	b.WriteString(macroRow("Protein", m.totals.Protein, g.Target.Protein, barMacro, colProtein) + "\n")
	b.WriteString(macroRow("Carbs", m.totals.Carbs, g.Target.Carbs, barMacro, colCarbs) + "\n")
	b.WriteString(macroRow("Fat", m.totals.Fat, g.Target.Fat, barMacro, colFat) + "\n")

	if !g.Manual && g.WeightKg > 0 {
		age := 0
		if a, err := nutrition.AgeFromDate(g.BirthDate, time.Now()); err == nil {
			age = a
		}
		rate := "maintain"
		for _, o := range goalOptions {
			if o.rate == g.GoalRate {
				rate = o.label
			}
		}
		b.WriteString("\n" + styleDim.Render(fmt.Sprintf("Based on: %s · %dy · %scm · %skg · %s · %s",
			g.Sex, age, trimNum(g.HeightCm), trimNum(g.WeightKg), g.Activity.Label(), rate)) + "\n")
	}
	b.WriteString("\n" + styleFaint.Render("a re-run wizard · e edit manually"))
	return b.String()
}

// openWizard opens the TDEE setup wizard, prefilled from the current goal.
func (m Model) openWizard() (tea.Model, tea.Cmd) {
	var pre *domain.Goal
	if m.hasGoal {
		pre = &m.goal
	}
	wm := newWizardModal(m.today, time.Now(), pre)
	m.modal = wm
	return m, wm.focusActive()
}

// openManualGoal opens the manual goal editor.
func (m Model) openManualGoal() (tea.Model, tea.Cmd) {
	mm := newManualGoalModal(m.today, m.goal)
	m.modal = mm
	return m, mm.focus()
}
