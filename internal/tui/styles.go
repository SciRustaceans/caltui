package tui

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"charm.land/lipgloss/v2"
)

// Color palette (ANSI-256 codes, which degrade gracefully on limited terminals).
var (
	colAccent  = lipgloss.Color("69")  // headings / active tab
	colText    = lipgloss.Color("252") // primary text
	colDim     = lipgloss.Color("245") // secondary text
	colFaint   = lipgloss.Color("240") // borders / empty bar
	colGood    = lipgloss.Color("78")  // within target (green)
	colWarn    = lipgloss.Color("203") // over target (red)
	colProtein = lipgloss.Color("110") // P
	colCarbs   = lipgloss.Color("180") // C
	colFat     = lipgloss.Color("215") // F
	colCalorie = lipgloss.Color("114") // calories
)

// Shared styles.
var (
	styleTitle    = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	styleDim      = lipgloss.NewStyle().Foreground(colDim)
	styleFaint    = lipgloss.NewStyle().Foreground(colFaint)
	styleText     = lipgloss.NewStyle().Foreground(colText)
	styleGood     = lipgloss.NewStyle().Foreground(colGood)
	styleWarn     = lipgloss.NewStyle().Foreground(colWarn)
	styleSelected = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	stylePanel    = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colFaint).
			Padding(0, 1)
)

// panel wraps content in a rounded box with an optional title line.
func panel(title, content string, width int) string {
	body := content
	if title != "" {
		body = styleTitle.Render(title) + "\n" + content
	}
	s := stylePanel
	if width > 0 {
		s = s.Width(width)
	}
	return s.Render(body)
}

// renderBar draws a fixed-width bar filled to ratio (0..1+, clamped to 1) using
// block characters. The fill color is provided; over-target callers pass a warn
// color.
func renderBar(width int, ratio float64, fill color.Color) string {
	if width < 1 {
		width = 1
	}
	if ratio < 0 {
		ratio = 0
	}
	filled := int(math.Round(ratio * float64(width)))
	if filled > width {
		filled = width
	}
	full := lipgloss.NewStyle().Foreground(fill).Render(strings.Repeat("█", filled))
	empty := styleFaint.Render(strings.Repeat("░", width-filled))
	return full + empty
}

// macroRow renders one labeled macro progress row: "P ████░░ 95/140g".
func macroRow(label string, have, target float64, barWidth int, fill color.Color) string {
	ratio := 0.0
	if target > 0 {
		ratio = have / target
	}
	c := fill
	if target > 0 && have > target {
		c = colWarn
	}
	bar := renderBar(barWidth, ratio, c)
	amounts := fmt.Sprintf("%d/%dg", int(math.Round(have)), int(math.Round(target)))
	if target <= 0 {
		amounts = fmt.Sprintf("%dg", int(math.Round(have)))
	}
	return fmt.Sprintf("%s %s %s", styleDim.Render(fmt.Sprintf("%-8s", label)), bar, styleText.Render(amounts))
}

// fmtInt rounds and formats a float as an integer string.
func fmtInt(f float64) string { return fmt.Sprintf("%d", int(math.Round(f))) }
