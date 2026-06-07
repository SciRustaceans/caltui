package tui

import (
	"fmt"
	"strconv"
	"strings"

	"caltui/internal/domain"
)

// viewDiary renders the meal-grouped diary for the current day with a selection
// cursor over entries. The body scrolls (keeping the selected entry in view)
// when it doesn't fit in the available height.
func (m *Model) viewDiary(_, height int) string {
	title := styleTitle.Render("Diary — "+m.prettyDate()) +
		"  " + styleDim.Render(fmtInt(m.totals.Kcal)+" kcal")

	var lines []string
	cursorLine := 0
	for _, meal := range domain.MealsInOrder {
		items := m.entriesForMeal(meal)
		var mealTot domain.Macros
		for _, it := range items {
			mealTot = mealTot.Add(it.entry.Total())
		}
		lines = append(lines, styleText.Bold(true).Render(strings.ToUpper(string(meal)))+
			"  "+styleDim.Render(fmtInt(mealTot.Kcal)+" kcal"))
		if len(items) == 0 {
			lines = append(lines, "  "+styleFaint.Render("(empty · press a to add)"), "")
			continue
		}
		for _, it := range items {
			line := entryLine(it.entry)
			cursor := "  "
			if m.active == tabDiary && it.index == m.diaryCursor {
				cursor = styleSelected.Render("❯ ")
				line = styleSelected.Render(line)
				cursorLine = len(lines)
			}
			lines = append(lines, cursor+line)
		}
		lines = append(lines, "")
	}
	if len(m.entries) == 0 {
		lines = append(lines, styleDim.Render("Nothing logged today. Press a to add a food."))
	}

	// Title + blank line are a fixed header; the rest scrolls.
	return title + "\n\n" + strings.Join(windowLines(lines, cursorLine, height-2), "\n")
}

// windowLines returns a slice of at most height lines centered on focus, with
// "↑ N more" / "↓ N more" indicators replacing the edge lines when content is
// hidden. The focus line is always kept visible.
func windowLines(lines []string, focus, height int) []string {
	if height < 1 {
		height = 1
	}
	if len(lines) <= height {
		return lines
	}
	start := focus - height/2
	if start < 0 {
		start = 0
	}
	if maxStart := len(lines) - height; start > maxStart {
		start = maxStart
	}
	end := start + height

	out := make([]string, 0, height)
	i := start
	if start > 0 {
		out = append(out, styleFaint.Render(fmt.Sprintf("  ↑ %d more", start)))
		i++ // the top line becomes the indicator
	}
	stop := end
	if end < len(lines) {
		stop-- // reserve the bottom line for the indicator
	}
	for ; i < stop; i++ {
		out = append(out, lines[i])
	}
	if end < len(lines) {
		out = append(out, styleFaint.Render(fmt.Sprintf("  ↓ %d more", len(lines)-end)))
	}
	return out
}

// indexedEntry pairs an entry with its global index in m.entries (the cursor
// space).
type indexedEntry struct {
	index int
	entry domain.LogEntry
}

func (m *Model) entriesForMeal(meal domain.Meal) []indexedEntry {
	var out []indexedEntry
	for i, e := range m.entries {
		if e.Meal == meal {
			out = append(out, indexedEntry{index: i, entry: e})
		}
	}
	return out
}

// entryLine formats one diary row: "Oatmeal              100 g    389  P13 C66 F7".
func entryLine(e domain.LogEntry) string {
	t := e.Total()
	name := truncate(e.Name, 28)
	qty := fmt.Sprintf("%s %s", fmtQty(e.Quantity), e.Unit)
	macros := styleDim.Render(fmt.Sprintf("P%d C%d F%d",
		int(t.Protein+0.5), int(t.Carbs+0.5), int(t.Fat+0.5)))
	return fmt.Sprintf("%-28s %8s  %s  %s",
		styleText.Render(name), styleDim.Render(qty),
		styleText.Render(fmt.Sprintf("%4s", fmtInt(t.Kcal))), macros)
}

// fmtQty formats a quantity without trailing zeros (e.g. 100, 1.5).
func fmtQty(q float64) string {
	return strconv.FormatFloat(q, 'f', -1, 64)
}

// selectedEntry returns the entry under the diary cursor, if any.
func (m *Model) selectedEntry() (domain.LogEntry, bool) {
	if m.diaryCursor < 0 || m.diaryCursor >= len(m.entries) {
		return domain.LogEntry{}, false
	}
	return m.entries[m.diaryCursor], true
}

// clampDiaryCursor keeps the cursor within the entry range.
func (m *Model) clampDiaryCursor() {
	if m.diaryCursor < 0 {
		m.diaryCursor = 0
	}
	if m.diaryCursor >= len(m.entries) {
		m.diaryCursor = len(m.entries) - 1
	}
	if len(m.entries) == 0 {
		m.diaryCursor = 0
	}
}
