package tui

import (
	"fmt"
	"strings"
	"testing"
)

func TestAPIKeyModalSkip(t *testing.T) {
	a := newAPIKeyModal()
	a.focus()
	// Empty + enter = skip (offline only).
	if _, cmd := a.Update(press("enter")); cmd == nil {
		t.Fatal("empty enter should produce a close command")
	} else if _, ok := cmd().(closeModalMsg); !ok {
		t.Errorf("empty enter should skip/close")
	}
	// esc also skips.
	a = newAPIKeyModal()
	a.focus()
	if _, cmd := a.Update(press("esc")); cmd == nil {
		t.Fatal("esc should close")
	} else if _, ok := cmd().(closeModalMsg); !ok {
		t.Errorf("esc should close")
	}
}

func TestAPIKeyModalVerifyFlow(t *testing.T) {
	a := newAPIKeyModal()
	a.focus()
	typeMM(a, "somekey")
	_, cmd := a.Update(press("enter"))
	if a.state != akChecking {
		t.Fatalf("after enter, state = %d, want checking", a.state)
	}
	if cmd == nil {
		t.Fatal("should issue a verify command")
	}

	// Simulate a successful verify result.
	_, cmd = a.Update(apiKeyResultMsg{key: "somekey"})
	if a.state != akVerified {
		t.Fatalf("after success, state = %d, want verified", a.state)
	}
	oe, ok := cmd().(onlineEnabledMsg)
	if !ok || oe.key != "somekey" {
		t.Errorf("success should emit onlineEnabledMsg{key}, got %T", cmd())
	}
	// In the verified state any key dismisses.
	if _, c := a.Update(press("enter")); c == nil {
		t.Fatal("verified: key should close")
	} else if _, ok := c().(closeModalMsg); !ok {
		t.Errorf("verified: key should close")
	}
}

func TestAPIKeyModalRejected(t *testing.T) {
	a := newAPIKeyModal()
	a.focus()
	typeMM(a, "bad")
	a.Update(press("enter"))
	a.Update(apiKeyResultMsg{err: fmt.Errorf("fdc: search failed: 403 Forbidden")})
	if a.state != akInput {
		t.Errorf("rejected key should return to input, state = %d", a.state)
	}
	if !strings.Contains(a.msg, "rejected") {
		t.Errorf("should explain the rejection, got %q", a.msg)
	}
}

func TestRootEnablesOnline(t *testing.T) {
	s := testStore(t)
	m := New(s, nil)
	if m.online != nil {
		t.Fatal("online should start nil")
	}
	m, _ = update(t, m, onlineEnabledMsg{key: "abc123"})
	if m.online == nil {
		t.Error("onlineEnabledMsg should enable the online provider for the session")
	}
}
