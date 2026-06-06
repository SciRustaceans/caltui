package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"caltui/internal/charts"
)

// viewTrends renders calorie + weight line charts and a recent-days table.
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

	b.WriteString(styleDim.Render(fmt.Sprintf("Calories — last %d days", trendDays)) + "\n")
	kcal := trimLeadingZeros(m.trendKcal)
	if countPositive(kcal) >= 2 {
		b.WriteString(lipgloss.NewStyle().Foreground(colCalorie).Render(charts.LineChart(kcal, 5, chartW, 0)) + "\n")
	} else {
		b.WriteString(styleFaint.Render("  Log a few days to see a trend.") + "\n")
	}

	b.WriteString("\n" + styleDim.Render("Weight") + "\n")
	if len(m.weights) >= 2 {
		vals := make([]float64, len(m.weights))
		for i, w := range m.weights {
			vals[i] = w.Kg
		}
		b.WriteString(lipgloss.NewStyle().Foreground(colGood).Render(charts.LineChart(vals, 5, chartW, 1)) + "\n")
	} else {
		b.WriteString(styleFaint.Render("  Log a few weigh-ins to see a trend.") + "\n")
	}

	b.WriteString("\n" + styleDim.Render("Recent days") + "\n")
	b.WriteString(m.historyTable())
	return b.String()
}

func (m *Model) historyTable() string {
	t, err := time.Parse("2006-01-02", m.today)
	if err != nil || len(m.trendKcal) == 0 {
		return ""
	}
	goal := m.goal.Target.Kcal
	n := len(m.trendKcal)
	var b strings.Builder
	for i, shown := n-1, 0; i >= 0 && shown < 10; i, shown = i-1, shown+1 {
		kcal := m.trendKcal[i]
		date := t.AddDate(0, 0, -(n - 1 - i))
		row := fmt.Sprintf("  %-11s %s", date.Format("Mon Jan 2"),
			styleText.Render(fmt.Sprintf("%5s kcal", fmtInt(kcal))))
		if m.hasGoal && goal > 0 && kcal > 0 {
			d := kcal - goal
			if d <= 0 {
				row += "  " + styleGood.Render(fmt.Sprintf("%d under", int(-d+0.5)))
			} else {
				row += "  " + styleWarn.Render(fmt.Sprintf("%d over", int(d+0.5)))
			}
		} else if kcal == 0 {
			row += "  " + styleFaint.Render("(none)")
		}
		b.WriteString(row + "\n")
	}
	return b.String()
}

func countPositive(vals []float64) int {
	n := 0
	for _, v := range vals {
		if v > 0 {
			n++
		}
	}
	return n
}

// trimLeadingZeros drops leading zero days so a sparse early history doesn't
// flatten the chart.
func trimLeadingZeros(vals []float64) []float64 {
	i := 0
	for i < len(vals) && vals[i] == 0 {
		i++
	}
	return vals[i:]
}
