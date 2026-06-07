package tui

import (
	"context"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"caltui/internal/domain"
	"caltui/internal/food"
	"caltui/internal/nutrition"
	"caltui/internal/store"
)

// onlineDebounce is how long after the last keystroke we query the online API.
const onlineDebounce = 350 * time.Millisecond

// onlineTickMsg fires after the debounce; onlineResultsMsg delivers results.
type onlineTickMsg struct{ gen int }
type onlineResultsMsg struct {
	gen     int
	results []domain.Food
}

const (
	stepSearch = iota
	stepDetail
	stepQuick
	stepNewFood
)

// searchResultsMsg delivers async food-search results, tagged with a generation
// so stale responses are ignored.
type searchResultsMsg struct {
	gen     int
	results []domain.Food
	err     error
}

// searchModal is the add/edit overlay: a food search step, a quantity/unit/meal
// detail step, and a quick-add step for raw calories.
type searchModal struct {
	store *store.Store
	date  string
	step  int
	msg   string

	// search step
	query         textinput.Model
	results       []domain.Food
	onlineResults []domain.Food
	online        food.Provider
	searching     bool
	recent        []domain.Food
	savedMeals    []domain.SavedMeal
	cursor        int
	scroll        int // index of the first visible row
	viewHeight    int // terminal height, for sizing the scroll window
	gen           int

	// detail step
	food         *domain.Food
	name         string
	foodID       *int64
	fixedPerUnit domain.Macros
	qty          textinput.Model
	units        []domain.Unit
	unitIdx      int
	meal         domain.Meal
	editID       int64
	detailFocus  int // 0 qty, 1 unit, 2 meal

	// quick-add step
	quick form

	// new-custom-food step
	newFood form
}

// newSearchModal opens the add flow at the search step.
func newSearchModal(s *store.Store, date string, defaultMeal domain.Meal, recent []domain.Food) *searchModal {
	q := textinput.New()
	q.Placeholder = "Search foods…"
	q.SetWidth(40)
	return &searchModal{store: s, date: date, step: stepSearch, query: q, recent: recent, meal: defaultMeal}
}

// newEditModal opens the detail step to edit an existing entry. Unit changes are
// disabled (we only have the per-unit snapshot, not the source food).
func newEditModal(date string, e domain.LogEntry) *searchModal {
	qty := textinput.New()
	qty.SetWidth(10)
	qty.SetValue(fmtQty(e.Quantity))
	return &searchModal{
		date: date, step: stepDetail,
		name: e.Name, foodID: e.FoodID, fixedPerUnit: e.PerUnit,
		qty: qty, units: []domain.Unit{e.Unit}, meal: e.Meal, editID: e.ID,
	}
}

// focus returns the command to focus the active input for the current step.
func (sm *searchModal) focus() tea.Cmd {
	switch sm.step {
	case stepSearch:
		return sm.query.Focus()
	case stepDetail:
		return sm.qty.Focus()
	case stepQuick:
		return sm.quick.Focus()
	}
	return nil
}

func (sm *searchModal) Update(msg tea.Msg) (modalModel, tea.Cmd) {
	switch msg := msg.(type) {
	case searchResultsMsg:
		if msg.gen == sm.gen {
			if msg.err != nil {
				sm.msg = msg.err.Error()
			} else {
				sm.results = msg.results
				sm.msg = ""
				sm.clampCursor()
			}
		}
		return sm, nil
	case onlineTickMsg:
		if msg.gen == sm.gen && sm.online != nil && strings.TrimSpace(sm.query.Value()) != "" {
			sm.searching = true
			return sm, sm.onlineSearchCmd()
		}
		return sm, nil
	case onlineResultsMsg:
		if msg.gen == sm.gen {
			sm.onlineResults = msg.results
			sm.searching = false
		}
		return sm, nil
	case tea.WindowSizeMsg:
		sm.viewHeight = msg.Height
		return sm, nil
	case tea.KeyPressMsg:
		switch sm.step {
		case stepSearch:
			return sm.updateSearch(msg)
		case stepDetail:
			return sm.updateDetail(msg)
		case stepQuick:
			return sm.updateQuick(msg)
		case stepNewFood:
			return sm.updateNewFood(msg)
		}
	}
	return sm, nil
}

// --- search step ---

// pickRow is one selectable row in the search list: either a food or a saved
// meal.
type pickRow struct {
	food *domain.Food
	meal *domain.SavedMeal
}

// rows returns the current selectable list: with a blank query, saved meals
// (recipes) then recent foods; otherwise offline results followed by online
// results not already present offline (by name).
func (sm *searchModal) rows() []pickRow {
	if strings.TrimSpace(sm.query.Value()) == "" {
		rows := make([]pickRow, 0, len(sm.savedMeals)+len(sm.recent))
		for i := range sm.savedMeals {
			rows = append(rows, pickRow{meal: &sm.savedMeals[i]})
		}
		for i := range sm.recent {
			rows = append(rows, pickRow{food: &sm.recent[i]})
		}
		return rows
	}
	foods := append([]domain.Food{}, sm.results...)
	seen := make(map[string]bool, len(sm.results))
	for _, f := range sm.results {
		seen[strings.ToLower(f.Name)] = true
	}
	for _, f := range sm.onlineResults {
		if !seen[strings.ToLower(f.Name)] {
			foods = append(foods, f)
		}
	}
	rows := make([]pickRow, 0, len(foods))
	for i := range foods {
		rows = append(rows, pickRow{food: &foods[i]})
	}
	return rows
}

func (sm *searchModal) onlineDebounceCmd() tea.Cmd {
	gen := sm.gen
	return tea.Tick(onlineDebounce, func(time.Time) tea.Msg { return onlineTickMsg{gen: gen} })
}

func (sm *searchModal) onlineSearchCmd() tea.Cmd {
	q, gen, online := sm.query.Value(), sm.gen, sm.online
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		res, err := online.Search(ctx, q, 15)
		if err != nil {
			return onlineResultsMsg{gen: gen, results: nil} // degrade silently to offline
		}
		return onlineResultsMsg{gen: gen, results: res}
	}
}

func (sm *searchModal) clampCursor() {
	n := len(sm.rows())
	if sm.cursor >= n {
		sm.cursor = n - 1
	}
	if sm.cursor < 0 {
		sm.cursor = 0
	}
	sm.adjustScroll()
}

// visibleRows is how many result rows fit, derived from the terminal height so
// the list expands on taller terminals. Defaults sanely before the first
// WindowSizeMsg.
func (sm *searchModal) visibleRows() int {
	v := sm.viewHeight - 13 // tab bar, footer, box chrome, hints
	switch {
	case sm.viewHeight <= 0:
		return 8
	case v < 4:
		return 4
	case v > 12:
		return 12
	default:
		return v
	}
}

// adjustScroll keeps the cursor inside the visible window.
func (sm *searchModal) adjustScroll() {
	v := sm.visibleRows()
	if sm.cursor < sm.scroll {
		sm.scroll = sm.cursor
	} else if sm.cursor >= sm.scroll+v {
		sm.scroll = sm.cursor - v + 1
	}
	if sm.scroll < 0 {
		sm.scroll = 0
	}
}

func (sm *searchModal) searchCmd() tea.Cmd {
	q, gen, s := sm.query.Value(), sm.gen, sm.store
	return func() tea.Msg {
		res, err := s.SearchFoods(q, 25)
		return searchResultsMsg{gen: gen, results: res, err: err}
	}
}

func (sm *searchModal) updateSearch(msg tea.KeyPressMsg) (modalModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return sm, closeModalCmd
	case "up", "ctrl+p":
		sm.cursor--
		sm.clampCursor()
		return sm, nil
	case "down", "ctrl+n":
		sm.cursor++
		sm.clampCursor()
		return sm, nil
	case "enter":
		rows := sm.rows()
		if sm.cursor >= 0 && sm.cursor < len(rows) {
			r := rows[sm.cursor]
			if r.meal != nil {
				id := r.meal.ID
				return sm, func() tea.Msg { return logSavedMealMsg{id: id, meal: sm.meal} }
			}
			if r.food != nil {
				return sm, sm.pickFood(*r.food)
			}
		}
		return sm, nil
	case "ctrl+d":
		rows := sm.rows()
		if sm.cursor >= 0 && sm.cursor < len(rows) && rows[sm.cursor].meal != nil {
			id := rows[sm.cursor].meal.ID
			return sm, func() tea.Msg { return deleteSavedMealMsg{id: id} }
		}
		return sm, nil
	case "ctrl+a":
		return sm.startQuick()
	case "ctrl+f":
		return sm.startNewFood()
	default:
		prev := sm.query.Value()
		var cmd tea.Cmd
		sm.query, cmd = sm.query.Update(msg)
		if sm.query.Value() != prev {
			sm.cursor = 0
			sm.scroll = 0
			sm.gen++
			sm.onlineResults = nil
			sm.searching = false
			cmds := []tea.Cmd{cmd, sm.searchCmd()}
			if sm.online != nil && strings.TrimSpace(sm.query.Value()) != "" {
				cmds = append(cmds, sm.onlineDebounceCmd())
			}
			return sm, tea.Batch(cmds...)
		}
		return sm, cmd
	}
}

func (sm *searchModal) pickFood(f domain.Food) tea.Cmd {
	// Cache an online result locally so it gets an id and becomes offline-
	// searchable / eligible for recent & frequent.
	if f.Source == domain.SourceOnlineUSDA && f.ID == 0 && sm.store != nil {
		if id, err := sm.store.UpsertFoodByFDC(f); err == nil {
			f.ID = id
		}
	}
	food := f
	sm.food = &food
	sm.name = f.Name
	if f.ID != 0 {
		id := f.ID
		sm.foodID = &id
	}
	sm.units = unitsFor(&food)
	sm.unitIdx = 0
	defQty := "100"
	if sm.units[0] == domain.UnitServing {
		defQty = "1"
	}
	qty := textinput.New()
	qty.SetWidth(10)
	qty.SetValue(defQty)
	sm.qty = qty
	sm.step = stepDetail
	sm.detailFocus = 0
	sm.msg = ""
	return sm.qty.Focus()
}

func unitsFor(f *domain.Food) []domain.Unit {
	if f != nil && f.ServingSize > 0 {
		return []domain.Unit{domain.UnitServing, domain.UnitGram, domain.UnitOunce, domain.UnitMilliliter}
	}
	return []domain.Unit{domain.UnitGram, domain.UnitOunce, domain.UnitMilliliter}
}

// --- detail step ---

func (sm *searchModal) updateDetail(msg tea.KeyPressMsg) (modalModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if sm.editID != 0 {
			return sm, closeModalCmd
		}
		sm.step = stepSearch
		sm.msg = ""
		return sm, sm.query.Focus()
	case "tab", "down":
		sm.detailFocus = (sm.detailFocus + 1) % 3
		return sm, sm.focusDetail()
	case "shift+tab", "up":
		sm.detailFocus = (sm.detailFocus + 2) % 3
		return sm, sm.focusDetail()
	case "left":
		sm.cycle(-1)
		return sm, nil
	case "right":
		sm.cycle(1)
		return sm, nil
	case "enter":
		return sm.submitDetail()
	default:
		if sm.detailFocus == 0 {
			var cmd tea.Cmd
			sm.qty, cmd = sm.qty.Update(msg)
			return sm, cmd
		}
		return sm, nil
	}
}

func (sm *searchModal) focusDetail() tea.Cmd {
	if sm.detailFocus == 0 {
		return sm.qty.Focus()
	}
	sm.qty.Blur()
	return nil
}

func (sm *searchModal) cycle(d int) {
	switch sm.detailFocus {
	case 1:
		if len(sm.units) > 1 {
			sm.unitIdx = (sm.unitIdx + d + len(sm.units)) % len(sm.units)
		}
	case 2:
		idx := 0
		for i, ml := range domain.MealsInOrder {
			if ml == sm.meal {
				idx = i
			}
		}
		idx = (idx + d + len(domain.MealsInOrder)) % len(domain.MealsInOrder)
		sm.meal = domain.MealsInOrder[idx]
	}
}

func (sm *searchModal) currentUnit() domain.Unit { return sm.units[sm.unitIdx] }

// preview computes the macros for the entered quantity, if valid.
func (sm *searchModal) preview() (domain.Macros, bool) {
	qty, err := strconv.ParseFloat(strings.TrimSpace(sm.qty.Value()), 64)
	if err != nil || qty <= 0 {
		return domain.Macros{}, false
	}
	pu, ok := sm.perUnit()
	if !ok {
		return domain.Macros{}, false
	}
	return pu.Scale(qty), true
}

// perUnit returns the macros for one of the current unit.
func (sm *searchModal) perUnit() (domain.Macros, bool) {
	if sm.food != nil {
		pu, err := nutrition.PerUnitMacros(*sm.food, sm.currentUnit())
		if err != nil {
			return domain.Macros{}, false
		}
		return pu, true
	}
	return sm.fixedPerUnit, true
}

func (sm *searchModal) submitDetail() (modalModel, tea.Cmd) {
	qty, err := strconv.ParseFloat(strings.TrimSpace(sm.qty.Value()), 64)
	if err != nil || qty <= 0 {
		sm.msg = "Enter a valid quantity."
		return sm, nil
	}
	pu, ok := sm.perUnit()
	if !ok {
		sm.msg = "Can't compute macros for this unit."
		return sm, nil
	}
	entry := domain.LogEntry{
		ID: sm.editID, Date: sm.date, Meal: sm.meal, FoodID: sm.foodID,
		Name: sm.name, PerUnit: pu, Quantity: qty, Unit: sm.currentUnit(),
	}
	return sm, func() tea.Msg { return saveEntryMsg{entry: entry} }
}

// --- quick-add step ---

func (sm *searchModal) startQuick() (modalModel, tea.Cmd) {
	sm.quick = newForm(
		fieldSpec{label: "Name", placeholder: "e.g. Protein shake", value: strings.TrimSpace(sm.query.Value()), width: 24, charLimit: 60},
		fieldSpec{label: "Calories", placeholder: "kcal", width: 8, charLimit: 6},
		fieldSpec{label: "Protein g", width: 8, charLimit: 6},
		fieldSpec{label: "Carbs g", width: 8, charLimit: 6},
		fieldSpec{label: "Fat g", width: 8, charLimit: 6},
	)
	sm.step = stepQuick
	sm.msg = ""
	return sm, sm.quick.Focus()
}

func (sm *searchModal) updateQuick(msg tea.KeyPressMsg) (modalModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		sm.step = stepSearch
		return sm, sm.query.Focus()
	case "tab":
		return sm, sm.quick.Next()
	case "shift+tab":
		return sm, sm.quick.Prev()
	case "enter":
		if !sm.quick.AtLast() {
			return sm, sm.quick.Next()
		}
		return sm.submitQuick()
	default:
		return sm, sm.quick.Update(msg)
	}
}

func (sm *searchModal) submitQuick() (modalModel, tea.Cmd) {
	name := sm.quick.Value(0)
	if name == "" {
		name = "Quick add"
	}
	kcal := parseF(sm.quick.Value(1))
	p := parseF(sm.quick.Value(2))
	c := parseF(sm.quick.Value(3))
	f := parseF(sm.quick.Value(4))
	macros := domain.Macros{Kcal: kcal, Protein: p, Carbs: c, Fat: f}
	if kcal <= 0 {
		macros.Kcal = macros.ComputedKcal()
	}
	if macros.Kcal <= 0 && p <= 0 && c <= 0 && f <= 0 {
		sm.msg = "Enter at least calories."
		return sm, nil
	}
	entry := domain.LogEntry{
		Date: sm.date, Meal: sm.meal, Name: name,
		PerUnit: macros, Quantity: 1, Unit: domain.UnitServing,
	}
	return sm, func() tea.Msg { return saveEntryMsg{entry: entry} }
}

// --- new-custom-food step ---

func (sm *searchModal) startNewFood() (modalModel, tea.Cmd) {
	sm.newFood = newForm(
		fieldSpec{label: "Name", placeholder: "e.g. Homemade granola", value: strings.TrimSpace(sm.query.Value()), width: 24, charLimit: 60},
		fieldSpec{label: "Cal /100g", placeholder: "kcal", width: 8, charLimit: 6},
		fieldSpec{label: "Protein /100g", width: 8, charLimit: 6},
		fieldSpec{label: "Carbs /100g", width: 8, charLimit: 6},
		fieldSpec{label: "Fat /100g", width: 8, charLimit: 6},
		fieldSpec{label: "Serving g", placeholder: "optional", width: 8, charLimit: 6},
	)
	sm.step = stepNewFood
	sm.msg = ""
	return sm, sm.newFood.Focus()
}

func (sm *searchModal) updateNewFood(msg tea.KeyPressMsg) (modalModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		sm.step = stepSearch
		return sm, sm.query.Focus()
	case "tab":
		return sm, sm.newFood.Next()
	case "shift+tab":
		return sm, sm.newFood.Prev()
	case "enter":
		if !sm.newFood.AtLast() {
			return sm, sm.newFood.Next()
		}
		return sm.submitNewFood()
	default:
		return sm, sm.newFood.Update(msg)
	}
}

func (sm *searchModal) submitNewFood() (modalModel, tea.Cmd) {
	name := sm.newFood.Value(0)
	if name == "" {
		name = "Custom food"
	}
	macros := domain.Macros{
		Kcal:    parseF(sm.newFood.Value(1)),
		Protein: parseF(sm.newFood.Value(2)),
		Carbs:   parseF(sm.newFood.Value(3)),
		Fat:     parseF(sm.newFood.Value(4)),
	}
	if macros.Kcal <= 0 {
		macros.Kcal = macros.ComputedKcal()
	}
	if macros.Kcal <= 0 {
		sm.msg = "Enter calories or macros."
		return sm, nil
	}
	f := domain.Food{Source: domain.SourceCustom, Name: name, Per100g: macros}
	if serving := parseF(sm.newFood.Value(5)); serving > 0 {
		f.ServingSize = serving
		f.ServingUnit = domain.UnitGram
		f.Household = "1 serving (" + fmtQty(serving) + " g)"
	}
	if sm.store != nil {
		id, err := sm.store.InsertFood(f)
		if err != nil {
			sm.msg = err.Error()
			return sm, nil
		}
		f.ID = id
	}
	// Saved & reusable — flow straight into logging it now.
	return sm, sm.pickFood(f)
}

func parseF(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v
}
