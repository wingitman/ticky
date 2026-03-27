package timer

import (
	"testing"
	"time"
)

func TestNewTimer(t *testing.T) {
	tmr := New(25)
	if tmr.Phase != PhaseFocus {
		t.Errorf("expected PhaseFocus, got %v", tmr.Phase)
	}
	if tmr.State != StateIdle {
		t.Errorf("expected StateIdle, got %v", tmr.State)
	}
	if tmr.Remaining != 25*time.Minute {
		t.Errorf("expected 25m remaining, got %v", tmr.Remaining)
	}
}

func TestStartAndPause(t *testing.T) {
	tmr := New(25)
	tmr.Start()
	if tmr.State != StateRunning {
		t.Errorf("expected StateRunning after Start, got %v", tmr.State)
	}

	tmr.Pause()
	if tmr.State != StatePaused {
		t.Errorf("expected StatePaused after Pause, got %v", tmr.State)
	}

	tmr.Resume()
	if tmr.State != StateRunning {
		t.Errorf("expected StateRunning after Resume, got %v", tmr.State)
	}
}

func TestTickDecrements(t *testing.T) {
	tmr := New(1) // 1 minute
	tmr.State = StateRunning
	before := tmr.Remaining

	done := tmr.Tick()
	if done {
		t.Error("expected tick to not be done after one second")
	}
	if tmr.Remaining != before-time.Second {
		t.Errorf("expected remaining to decrease by 1s, got %v", tmr.Remaining)
	}
}

func TestTickReturnsDoneAtZero(t *testing.T) {
	tmr := New(0)
	tmr.Remaining = time.Second
	tmr.State = StateRunning

	done := tmr.Tick()
	if !done {
		t.Error("expected tick to return done when remaining <= 1s")
	}
	if tmr.State != StateDone {
		t.Errorf("expected StateDone, got %v", tmr.State)
	}
}

func TestTickDoesNothingWhenPaused(t *testing.T) {
	tmr := New(25)
	tmr.State = StatePaused
	before := tmr.Remaining

	done := tmr.Tick()
	if done {
		t.Error("expected no done signal when paused")
	}
	if tmr.Remaining != before {
		t.Error("expected remaining unchanged when paused")
	}
}

func TestExtend(t *testing.T) {
	tmr := New(25)
	tmr.State = StateDone
	before := tmr.Remaining // 25m (timer was never ticked)
	tmr.Extend(5)

	if tmr.State != StateRunning {
		t.Errorf("expected StateRunning after Extend, got %v", tmr.State)
	}
	// Extend adds to Remaining, so 25m + 5m = 30m.
	want := before + 5*time.Minute
	if tmr.Remaining != want {
		t.Errorf("expected %v remaining after extend, got %v", want, tmr.Remaining)
	}
}

func TestStartBreak(t *testing.T) {
	tmr := New(25)
	tmr.State = StateDone
	tmr.StartBreak(5)

	if tmr.Phase != PhaseBreak {
		t.Errorf("expected PhaseBreak, got %v", tmr.Phase)
	}
	if tmr.State != StateRunning {
		t.Errorf("expected StateRunning, got %v", tmr.State)
	}
	if tmr.Remaining != 5*time.Minute {
		t.Errorf("expected 5m remaining, got %v", tmr.Remaining)
	}
}

func TestProgress(t *testing.T) {
	tmr := New(10) // 10 minutes total
	tmr.State = StateRunning

	// Simulate 5 minutes elapsed.
	tmr.Remaining = 5 * time.Minute

	p := tmr.Progress()
	if p < 0.49 || p > 0.51 {
		t.Errorf("expected ~0.5 progress, got %f", p)
	}
}

func TestHHMMString(t *testing.T) {
	tmr := New(0)
	tmr.Remaining = 25*time.Minute + 30*time.Second

	s := tmr.HHMMString()
	if s != "25:30" {
		t.Errorf("expected '25:30', got %q", s)
	}
}

func TestHHMMStringHours(t *testing.T) {
	tmr := New(0)
	tmr.Remaining = 1*time.Hour + 5*time.Minute + 3*time.Second

	s := tmr.HHMMString()
	if s != "01:05:03" {
		t.Errorf("expected '01:05:03', got %q", s)
	}
}
