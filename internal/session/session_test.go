package session

import (
	"testing"
	"time"
)

func TestIsActive(t *testing.T) {
	if IsActive(&Session{}) {
		t.Error("empty session should not be active")
	}
	if !IsActive(&Session{TaskID: "abc"}) {
		t.Error("session with TaskID should be active")
	}
}

func TestIsDue(t *testing.T) {
	past := &Session{TaskID: "x", EndTime: time.Now().Add(-time.Minute)}
	if !IsDue(past) {
		t.Error("session with past EndTime should be due")
	}

	future := &Session{TaskID: "x", EndTime: time.Now().Add(time.Minute)}
	if IsDue(future) {
		t.Error("session with future EndTime should not be due")
	}
}

func TestRemaining(t *testing.T) {
	// No session.
	if Remaining(&Session{}) != 0 {
		t.Error("expected 0 remaining for inactive session")
	}

	// Future end time.
	in5 := &Session{TaskID: "x", EndTime: time.Now().Add(5 * time.Minute)}
	rem := Remaining(in5)
	if rem < 4*time.Minute || rem > 5*time.Minute {
		t.Errorf("expected ~5m remaining, got %v", rem)
	}

	// Past end time.
	past := &Session{TaskID: "x", EndTime: time.Now().Add(-time.Minute)}
	if Remaining(past) != 0 {
		t.Error("expected 0 for overdue session")
	}
}

func TestEnvSnapshot(t *testing.T) {
	// Verify the session struct carries env fields correctly.
	sess := &Session{
		TaskID:     "t1",
		Phase:      PhaseFocus,
		EndTime:    time.Now().Add(10 * time.Minute),
		InTmux:     true,
		NvimSocket: "/run/user/1000/nvim.1234.0",
		VimContext: false,
	}

	if !sess.InTmux {
		t.Error("expected InTmux to be true")
	}
	if sess.NvimSocket != "/run/user/1000/nvim.1234.0" {
		t.Errorf("unexpected NvimSocket: %q", sess.NvimSocket)
	}
	if sess.VimContext {
		t.Error("expected VimContext to be false")
	}
	if !IsActive(sess) {
		t.Error("expected session to be active")
	}
	if IsDue(sess) {
		t.Error("expected session not to be due yet")
	}
}

func TestPhaseConstants(t *testing.T) {
	if PhaseFocus != "focus" {
		t.Errorf("PhaseFocus = %q, want 'focus'", PhaseFocus)
	}
	if PhaseBreak != "break" {
		t.Errorf("PhaseBreak = %q, want 'break'", PhaseBreak)
	}
}
