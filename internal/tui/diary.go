package tui

import (
	"fmt"
	"strconv"
	"strings"

	"caltui/internal/domain"
)

// viewDiary renders the meal-grouped diary for the current day with a selection
// cursor over entries. The width parameter is reserved for future column
// layout; the diary currently lays out at a fixed content width.
func (m *Model) viewDiary(_ int) string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("Diary — "+m.prettyDate()) +
		"  " + styleDim.Render(fmtInt(m.totals.Kcal)+" kcal") + "\n\n")

	for _, meal := range domain.MealsInOrder {
		items := m.entriesForMeal(meal)
		var mealTot domain.Macros
		for _, it := range items {
			mealTot = mealTot.Add(it.entry.Total())
		}
		head := styleText.Bold(true).Render(strings.ToUpper(string(meal)))
		b.WriteString(head + "  " + styleDim.Render(fmtInt(mealTot.Kcal)+" kcal") + "\n")

		if len(items) == 0 {
			b.WriteString("  " + styleFaint.Render("(empty · press a to add)") + "\n\n")
			continue
		}
		for _, it := range items {
			selected := m.active == tabDiary && it.index == m.diaryCursor
			cursor := "  "
			line := entryLine(it.entry)
			if selected {
				cursor = styleSelected.Render("❯ ")
				line = styleSelected.Render(line)
			}
			b.WriteString(cursor + line + "\n")
		}
		b.WriteString("\n")
	}
	if len(m.entries) == 0 {
		b.WriteString(styleDim.Render("Nothing logged today. Press a to add a food.\n"))
	}
	return b.String()
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
