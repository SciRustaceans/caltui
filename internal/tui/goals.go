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
	if est, ok := nutrition.EstimateMaintenance(m.intakeByDay, m.weights, m.today, nutrition.MaintenanceWindowDays); ok {
		line := fmt.Sprintf("Est. maintenance (last %dd): ~%s kcal", nutrition.MaintenanceWindowDays, fmtInt(est))
		if !g.Manual && g.WeightKg > 0 && g.Activity.Valid() {
			age, _ := nutrition.AgeFromDate(g.BirthDate, time.Now())
			tdee := nutrition.TDEE(nutrition.BMR(g.Sex, g.WeightKg, g.HeightCm, age), g.Activity)
			line += fmt.Sprintf(" · formula: %s", fmtInt(tdee))
		}
		b.WriteString("\n" + styleText.Render(line) + "\n")
		b.WriteString(styleFaint.Render("m: set goal from this estimate") + "\n")
	}

	b.WriteString("\n")
	if m.online != nil {
		b.WriteString(styleGood.Render("● Online food lookup: on") + "\n")
	} else {
		b.WriteString(styleDim.Render("○ Online food lookup: off — press k to connect") + "\n")
	}
	b.WriteString("\n" + styleFaint.Render("a re-run wizard · e edit manually · k API key"))
	return b.String()
}

// applyMaintenance rebuilds the goal using the measured maintenance estimate as
// the TDEE, keeping the current goal's weekly rate.
func (m Model) applyMaintenance() (tea.Model, tea.Cmd) {
	est, ok := nutrition.EstimateMaintenance(m.intakeByDay, m.weights, m.today, nutrition.MaintenanceWindowDays)
	if !ok || !m.hasGoal {
		return m, nil
	}
	cur := m.goal
	target := nutrition.CalorieTarget(est, cur.GoalRate)
	weight := cur.WeightKg
	if weight <= 0 && len(m.weights) > 0 {
		weight = m.weights[len(m.weights)-1].Kg
	}
	var macros domain.Macros
	if weight > 0 {
		macros = nutrition.DefaultMacroSplit(target.Kcal, weight).Macros
	} else {
		macros = nutrition.MacroSplitPercent(target.Kcal, nutrition.DefaultProteinPct, nutrition.DefaultCarbsPct, nutrition.DefaultFatPct)
	}
	goal := domain.Goal{
		EffectiveDate: m.today,
		Target:        macros.Round(),
		Sex:           cur.Sex,
		BirthDate:     cur.BirthDate,
		HeightCm:      cur.HeightCm,
		WeightKg:      weight,
		Activity:      cur.Activity,
		GoalRate:      cur.GoalRate,
	}
	return m, m.saveGoalCmd(goal)
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
