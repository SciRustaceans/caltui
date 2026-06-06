package tui

import (
	"bytes"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/teatest/v2"

	"caltui/internal/domain"
)

// TestProgramSmoke runs the real Bubble Tea program through teatest: it
// exercises the full lifecycle (Init -> WindowSize -> render) and the actual
// input decoder (typed bytes -> KeyPressMsg -> handlers), which the direct
// Update tests bypass.
func TestProgramSmoke(t *testing.T) {
	s := testStore(t)
	today := time.Now().Format("2006-01-02")
	// Seed a goal so the first-run wizard does not auto-open.
	if _, err := s.AddGoal(domain.Goal{
		EffectiveDate: today,
		Target:        domain.Macros{Kcal: 2100, Protein: 140, Carbs: 210, Fat: 70},
		Manual:        true,
	}); err != nil {
		t.Fatal(err)
	}

	tm := teatest.NewTestModel(t, New(s, nil), teatest.WithInitialTermSize(100, 30))
	out := tm.Output()

	// Initial dashboard render.
	teatest.WaitFor(t, out, func(b []byte) bool {
		return bytes.Contains(b, []byte("Dashboard")) && bytes.Contains(b, []byte("Calories"))
	}, teatest.WithDuration(5*time.Second))

	// A real "2" keystroke must decode and switch to the Diary tab.
	tm.Type("2")
	teatest.WaitFor(t, out, func(b []byte) bool {
		return bytes.Contains(b, []byte("BREAKFAST"))
	}, teatest.WithDuration(5*time.Second))

	// "q" quits.
	tm.Type("q")
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
