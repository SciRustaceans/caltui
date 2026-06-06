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

// manualGoalModal edits calorie + macro targets by hand. Editing a macro
// rebalances the other two to keep the calorie total fixed; editing calories
// rescales the macros proportionally.
type manualGoalModal struct {
	date     string
	form     form
	focusVal string // value of the focused field when it gained focus
	msg      string
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

func (mm *manualGoalModal) focus() tea.Cmd {
	cmd := mm.form.Focus()
	mm.focusVal = mm.form.Value(mm.form.focus)
	return cmd
}

func (mm *manualGoalModal) Update(msg tea.Msg) (modalModel, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return mm, nil
	}
	switch key.String() {
	case "esc":
		return mm, closeModalCmd
	case "tab", "down":
		return mm, mm.move(true)
	case "shift+tab", "up":
		return mm, mm.move(false)
	case "enter":
		mm.balanceIfChanged()
		if !mm.form.AtLast() {
			return mm, mm.move(true)
		}
		return mm.submit()
	default:
		return mm, mm.form.Update(msg)
	}
}

// move rebalances if the leaving field changed, then advances focus.
func (mm *manualGoalModal) move(next bool) tea.Cmd {
	mm.balanceIfChanged()
	var cmd tea.Cmd
	if next {
		cmd = mm.form.Next()
	} else {
		cmd = mm.form.Prev()
	}
	mm.focusVal = mm.form.Value(mm.form.focus)
	return cmd
}

// balanceIfChanged rebalances when the focused field's value differs from when
// it gained focus.
func (mm *manualGoalModal) balanceIfChanged() {
	leaving := mm.form.focus
	if mm.form.Value(leaving) != mm.focusVal {
		mm.balance(leaving)
		mm.focusVal = mm.form.Value(leaving)
	}
}

func macroKcalPerGram(field int) float64 {
	if field == 3 { // fat
		return 9
	}
	return 4 // protein, carbs
}

// balance keeps the calorie total consistent after the user edits `edited`:
// editing calories (field 0) rescales the macros; editing a macro (1-3)
// rebalances the other two so total calories stay fixed.
func (mm *manualGoalModal) balance(edited int) {
	k := parseF(mm.form.Value(0))
	if k <= 0 {
		return
	}
	switch edited {
	case 0:
		mm.rescaleMacros(k)
	case 1, 2, 3:
		mm.rebalanceMacros(edited, k)
	}
}

// rescaleMacros scales the macros to hit k while preserving their ratios; if
// there are no macros yet it applies a default 30P/40C/30F split.
func (mm *manualGoalModal) rescaleMacros(k float64) {
	p, c, f := parseF(mm.form.Value(1)), parseF(mm.form.Value(2)), parseF(mm.form.Value(3))
	total := 4*p + 4*c + 9*f
	if total <= 0 {
		mm.setMacro(1, k*0.30/4)
		mm.setMacro(2, k*0.40/4)
		mm.setMacro(3, k*0.30/9)
		return
	}
	factor := k / total
	mm.setMacro(1, p*factor)
	mm.setMacro(2, c*factor)
	mm.setMacro(3, f*factor)
}

// rebalanceMacros holds `edited` and splits the remaining calories across the
// other two macros in proportion to their current calorie shares.
func (mm *manualGoalModal) rebalanceMacros(edited int, k float64) {
	var o1, o2 int
	for _, i := range []int{1, 2, 3} {
		if i == edited {
			continue
		}
		if o1 == 0 {
			o1 = i
		} else {
			o2 = i
		}
	}
	remaining := k - parseF(mm.form.Value(edited))*macroKcalPerGram(edited)
	if remaining < 0 {
		remaining = 0
	}
	o1kcal := parseF(mm.form.Value(o1)) * macroKcalPerGram(o1)
	o2kcal := parseF(mm.form.Value(o2)) * macroKcalPerGram(o2)
	if total := o1kcal + o2kcal; total > 0 {
		mm.setMacro(o1, remaining*o1kcal/total/macroKcalPerGram(o1))
		mm.setMacro(o2, remaining*o2kcal/total/macroKcalPerGram(o2))
	} else {
		mm.setMacro(o1, remaining/2/macroKcalPerGram(o1))
		mm.setMacro(o2, remaining/2/macroKcalPerGram(o2))
	}
}

func (mm *manualGoalModal) setMacro(field int, grams float64) {
	if grams < 0 {
		grams = 0
	}
	mm.form.fields[field].ti.SetValue(strconv.Itoa(int(grams + 0.5)))
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
	b.WriteString("\n" + styleFaint.Render("editing a macro rebalances the others to keep calories fixed") + "\n")
	if mm.msg != "" {
		b.WriteString(styleWarn.Render(mm.msg) + "\n")
	}
	b.WriteString("\n" + styleFaint.Render("enter next/save · tab move · esc cancel"))
	return stylePanel.Width(boxW).Render(b.String())
}
