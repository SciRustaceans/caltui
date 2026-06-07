package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"caltui/internal/domain"
	"caltui/internal/nutrition"
)

// weightUnit is the display unit, taken from the latest weigh-in (or goal).
func (m *Model) weightUnit() string {
	if n := len(m.weights); n > 0 && m.weights[n-1].Unit != "" {
		return m.weights[n-1].Unit
	}
	if m.hasWeightGoal && m.weightGoal.Unit != "" {
		return m.weightGoal.Unit
	}
	return "kg"
}

// dispWeight converts a kilogram value to the display unit (linear, so it also
// works for deltas).
func dispWeight(kg float64, unit string) float64 {
	if unit == "lb" {
		return nutrition.KgToLb(kg)
	}
	return kg
}

func trimNum1(f float64) string { return fmt.Sprintf("%.1f", f) }

func (m *Model) viewWeight(width int) string {
	if len(m.weights) == 0 {
		return styleTitle.Render("Weight") + "\n\n" +
			styleDim.Render("No weigh-ins yet.\nPress a to log your weight, or e to set a goal.")
	}
	unit := m.weightUnit()
	latest := m.weights[len(m.weights)-1]
	var b strings.Builder
	b.WriteString(styleTitle.Render("Weight") + "\n\n")

	b.WriteString(styleText.Bold(true).Render(fmt.Sprintf("Current: %s %s", trimNum1(dispWeight(latest.Kg, unit)), unit)))
	b.WriteString("  " + styleDim.Render(latest.Date))
	if len(m.weights) >= 2 {
		d := dispWeight(latest.Kg-m.weights[len(m.weights)-2].Kg, unit)
		arrow, st := "→", styleDim
		switch {
		case d < 0:
			arrow, st = "↓", styleGood
		case d > 0:
			arrow, st = "↑", styleWarn
		}
		b.WriteString("  " + st.Render(fmt.Sprintf("%s %s", arrow, trimNum1(absf(d)))))
	}
	b.WriteString("\n\n")

	// Weight plot with the projected trend toward the goal (same chart as the
	// Trends tab). Falls back to a plain line or a hint when there isn't enough
	// data to infer a trend.
	chartW := width - 14
	if chartW > 80 {
		chartW = 80
	}
	if chartW < 20 {
		chartW = 20
	}
	// A taller plot renders a shallow trend as a smooth diagonal rather than a
	// coarse staircase. Use the vertical space available on this tab, leaving
	// room for the header, goal block, recent list and footer.
	chartH := 14
	if m.height > 0 {
		if h := m.height - 16; h < chartH {
			chartH = h
		}
	}
	if chartH < 6 {
		chartH = 6
	}
	b.WriteString(m.weightChart(chartW, chartH) + "\n")

	if m.hasWeightGoal {
		g := m.weightGoal
		toGo := absf(dispWeight(latest.Kg-g.TargetKg, unit))
		fmt.Fprintf(&b, "Goal: %s %s", trimNum1(dispWeight(g.TargetKg, unit)), unit)
		b.WriteString("  " + styleDim.Render(fmt.Sprintf("%s %s to go", trimNum1(toGo), unit)) + "\n")
		if g.StartKg > 0 && g.StartKg != g.TargetKg {
			ratio := (g.StartKg - latest.Kg) / (g.StartKg - g.TargetKg)
			b.WriteString(renderBar(barCal, ratio, colGood) + fmt.Sprintf(" %d%%\n", clampPct(ratio)))
		}
	} else {
		b.WriteString(styleDim.Render("No weight goal set. Press e to set one.") + "\n")
	}

	// Recent weigh-ins (most recent first).
	b.WriteString("\n" + styleDim.Render("Recent") + "\n")
	n := len(m.weights)
	for i := n - 1; i >= 0 && i > n-6; i-- {
		w := m.weights[i]
		fmt.Fprintf(&b, "  %s  %s %s\n", styleDim.Render(w.Date),
			styleText.Render(trimNum1(dispWeight(w.Kg, unit))), unit)
	}
	b.WriteString("\n" + styleFaint.Render("a log weight · e set goal"))
	return b.String()
}

func absf(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func clampPct(ratio float64) int {
	p := int(ratio*100 + 0.5)
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	return p
}

// openWeightEntry / openWeightGoal open the weight modals.
func (m Model) openWeightEntry() (tea.Model, tea.Cmd) {
	unit := m.weightUnit()
	var prev float64
	if n := len(m.weights); n > 0 {
		prev = dispWeight(m.weights[n-1].Kg, unit)
	}
	wm := newWeightEntryModal(m.today, unit, prev)
	m.modal = wm
	return m, wm.focus()
}

func (m Model) openWeightGoal() (tea.Model, tea.Cmd) {
	unit := m.weightUnit()
	var cur float64
	if n := len(m.weights); n > 0 {
		cur = m.weights[n-1].Kg
	}
	gm := newWeightGoalModal(m.today, unit, cur, m.weightGoal, m.hasWeightGoal)
	m.modal = gm
	return m, gm.focus()
}

// --- weigh-in entry modal ---

type weightEntryModal struct {
	date     string
	input    textinput.Model
	unit     string
	fieldIdx int // 0 weight, 1 unit
	msg      string
}

func newWeightEntryModal(date, unit string, prev float64) *weightEntryModal {
	ti := textinput.New()
	ti.SetWidth(8)
	ti.CharLimit = 6
	if prev > 0 {
		ti.SetValue(trimNum1(prev))
	}
	if unit == "" {
		unit = "kg"
	}
	return &weightEntryModal{date: date, input: ti, unit: unit}
}

func (wm *weightEntryModal) focus() tea.Cmd { return wm.input.Focus() }

func (wm *weightEntryModal) Update(msg tea.Msg) (modalModel, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return wm, nil
	}
	switch key.String() {
	case "esc":
		return wm, closeModalCmd
	case "tab", "down", "shift+tab", "up":
		wm.fieldIdx = (wm.fieldIdx + 1) % 2
		if wm.fieldIdx == 0 {
			return wm, wm.input.Focus()
		}
		wm.input.Blur()
		return wm, nil
	case "left", "right":
		if wm.fieldIdx == 1 {
			wm.unit = toggleUnit(wm.unit)
		}
		return wm, nil
	case "enter":
		return wm.submit()
	default:
		if wm.fieldIdx == 0 {
			var cmd tea.Cmd
			wm.input, cmd = wm.input.Update(msg)
			return wm, cmd
		}
		return wm, nil
	}
}

func (wm *weightEntryModal) submit() (modalModel, tea.Cmd) {
	v := parseF(wm.input.Value())
	if v <= 0 {
		wm.msg = "Enter a valid weight."
		return wm, nil
	}
	kg := v
	if wm.unit == "lb" {
		kg = nutrition.LbToKg(v)
	}
	w := domain.Weight{Date: wm.date, Kg: kg, Unit: wm.unit}
	return wm, func() tea.Msg { return saveWeightMsg{weight: w} }
}

func (wm *weightEntryModal) View(width, _ int) string {
	boxW := 40
	if width-6 < boxW {
		boxW = width - 6
	}
	var b strings.Builder
	b.WriteString(styleTitle.Render("Log weight") + "\n\n")
	b.WriteString(wizField("Weight", wm.input.View(), wm.fieldIdx == 0) + "\n")
	b.WriteString(wizSelector("Unit", wm.unit, wm.fieldIdx == 1) + "\n")
	if wm.msg != "" {
		b.WriteString("\n" + styleWarn.Render(wm.msg) + "\n")
	}
	b.WriteString("\n" + styleFaint.Render("enter save · tab switch · ←/→ unit · esc cancel"))
	return stylePanel.Width(boxW).Render(b.String())
}

func toggleUnit(u string) string {
	if u == "lb" {
		return "kg"
	}
	return "lb"
}

// --- weight goal modal ---

type weightGoalModal struct {
	date     string
	unit     string
	startKg  float64
	target   textinput.Model
	rateIdx  int
	fieldIdx int // 0 target, 1 unit, 2 rate
	msg      string
}

func newWeightGoalModal(date, unit string, curKg float64, cur domain.WeightGoal, has bool) *weightGoalModal {
	ti := textinput.New()
	ti.SetWidth(8)
	ti.CharLimit = 6
	rateIdx := defaultGoalIdx() // maintain
	if has {
		ti.SetValue(trimNum1(dispWeight(cur.TargetKg, unit)))
		for i, o := range goalOptions {
			if o.rate == cur.RatePerWeek {
				rateIdx = i
			}
		}
	}
	if unit == "" {
		unit = "kg"
	}
	return &weightGoalModal{date: date, unit: unit, startKg: curKg, target: ti, rateIdx: rateIdx}
}

func (gm *weightGoalModal) focus() tea.Cmd { return gm.target.Focus() }

func (gm *weightGoalModal) Update(msg tea.Msg) (modalModel, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return gm, nil
	}
	switch key.String() {
	case "esc":
		return gm, closeModalCmd
	case "tab", "down":
		gm.fieldIdx = (gm.fieldIdx + 1) % 3
		return gm, gm.focusField()
	case "shift+tab", "up":
		gm.fieldIdx = (gm.fieldIdx + 2) % 3
		return gm, gm.focusField()
	case "left":
		gm.cycle(-1)
		return gm, nil
	case "right":
		gm.cycle(1)
		return gm, nil
	case "enter":
		return gm.submit()
	default:
		if gm.fieldIdx == 0 {
			var cmd tea.Cmd
			gm.target, cmd = gm.target.Update(msg)
			return gm, cmd
		}
		return gm, nil
	}
}

func (gm *weightGoalModal) focusField() tea.Cmd {
	if gm.fieldIdx == 0 {
		return gm.target.Focus()
	}
	gm.target.Blur()
	return nil
}

func (gm *weightGoalModal) cycle(d int) {
	switch gm.fieldIdx {
	case 1:
		gm.unit = toggleUnit(gm.unit)
	case 2:
		gm.rateIdx = (gm.rateIdx + d + len(goalOptions)) % len(goalOptions)
	}
}

func (gm *weightGoalModal) submit() (modalModel, tea.Cmd) {
	v := parseF(gm.target.Value())
	if v <= 0 {
		gm.msg = "Enter a valid target weight."
		return gm, nil
	}
	targetKg := v
	if gm.unit == "lb" {
		targetKg = nutrition.LbToKg(v)
	}
	g := domain.WeightGoal{
		TargetKg:    targetKg,
		Unit:        gm.unit,
		RatePerWeek: goalOptions[gm.rateIdx].rate,
		StartDate:   gm.date,
		StartKg:     gm.startKg,
	}
	return gm, func() tea.Msg { return saveWeightGoalMsg{goal: g} }
}

func (gm *weightGoalModal) View(width, _ int) string {
	boxW := 48
	if width-6 < boxW {
		boxW = width - 6
	}
	var b strings.Builder
	b.WriteString(styleTitle.Render("Weight goal") + "\n\n")
	b.WriteString(wizField("Target", gm.target.View(), gm.fieldIdx == 0) + "\n")
	b.WriteString(wizSelector("Unit", gm.unit, gm.fieldIdx == 1) + "\n")
	b.WriteString(wizSelector("Pace", goalOptions[gm.rateIdx].label, gm.fieldIdx == 2) + "\n")
	if gm.msg != "" {
		b.WriteString("\n" + styleWarn.Render(gm.msg) + "\n")
	}
	b.WriteString("\n" + styleFaint.Render("enter save · tab next · ←/→ change · esc cancel"))
	return stylePanel.Width(boxW).Render(b.String())
}
