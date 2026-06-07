package tui

import (
	"context"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"caltui/internal/config"
	"caltui/internal/food/fdc"
)

// API-key modal states.
const (
	akInput = iota
	akChecking
	akVerified
)

// apiKeyResultMsg carries the outcome of verifying + saving a key.
type apiKeyResultMsg struct {
	key string
	err error
}

// onlineEnabledMsg tells the root to use the verified key for this session.
type onlineEnabledMsg struct{ key string }

// apiKeyModal prompts for a USDA FoodData Central key, verifies it against the
// live API, saves it to the config file, and confirms.
type apiKeyModal struct {
	input textinput.Model
	state int
	msg   string
}

func newAPIKeyModal() *apiKeyModal {
	ti := textinput.New()
	ti.Placeholder = "paste key, or leave blank to skip"
	ti.SetWidth(44)
	ti.CharLimit = 100
	return &apiKeyModal{input: ti}
}

func (a *apiKeyModal) focus() tea.Cmd { return a.input.Focus() }

func (a *apiKeyModal) Update(msg tea.Msg) (modalModel, tea.Cmd) {
	switch msg := msg.(type) {
	case apiKeyResultMsg:
		if msg.err != nil {
			a.state = akInput
			a.msg = "✗ " + friendlyKeyErr(msg.err)
			return a, a.input.Focus()
		}
		a.state = akVerified
		a.msg = "✓ API key verified and saved."
		return a, func() tea.Msg { return onlineEnabledMsg{key: msg.key} }
	case tea.KeyPressMsg:
		switch a.state {
		case akChecking:
			return a, nil // ignore input while verifying
		case akVerified:
			return a, closeModalCmd // any key dismisses
		default:
			switch msg.String() {
			case "esc":
				return a, closeModalCmd // skip → offline only
			case "enter":
				key := strings.TrimSpace(a.input.Value())
				if key == "" {
					return a, closeModalCmd
				}
				a.state = akChecking
				a.msg = ""
				return a, apiKeyCheckCmd(key)
			default:
				var cmd tea.Cmd
				a.input, cmd = a.input.Update(msg)
				return a, cmd
			}
		}
	}
	return a, nil
}

// apiKeyCheckCmd verifies the key against the live API and, on success, saves it
// to the config file (preserving other settings).
func apiKeyCheckCmd(key string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		if err := fdc.New(key).Validate(ctx); err != nil {
			return apiKeyResultMsg{err: err}
		}
		cfg, _ := config.Load()
		cfg.FDCAPIKey = key
		if err := config.Save(cfg); err != nil {
			return apiKeyResultMsg{err: err}
		}
		return apiKeyResultMsg{key: key}
	}
}

func friendlyKeyErr(err error) string {
	s := err.Error()
	switch {
	case strings.Contains(s, "403"), strings.Contains(s, "401"):
		return "key was rejected — double-check it and try again"
	case strings.Contains(s, "429"):
		return "rate limited — wait a moment and retry"
	default:
		return "couldn't verify (network?): " + s
	}
}

func (a *apiKeyModal) View(width, _ int) string {
	boxW := 58
	if width-6 < boxW {
		boxW = width - 6
	}
	var b strings.Builder
	b.WriteString(styleTitle.Render("Connect USDA FoodData Central") + "  " + styleDim.Render("(optional)") + "\n\n")
	b.WriteString(styleDim.Render("Enables online lookup of branded/packaged foods.") + "\n")
	b.WriteString(styleDim.Render("Free key: fdc.nal.usda.gov/api-key-signup") + "\n\n")
	b.WriteString(styleDim.Render("Key") + "  " + a.input.View() + "\n")

	switch a.state {
	case akChecking:
		b.WriteString("\n" + styleDim.Render("Verifying…") + "\n")
		b.WriteString("\n" + styleFaint.Render("contacting USDA FoodData Central"))
	case akVerified:
		b.WriteString("\n" + styleGood.Render(a.msg) + "\n")
		b.WriteString("\n" + styleFaint.Render("press any key to continue"))
	default:
		if a.msg != "" {
			b.WriteString("\n" + styleWarn.Render(a.msg) + "\n")
		}
		b.WriteString("\n" + styleFaint.Render("enter verify & save · esc skip (offline only)"))
	}
	return stylePanel.Width(boxW).Render(b.String())
}
