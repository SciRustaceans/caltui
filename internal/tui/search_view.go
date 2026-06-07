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
	case stepNewFood:
		content = sm.newFoodView(innerW)
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

	blank := strings.TrimSpace(sm.query.Value()) == ""
	rows := sm.rows()
	heading := "Results"
	if blank {
		heading = "Saved meals & recent"
	}
	b.WriteString(styleDim.Render(heading) + "\n")

	if len(rows) == 0 {
		if blank {
			b.WriteString(styleFaint.Render("  Nothing yet — start typing.") + "\n")
		} else {
			b.WriteString(styleFaint.Render("  No matches. Press ctrl+a to quick-add.") + "\n")
		}
	}

	// Scrolling window around the cursor.
	visible := sm.visibleRows()
	start := sm.scroll
	if start > len(rows)-visible {
		start = len(rows) - visible
	}
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > len(rows) {
		end = len(rows)
	}
	if start > 0 {
		b.WriteString(styleFaint.Render(fmt.Sprintf("  ↑ %d more", start)) + "\n")
	}
	for i := start; i < end; i++ {
		r := rows[i]
		var name, right string
		if r.meal != nil {
			right = fmt.Sprintf("recipe %d kcal", int(r.meal.Total().Kcal+0.5))
			name = truncate(r.meal.Name, w-2-1-len(right))
		} else {
			right = fmt.Sprintf("%d/100g", int(r.food.Per100g.Kcal+0.5))
			name = truncate(r.food.Name, w-2-1-len(right))
		}
		nameW := w - 4 - len(right) // 2 cursor + 1 gap + 1 safety margin
		if nameW < 8 {
			nameW = 8
		}
		if i == sm.cursor {
			b.WriteString(styleSelected.Render(fmt.Sprintf("❯ %-*s %s", nameW, name, right)) + "\n")
		} else {
			b.WriteString("  " + styleText.Render(fmt.Sprintf("%-*s", nameW, name)) + " " + styleDim.Render(right) + "\n")
		}
	}
	if end < len(rows) {
		b.WriteString(styleFaint.Render(fmt.Sprintf("  ↓ %d more", len(rows)-end)) + "\n")
	}
	if sm.searching {
		b.WriteString(styleDim.Render("· searching online…") + "\n")
	}
	if sm.msg != "" {
		b.WriteString(styleWarn.Render(sm.msg) + "\n")
	}
	b.WriteString(styleFaint.Render("↑↓ · enter log · ctrl+a quick-add · ctrl+f new food · ctrl+d del recipe"))
	return b.String()
}

func (sm *searchModal) newFoodView(_ int) string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("New custom food") + "\n\n")
	b.WriteString(sm.newFood.View(13))
	b.WriteString("\n" + styleFaint.Render("per-100g macros · saved & reusable") + "\n")
	if sm.msg != "" {
		b.WriteString(styleWarn.Render(sm.msg) + "\n")
	}
	b.WriteString("\n" + styleFaint.Render("enter next/create · tab move · esc back"))
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
