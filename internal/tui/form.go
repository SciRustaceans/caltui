package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// fieldSpec describes one text field in a form.
type fieldSpec struct {
	label       string
	placeholder string
	value       string
	width       int
	charLimit   int
}

type field struct {
	label string
	ti    textinput.Model
}

// form is a small vertical stack of labeled text inputs with tab/shift-tab (or
// up/down) focus movement. It is reused by quick-add, the goals wizard, and the
// weight screen.
type form struct {
	fields []field
	focus  int
}

func newForm(specs ...fieldSpec) form {
	f := form{}
	for _, s := range specs {
		ti := textinput.New()
		ti.Placeholder = s.placeholder
		if s.value != "" {
			ti.SetValue(s.value)
		}
		w := s.width
		if w <= 0 {
			w = 18
		}
		ti.SetWidth(w)
		if s.charLimit > 0 {
			ti.CharLimit = s.charLimit
		}
		f.fields = append(f.fields, field{label: s.label, ti: ti})
	}
	return f
}

// Focus focuses the first field and returns its focus command.
func (f *form) Focus() tea.Cmd {
	if len(f.fields) == 0 {
		return nil
	}
	return f.focusField(0)
}

func (f *form) blurAll() {
	for i := range f.fields {
		f.fields[i].ti.Blur()
	}
}

func (f *form) focusField(i int) tea.Cmd {
	f.blurAll()
	f.focus = i
	return f.fields[i].ti.Focus()
}

// Next/Prev move focus, wrapping around.
func (f *form) Next() tea.Cmd { return f.focusField((f.focus + 1) % len(f.fields)) }
func (f *form) Prev() tea.Cmd { return f.focusField((f.focus - 1 + len(f.fields)) % len(f.fields)) }

// AtLast reports whether the last field is focused.
func (f *form) AtLast() bool { return f.focus == len(f.fields)-1 }

// Update routes a message to the focused input.
func (f *form) Update(msg tea.Msg) tea.Cmd {
	if len(f.fields) == 0 {
		return nil
	}
	var cmd tea.Cmd
	f.fields[f.focus].ti, cmd = f.fields[f.focus].ti.Update(msg)
	return cmd
}

// Value returns the trimmed value of field i.
func (f *form) Value(i int) string { return strings.TrimSpace(f.fields[i].ti.Value()) }

// View renders the form with labels right-aligned to labelWidth.
func (f *form) View(labelWidth int) string {
	var b strings.Builder
	for i, fl := range f.fields {
		label := fl.label
		for len([]rune(label)) < labelWidth {
			label = " " + label
		}
		marker := "  "
		labelStyle := styleDim
		if i == f.focus {
			marker = styleSelected.Render("❯ ")
			labelStyle = styleSelected
		}
		b.WriteString(marker + labelStyle.Render(label) + "  " + fl.ti.View() + "\n")
	}
	return b.String()
}
