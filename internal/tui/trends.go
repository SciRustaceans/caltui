package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"caltui/internal/charts"
)

const barWeek = 22 // width of the weekly calorie bars

// viewTrends renders a weekly calorie bar chart, a weight plot with a projected
// trend line, and a recent-days table.
func (m *Model) viewTrends(width int) string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("Trends") + "\n\n")

	chartW := width - 14
	if chartW > 80 {
		chartW = 80
	}
	if chartW < 20 {
		chartW = 20
	}

	b.WriteString(styleDim.Render("Calories — this week") + "\n")
	b.WriteString(m.weeklyBars())

	b.WriteString("\n" + styleDim.Render("Weight") + "\n")
	b.WriteString(m.weightChart(chartW))

	b.WriteString("\n" + styleDim.Render("Recent days") + "\n")
	b.WriteString(m.historyTable())
	return b.String()
}

// weeklyBars renders one horizontal calorie bar per day of the week, colored
// green when at/under the goal and red when over.
func (m *Model) weeklyBars() string {
	if len(m.weekKcal) == 0 {
		return styleFaint.Render("  No data yet.\n")
	}
	t, err := time.Parse("2006-01-02", m.today)
	if err != nil {
		return ""
	}
	goal := 0.0
	if m.hasGoal {
		goal = m.goal.Target.Kcal
	}
	maxv := goal
	for _, v := range m.weekKcal {
		if v > maxv {
			maxv = v
		}
	}
	if maxv <= 0 {
		maxv = 1
	}
	n := len(m.weekKcal)
	var b strings.Builder
	for i, v := range m.weekKcal {
		day := t.AddDate(0, 0, -(n-1)+i).Format("Mon")
		fill := colCalorie
		if goal > 0 {
			switch {
			case v == 0:
				fill = colFaint
			case v <= goal:
				fill = colGood
			default:
				fill = colWarn
			}
		}
		fmt.Fprintf(&b, "  %s %s %s\n",
			styleDim.Render(fmt.Sprintf("%-3s", day)),
			renderBar(barWeek, v/maxv, fill),
			styleText.Render(fmt.Sprintf("%5s", fmtInt(v))))
	}
	if goal > 0 {
		b.WriteString(styleFaint.Render(fmt.Sprintf("  goal %s kcal · green ≤ goal, red over\n", fmtInt(goal))))
	}
	return b.String()
}

// weightChart renders the weight series with a projected continuation when a
// trend can be inferred; otherwise a plain line.
func (m *Model) weightChart(chartW int) string {
	if len(m.weights) < 2 {
		return styleFaint.Render("  Log a few weigh-ins to see a trend.\n")
	}
	unit := m.weightUnit()
	actual := make([]float64, len(m.weights))
	for i, w := range m.weights {
		actual[i] = dispWeight(w.Kg, unit)
	}

	projKg, weeklyKg, weeks, ok := m.weightProjection()
	if !ok {
		return charts.LineChart(actual, 6, chartW, 1) + "\n"
	}
	proj := make([]float64, len(projKg))
	for i, v := range projKg {
		proj[i] = dispWeight(v, unit)
	}
	chart := charts.ProjectionChart(actual, proj, 6, chartW, 1)

	arrow := "↓"
	if weeklyKg > 0 {
		arrow = "↑"
	}
	caption := fmt.Sprintf("  actual · projected %s %.2f %s/wk (recent rate)",
		arrow, math.Abs(dispWeight(weeklyKg, unit)), unit)
	if weeks > 0 {
		caption += fmt.Sprintf(" → goal in ~%d wk", int(weeks+0.5))
	}
	return chart + "\n" + styleDim.Render(caption) + "\n"
}

// weightProjection infers the recent rate of change from the weigh-in series and
// projects it forward (in kg). It returns the projected points, the weekly rate,
// and weeks-to-goal (0 when no goal or not converging). ok is false when the
// trend is flat or the series is too short.
func (m *Model) weightProjection() (proj []float64, weeklyKg, weeksToGoal float64, ok bool) {
	if len(m.weights) < 2 {
		return nil, 0, 0, false
	}
	first, last := m.weights[0], m.weights[len(m.weights)-1]
	d1, e1 := time.Parse("2006-01-02", first.Date)
	d2, e2 := time.Parse("2006-01-02", last.Date)
	if e1 != nil || e2 != nil {
		return nil, 0, 0, false
	}
	spanDays := d2.Sub(d1).Hours() / 24
	if spanDays < 1 {
		return nil, 0, 0, false
	}
	ratePerDay := (last.Kg - first.Kg) / spanDays
	weeklyKg = ratePerDay * 7
	if math.Abs(weeklyKg) < 0.05 { // essentially flat — nothing to project
		return nil, 0, 0, false
	}

	stepDays := spanDays / float64(len(m.weights)-1) // avg spacing keeps the slope visually consistent
	stepDelta := ratePerDay * stepDays

	steps := 8
	if m.hasWeightGoal {
		target := m.weightGoal.TargetKg
		converging := (target < last.Kg && ratePerDay < 0) || (target > last.Kg && ratePerDay > 0)
		if converging {
			if s := int((target-last.Kg)/stepDelta + 0.999); s >= 1 && s <= 60 {
				steps = s
			}
			weeksToGoal = (target - last.Kg) / ratePerDay / 7
		}
	}

	proj = make([]float64, steps)
	v := last.Kg
	for i := 0; i < steps; i++ {
		v += stepDelta
		proj[i] = v
	}
	return proj, weeklyKg, weeksToGoal, true
}

func (m *Model) historyTable() string {
	t, err := time.Parse("2006-01-02", m.today)
	if err != nil || len(m.trendKcal) == 0 {
		return ""
	}
	goal := m.goal.Target.Kcal
	n := len(m.trendKcal)
	var b strings.Builder
	for i, shown := n-1, 0; i >= 0 && shown < 8; i, shown = i-1, shown+1 {
		kcal := m.trendKcal[i]
		date := t.AddDate(0, 0, -(n - 1 - i))
		row := fmt.Sprintf("  %-11s %s", date.Format("Mon Jan 2"),
			styleText.Render(fmt.Sprintf("%5s kcal", fmtInt(kcal))))
		switch {
		case m.hasGoal && goal > 0 && kcal > 0:
			if d := kcal - goal; d <= 0 {
				row += "  " + styleGood.Render(fmt.Sprintf("%d under", int(-d+0.5)))
			} else {
				row += "  " + styleWarn.Render(fmt.Sprintf("%d over", int(d+0.5)))
			}
		case kcal == 0:
			row += "  " + styleFaint.Render("(none)")
		}
		b.WriteString(row + "\n")
	}
	return b.String()
}
