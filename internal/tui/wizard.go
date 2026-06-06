package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"caltui/internal/domain"
	"caltui/internal/nutrition"
)

// goalOption maps a friendly goal label to a signed weekly rate (kg/week).
type goalOption struct {
	label string
	rate  float64
}

var goalOptions = []goalOption{
	{"Lose 3 kg/week", -3.0},
	{"Lose 2.5 kg/week", -2.5},
	{"Lose 2 kg/week", -2.0},
	{"Lose 1.5 kg/week", -1.5},
	{"Lose 1 kg/week", -1.0},
	{"Lose 0.5 kg/week", -0.5},
	{"Lose 0.25 kg/week", -0.25},
	{"Maintain weight", 0},
	{"Gain 0.25 kg/week", 0.25},
	{"Gain 0.5 kg/week", 0.5},
	{"Gain 1 kg/week", 1.0},
	{"Gain 1.5 kg/week", 1.5},
	{"Gain 2 kg/week", 2.0},
	{"Gain 2.5 kg/week", 2.5},
	{"Gain 3 kg/week", 3.0},
}

// defaultGoalIdx returns the index of the "maintain" option (rate 0).
func defaultGoalIdx() int {
	for i, o := range goalOptions {
		if o.rate == 0 {
			return i
		}
	}
	return 0
}

var sexOptions = []domain.Sex{domain.Male, domain.Female}

// wizard field indices.
const (
	wfSex = iota
	wfAge
	wfHeight
	wfWeight
	wfActivity
	wfGoal
	wfCount
)

// wizardModal collects the inputs for a Mifflin-St Jeor TDEE estimate and
// suggests calorie + macro targets, shown live as the user edits.
type wizardModal struct {
	date  string
	focus int
	msg   string
	now   time.Time

	sexIdx      int
	age         textinput.Model
	height      textinput.Model
	weight      textinput.Model
	activityIdx int
	goalIdx     int
}

func newWizardModal(date string, now time.Time, prefill *domain.Goal) *wizardModal {
	num := func(v string) textinput.Model {
		ti := textinput.New()
		ti.SetWidth(8)
		ti.CharLimit = 6
		if v != "" {
			ti.SetValue(v)
		}
		return ti
	}
	w := &wizardModal{
		date: date, now: now, focus: wfSex,
		age: num(""), height: num(""), weight: num(""),
		sexIdx: 0, activityIdx: 2, goalIdx: defaultGoalIdx(), // male, moderately active, maintain
	}
	if prefill != nil {
		w.prefill(*prefill, now)
	}
	return w
}

func (w *wizardModal) prefill(g domain.Goal, now time.Time) {
	for i, s := range sexOptions {
		if s == g.Sex {
			w.sexIdx = i
		}
	}
	if g.BirthDate != "" {
		if age, err := nutrition.AgeFromDate(g.BirthDate, now); err == nil && age > 0 {
			w.age.SetValue(strconv.Itoa(age))
		}
	}
	if g.HeightCm > 0 {
		w.height.SetValue(trimNum(g.HeightCm))
	}
	if g.WeightKg > 0 {
		w.weight.SetValue(trimNum(g.WeightKg))
	}
	for i, a := range domain.ActivityLevels {
		if a == g.Activity {
			w.activityIdx = i
		}
	}
	for i, o := range goalOptions {
		if o.rate == g.GoalRate {
			w.goalIdx = i
		}
	}
}

func (w *wizardModal) focusActive() tea.Cmd {
	w.age.Blur()
	w.height.Blur()
	w.weight.Blur()
	switch w.focus {
	case wfAge:
		return w.age.Focus()
	case wfHeight:
		return w.height.Focus()
	case wfWeight:
		return w.weight.Focus()
	}
	return nil
}

func (w *wizardModal) Update(msg tea.Msg) (modalModel, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return w, nil
	}
	switch key.String() {
	case "esc":
		return w, closeModalCmd
	case "tab", "down":
		w.focus = (w.focus + 1) % wfCount
		return w, w.focusActive()
	case "shift+tab", "up":
		w.focus = (w.focus + wfCount - 1) % wfCount
		return w, w.focusActive()
	case "left":
		w.cycle(-1)
		return w, nil
	case "right":
		w.cycle(1)
		return w, nil
	case "enter":
		return w.submit()
	default:
		var cmd tea.Cmd
		switch w.focus {
		case wfAge:
			w.age, cmd = w.age.Update(msg)
		case wfHeight:
			w.height, cmd = w.height.Update(msg)
		case wfWeight:
			w.weight, cmd = w.weight.Update(msg)
		}
		return w, cmd
	}
}

func (w *wizardModal) cycle(d int) {
	switch w.focus {
	case wfSex:
		w.sexIdx = (w.sexIdx + d + len(sexOptions)) % len(sexOptions)
	case wfActivity:
		w.activityIdx = (w.activityIdx + d + len(domain.ActivityLevels)) % len(domain.ActivityLevels)
	case wfGoal:
		w.goalIdx = (w.goalIdx + d + len(goalOptions)) % len(goalOptions)
	}
}

// compute returns the suggested target and split if the inputs are valid.
func (w *wizardModal) compute() (nutrition.TargetResult, nutrition.Split, bool) {
	age, err1 := strconv.Atoi(strings.TrimSpace(w.age.Value()))
	h := parseF(w.height.Value())
	wt := parseF(w.weight.Value())
	if err1 != nil || age <= 0 || h <= 0 || wt <= 0 {
		return nutrition.TargetResult{}, nutrition.Split{}, false
	}
	sex := sexOptions[w.sexIdx]
	activity := domain.ActivityLevels[w.activityIdx]
	rate := goalOptions[w.goalIdx].rate
	bmr := nutrition.BMR(sex, wt, h, age)
	tdee := nutrition.TDEE(bmr, activity)
	target := nutrition.CalorieTarget(tdee, rate)
	split := nutrition.DefaultMacroSplit(target.Kcal, wt)
	return target, split, true
}

func (w *wizardModal) submit() (modalModel, tea.Cmd) {
	_, split, ok := w.compute()
	if !ok {
		w.msg = "Enter a valid age, height, and weight."
		return w, nil
	}
	age, _ := strconv.Atoi(strings.TrimSpace(w.age.Value()))
	goal := domain.Goal{
		EffectiveDate: w.date,
		Target:        split.Macros.Round(),
		Sex:           sexOptions[w.sexIdx],
		BirthDate:     nutrition.BirthDateForAge(age, w.now),
		HeightCm:      parseF(w.height.Value()),
		WeightKg:      parseF(w.weight.Value()),
		Activity:      domain.ActivityLevels[w.activityIdx],
		GoalRate:      goalOptions[w.goalIdx].rate,
		Manual:        false,
	}
	return w, func() tea.Msg { return saveGoalMsg{goal: goal} }
}

func (w *wizardModal) View(width, _ int) string {
	boxW := 56
	if width-6 < boxW {
		boxW = width - 6
	}
	if boxW < 40 {
		boxW = 40
	}
	var b strings.Builder
	b.WriteString(styleTitle.Render("Set up your goal") + "  " + styleDim.Render("(esc cancel)") + "\n\n")

	b.WriteString(wizSelector("Sex", string(sexOptions[w.sexIdx]), w.focus == wfSex) + "\n")
	b.WriteString(wizField("Age", w.age.View(), w.focus == wfAge) + "\n")
	b.WriteString(wizField("Height cm", w.height.View(), w.focus == wfHeight) + "\n")
	b.WriteString(wizField("Weight kg", w.weight.View(), w.focus == wfWeight) + "\n")
	b.WriteString(wizSelector("Activity", domain.ActivityLevels[w.activityIdx].Label(), w.focus == wfActivity) + "\n")
	b.WriteString(wizSelector("Goal", goalOptions[w.goalIdx].label, w.focus == wfGoal) + "\n")
	b.WriteString(styleFaint.Render(strings.Repeat("─", boxW-4)) + "\n")

	if target, split, ok := w.compute(); ok {
		m := split.Macros
		b.WriteString(styleGood.Render(fmt.Sprintf("→ %s kcal", fmtInt(target.Kcal))) +
			styleDim.Render(fmt.Sprintf("  %dP / %dC / %dF",
				int(m.Protein+0.5), int(m.Carbs+0.5), int(m.Fat+0.5))) + "\n")
	} else {
		b.WriteString(styleFaint.Render("Fill in age, height, and weight to see a suggestion.\n"))
	}
	if w.msg != "" {
		b.WriteString(styleWarn.Render(w.msg) + "\n")
	}
	b.WriteString("\n" + styleFaint.Render("enter save · tab next · ←/→ change · esc cancel"))
	return stylePanel.Width(boxW).Render(b.String())
}

func wizField(label, value string, focused bool) string {
	marker, ls := "  ", styleDim
	if focused {
		marker, ls = styleSelected.Render("❯ "), styleSelected
	}
	return marker + ls.Render(fmt.Sprintf("%-10s", label)) + "  " + value
}

func wizSelector(label, value string, focused bool) string {
	return wizField(label, styleText.Render("‹ "+value+" ›"), focused)
}

func trimNum(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// --- manual goal editing ---

// manualGoalModal edits calorie + macro targets by hand.
type manualGoalModal struct {
	date string
	form form
	msg  string
}

func newManualGoalModal(date string, cur domain.Goal) *manualGoalModal {
	optInt := func(f float64) string {
		if f > 0 {
			return strconv.Itoa(int(f + 0.5))
		}
		return ""
	}
	f := newForm(
		fieldSpec{label: "Calories", value: optInt(cur.Target.Kcal), width: 8, charLimit: 6},
		fieldSpec{label: "Protein g", value: optInt(cur.Target.Protein), width: 8, charLimit: 6},
		fieldSpec{label: "Carbs g", value: optInt(cur.Target.Carbs), width: 8, charLimit: 6},
		fieldSpec{label: "Fat g", value: optInt(cur.Target.Fat), width: 8, charLimit: 6},
	)
	return &manualGoalModal{date: date, form: f}
}

func (mm *manualGoalModal) focus() tea.Cmd { return mm.form.Focus() }

func (mm *manualGoalModal) Update(msg tea.Msg) (modalModel, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return mm, nil
	}
	switch key.String() {
	case "esc":
		return mm, closeModalCmd
	case "tab":
		return mm, mm.form.Next()
	case "shift+tab":
		return mm, mm.form.Prev()
	case "enter":
		if !mm.form.AtLast() {
			return mm, mm.form.Next()
		}
		return mm.submit()
	default:
		return mm, mm.form.Update(msg)
	}
}

func (mm *manualGoalModal) submit() (modalModel, tea.Cmd) {
	kcal := parseF(mm.form.Value(0))
	p := parseF(mm.form.Value(1))
	c := parseF(mm.form.Value(2))
	f := parseF(mm.form.Value(3))
	macros := domain.Macros{Kcal: kcal, Protein: p, Carbs: c, Fat: f}
	if kcal <= 0 {
		macros.Kcal = macros.ComputedKcal()
	}
	if macros.Kcal <= 0 {
		mm.msg = "Enter a calorie target."
		return mm, nil
	}
	goal := domain.Goal{EffectiveDate: mm.date, Target: macros, Manual: true}
	return mm, func() tea.Msg { return saveGoalMsg{goal: goal} }
}

func (mm *manualGoalModal) View(width, _ int) string {
	boxW := 44
	if width-6 < boxW {
		boxW = width - 6
	}
	var b strings.Builder
	b.WriteString(styleTitle.Render("Edit goal manually") + "\n\n")
	b.WriteString(mm.form.View(9))
	if mm.msg != "" {
		b.WriteString("\n" + styleWarn.Render(mm.msg) + "\n")
	}
	b.WriteString("\n" + styleFaint.Render("enter next/save · tab move · esc cancel"))
	return stylePanel.Width(boxW).Render(b.String())
}
