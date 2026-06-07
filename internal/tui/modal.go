package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"caltui/internal/domain"
	"caltui/internal/nutrition"
)

// modalModel is an overlay (search, add/edit, wizard) that captures input while
// open. Its Update returns the (possibly updated) modal and a command; returning
// a closeModalMsg command tells the root to dismiss it.
type modalModel interface {
	Update(tea.Msg) (modalModel, tea.Cmd)
	View(width, height int) string
}

// Messages exchanged between modals and the root.
type closeModalMsg struct{}

// saveEntryMsg asks the root to persist an entry: ID == 0 inserts, else updates.
type saveEntryMsg struct{ entry domain.LogEntry }

// saveGoalMsg asks the root to persist a goal.
type saveGoalMsg struct{ goal domain.Goal }

// saveSetupMsg persists the first-run wizard result: the goal AND the entered
// body weight as a weigh-in, so setup populates the weight tracker.
type saveSetupMsg struct {
	goal   domain.Goal
	weight domain.Weight
}

// saveWeightMsg / saveWeightGoalMsg persist a weigh-in / weight goal.
type saveWeightMsg struct{ weight domain.Weight }
type saveWeightGoalMsg struct{ goal domain.WeightGoal }

// Saved-meal messages.
type saveMealMsg struct{ meal domain.SavedMeal }
type logSavedMealMsg struct {
	id   int64
	meal domain.Meal
}
type deleteSavedMealMsg struct{ id int64 }

// mutationDoneMsg signals a store mutation finished; the root closes any modal,
// records an error, and reloads the day.
type mutationDoneMsg struct{ err error }

func closeModalCmd() tea.Msg { return closeModalMsg{} }

// saveEntryCmd inserts or updates a diary entry.
func (m Model) saveEntryCmd(e domain.LogEntry) tea.Cmd {
	s := m.store
	return func() tea.Msg {
		var err error
		if e.ID == 0 {
			_, err = s.AddEntry(e)
		} else {
			err = s.UpdateEntry(e)
		}
		return mutationDoneMsg{err: err}
	}
}

// saveGoalCmd persists a goal (a new dated row).
func (m Model) saveGoalCmd(g domain.Goal) tea.Cmd {
	s := m.store
	return func() tea.Msg {
		_, err := s.AddGoal(g)
		return mutationDoneMsg{err: err}
	}
}

// saveSetupCmd persists the wizard's goal and records the entered weight as
// today's weigh-in (so the weight tracker is seeded during setup).
func (m Model) saveSetupCmd(g domain.Goal, w domain.Weight) tea.Cmd {
	s := m.store
	return func() tea.Msg {
		if _, err := s.AddGoal(g); err != nil {
			return mutationDoneMsg{err: err}
		}
		if w.Kg > 0 {
			if err := s.UpsertWeight(w); err != nil {
				return mutationDoneMsg{err: err}
			}
		}
		return mutationDoneMsg{err: nil}
	}
}

// saveWeightCmd records (upserts) a weigh-in.
func (m Model) saveWeightCmd(w domain.Weight) tea.Cmd {
	s := m.store
	return func() tea.Msg { return mutationDoneMsg{err: s.UpsertWeight(w)} }
}

// saveWeightGoalCmd records the weight goal and, when the current calorie goal
// carries TDEE inputs, recomputes the daily calorie target so the chosen pace
// actually drives the deficit/surplus shown on the dashboard.
func (m Model) saveWeightGoalCmd(wg domain.WeightGoal) tea.Cmd {
	s := m.store
	cur, hasGoal, today := m.goal, m.hasGoal, m.today
	return func() tea.Msg {
		if err := s.SetWeightGoal(wg); err != nil {
			return mutationDoneMsg{err: err}
		}
		if g, ok := recalcCalorieGoal(cur, hasGoal, wg.RatePerWeek, wg.StartKg, today, time.Now()); ok {
			if _, err := s.AddGoal(g); err != nil {
				return mutationDoneMsg{err: err}
			}
		}
		return mutationDoneMsg{err: nil}
	}
}

// recalcCalorieGoal rebuilds a calorie goal from an existing goal's TDEE inputs
// at a new weekly rate, preferring the latest weight. ok is false when the
// inputs are insufficient (e.g. a manual goal without body stats), in which case
// the calorie target is left unchanged.
func recalcCalorieGoal(cur domain.Goal, hasGoal bool, rate, latestKg float64, today string, now time.Time) (domain.Goal, bool) {
	if !hasGoal || cur.HeightCm <= 0 || !cur.Sex.Valid() || !cur.Activity.Valid() {
		return domain.Goal{}, false
	}
	weight := cur.WeightKg
	if latestKg > 0 {
		weight = latestKg
	}
	if weight <= 0 {
		return domain.Goal{}, false
	}
	age, err := nutrition.AgeFromDate(cur.BirthDate, now)
	if err != nil || age <= 0 {
		return domain.Goal{}, false
	}
	bmr := nutrition.BMR(cur.Sex, weight, cur.HeightCm, age)
	tdee := nutrition.TDEE(bmr, cur.Activity)
	target := nutrition.CalorieTarget(tdee, rate)
	split := nutrition.DefaultMacroSplit(target.Kcal, weight)
	return domain.Goal{
		EffectiveDate: today,
		Target:        split.Macros.Round(),
		Sex:           cur.Sex,
		BirthDate:     cur.BirthDate,
		HeightCm:      cur.HeightCm,
		WeightKg:      weight,
		Activity:      cur.Activity,
		GoalRate:      rate,
	}, true
}

// saveMealCmd persists a new saved meal/recipe.
func (m Model) saveMealCmd(sm domain.SavedMeal) tea.Cmd {
	s := m.store
	return func() tea.Msg {
		_, err := s.AddSavedMeal(sm)
		return mutationDoneMsg{err: err}
	}
}

// logSavedMealCmd logs all items of a saved meal into the day.
func (m Model) logSavedMealCmd(id int64, meal domain.Meal) tea.Cmd {
	s, date := m.store, m.today
	return func() tea.Msg {
		_, err := s.LogSavedMeal(id, date, meal)
		return mutationDoneMsg{err: err}
	}
}

// deleteSavedMealCmd removes a saved meal.
func (m Model) deleteSavedMealCmd(id int64) tea.Cmd {
	s := m.store
	return func() tea.Msg { return mutationDoneMsg{err: s.DeleteSavedMeal(id)} }
}

// deleteEntryCmd removes a diary entry.
func (m Model) deleteEntryCmd(id int64) tea.Cmd {
	s := m.store
	return func() tea.Msg { return mutationDoneMsg{err: s.DeleteEntry(id)} }
}

// copyYesterdayCmd copies yesterday's entries into today.
func (m Model) copyYesterdayCmd() tea.Cmd {
	s, today := m.store, m.today
	return func() tea.Msg {
		y, err := prevDate(today)
		if err != nil {
			return mutationDoneMsg{err: err}
		}
		_, err = s.CopyDay(y, today)
		return mutationDoneMsg{err: err}
	}
}

func prevDate(date string) (string, error) {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", err
	}
	return t.AddDate(0, 0, -1).Format("2006-01-02"), nil
}
