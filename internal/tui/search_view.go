package tui

import (
	"fmt"
	"strings"
)

// View renders the modal box content for the current step. The root centers it.
// boxW is the panel's content width; inner content is kept to boxW-4 to stay
// clear of the border + horizontal padding.
func (sm *searchModal) View(width, _ int) string {
	boxW := 56
	if width-6 < boxW {
		boxW = width - 6
	}
	if boxW < 34 {
		boxW = 34
	}
	innerW := boxW - 4
	var content string
	switch sm.step {
	case stepDetail:
		content = sm.detailView(innerW)
	case stepQuick:
		content = sm.quickView(innerW)
	default:
		content = sm.searchView(innerW)
	}
	return stylePanel.Width(boxW).Render(content)
}

func (sm *searchModal) searchView(w int) string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("Add food to "+sm.meal.Title()) + "  " + styleDim.Render("(esc cancel)") + "\n")
	b.WriteString(sm.query.View() + "\n")
	b.WriteString(styleFaint.Render(strings.Repeat("─", w)) + "\n")

	list := sm.activeList()
	heading := "Results"
	if strings.TrimSpace(sm.query.Value()) == "" {
		heading = "Recent"
	}
	b.WriteString(styleDim.Render(heading) + "\n")

	if len(list) == 0 {
		if strings.TrimSpace(sm.query.Value()) == "" {
			b.WriteString(styleFaint.Render("  Nothing yet — start typing.\n"))
		} else {
			b.WriteString(styleFaint.Render("  No matches. Press ctrl+a to quick-add.\n"))
		}
	}
	limit := 8
	for i, f := range list {
		if i >= limit {
			b.WriteString(styleFaint.Render(fmt.Sprintf("  …and %d more\n", len(list)-limit)))
			break
		}
		kcal := fmt.Sprintf("%d/100g", int(f.Per100g.Kcal+0.5))
		nameW := w - 2 - 1 - len(kcal) // cursor + space + kcal column
		if nameW < 8 {
			nameW = 8
		}
		name := truncate(f.Name, nameW)
		if i == sm.cursor {
			b.WriteString(styleSelected.Render(fmt.Sprintf("❯ %-*s %s", nameW, name, kcal)) + "\n")
		} else {
			b.WriteString("  " + styleText.Render(fmt.Sprintf("%-*s", nameW, name)) + " " + styleDim.Render(kcal) + "\n")
		}
	}
	if sm.msg != "" {
		b.WriteString(styleWarn.Render(sm.msg) + "\n")
	}
	b.WriteString(styleFaint.Render("↑↓ move · enter select · ctrl+a quick-add"))
	return b.String()
}

func (sm *searchModal) detailView(_ int) string {
	var b strings.Builder
	b.WriteString(styleTitle.Render(truncate(sm.name, 50)) + "\n\n")
	b.WriteString(sm.fieldLine("Quantity", sm.qty.View(), sm.detailFocus == 0, false) + "\n")

	unitVal := string(sm.currentUnit())
	b.WriteString(sm.fieldLine("Unit", unitVal, sm.detailFocus == 1, len(sm.units) > 1) + "\n")
	b.WriteString(sm.fieldLine("Meal", sm.meal.Title(), sm.detailFocus == 2, true) + "\n\n")

	if prev, ok := sm.preview(); ok {
		b.WriteString(styleGood.Render(fmt.Sprintf("≈ %s kcal", fmtInt(prev.Kcal))) +
			styleDim.Render(fmt.Sprintf("  P%d C%d F%d\n",
				int(prev.Protein+0.5), int(prev.Carbs+0.5), int(prev.Fat+0.5))))
	}
	if sm.msg != "" {
		b.WriteString(styleWarn.Render(sm.msg) + "\n")
	}
	action := "log"
	if sm.editID != 0 {
		action = "save"
	}
	b.WriteString("\n" + styleFaint.Render("enter "+action+" · tab next · ←/→ change · esc back"))
	return b.String()
}

func (sm *searchModal) quickView(_ int) string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("Quick add to "+sm.meal.Title()) + "\n\n")
	b.WriteString(sm.quick.View(9))
	if sm.msg != "" {
		b.WriteString("\n" + styleWarn.Render(sm.msg) + "\n")
	}
	b.WriteString("\n" + styleFaint.Render("enter next/log · tab move · esc back"))
	return b.String()
}

// fieldLine renders a labeled value with a focus marker; arrows wrap cyclable
// selectors.
func (sm *searchModal) fieldLine(label, value string, focused, cyclable bool) string {
	marker := "  "
	ls := styleDim
	if focused {
		marker = styleSelected.Render("❯ ")
		ls = styleSelected
	}
	if cyclable {
		value = "‹ " + value + " ›"
	}
	return marker + ls.Render(fmt.Sprintf("%-9s", label)) + "  " + styleText.Render(value)
}
