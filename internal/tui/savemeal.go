package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"caltui/internal/domain"
)

// openSaveMeal builds a recipe from the entries in the meal section the diary
// cursor is on, and opens a name prompt to save it.
func (m Model) openSaveMeal() (tea.Model, tea.Cmd) {
	e, ok := m.selectedEntry()
	if !ok {
		return m, nil // nothing selected / empty diary
	}
	var items []domain.SavedMealItem
	for _, le := range m.entries {
		if le.Meal != e.Meal {
			continue
		}
		items = append(items, domain.SavedMealItem{
			FoodID: le.FoodID, Name: le.Name, PerUnit: le.PerUnit,
			Quantity: le.Quantity, Unit: le.Unit,
		})
	}
	if len(items) == 0 {
		return m, nil
	}
	sm := newSaveMealModal(e.Meal, items)
	m.modal = sm
	return m, sm.focus()
}

// saveMealModal prompts for a name and emits a SavedMeal built from prepared
// items.
type saveMealModal struct {
	meal  domain.Meal
	items []domain.SavedMealItem
	form  form
	msg   string
}

func newSaveMealModal(meal domain.Meal, items []domain.SavedMealItem) *saveMealModal {
	f := newForm(fieldSpec{label: "Name", placeholder: meal.Title() + " recipe", width: 28, charLimit: 60})
	return &saveMealModal{meal: meal, items: items, form: f}
}

func (sm *saveMealModal) focus() tea.Cmd { return sm.form.Focus() }

func (sm *saveMealModal) Update(msg tea.Msg) (modalModel, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return sm, nil
	}
	switch key.String() {
	case "esc":
		return sm, closeModalCmd
	case "enter":
		name := sm.form.Value(0)
		if name == "" {
			sm.msg = "Enter a name."
			return sm, nil
		}
		meal := domain.SavedMeal{Name: name, Kind: "meal", Servings: 1, Items: sm.items}
		return sm, func() tea.Msg { return saveMealMsg{meal: meal} }
	default:
		return sm, sm.form.Update(msg)
	}
}

func (sm *saveMealModal) View(width, _ int) string {
	boxW := 48
	if width-6 < boxW {
		boxW = width - 6
	}
	var b strings.Builder
	var total domain.Macros
	for _, it := range sm.items {
		total = total.Add(it.Total())
	}
	b.WriteString(styleTitle.Render("Save "+sm.meal.Title()+" as a recipe") + "\n\n")
	b.WriteString(sm.form.View(6))
	b.WriteString("\n" + styleDim.Render(
		fmtInt(float64(len(sm.items)))+" items · "+fmtInt(total.Kcal)+" kcal") + "\n")
	if sm.msg != "" {
		b.WriteString(styleWarn.Render(sm.msg) + "\n")
	}
	b.WriteString("\n" + styleFaint.Render("enter save · esc cancel"))
	return stylePanel.Width(boxW).Render(b.String())
}
