// Package timer provides a simple count-down timer with pause/resume support.
// It is designed to integrate with BubbleTea via tea.Tick: the app sends a
// TickMsg every second and calls timer.Tick() to advance the state.
package timer

import "time"

// Phase indicates whether the timer is running a focus or break session.
type Phase int

const (
	PhaseFocus Phase = iota
	PhaseBreak
)

// State represents the current state of the pomodoro timer.
type State int

const (
	StateIdle    State = iota // not yet started
	StateRunning              // counting down
	StatePaused               // paused, awaiting resume
	StateDone                 // reached zero
)

// Timer holds all mutable pomodoro state.
type Timer struct {
	Phase     Phase
	State     State
	Remaining time.Duration
	Total     time.Duration // original duration for this phase (used for progress %)
}

// New returns an idle timer configured for a focus session of focusMinutes.
func New(focusMinutes int) Timer {
	d := time.Duration(focusMinutes) * time.Minute
	return Timer{
		Phase:     PhaseFocus,
		State:     StateIdle,
		Remaining: d,
		Total:     d,
	}
}

// Start transitions an idle or paused timer to running.
func (t *Timer) Start() {
	if t.State == StateIdle || t.State == StatePaused {
		t.State = StateRunning
	}
}

// Pause transitions a running timer to paused.
func (t *Timer) Pause() {
	if t.State == StateRunning {
		t.State = StatePaused
	}
}

// Resume transitions a paused timer back to running.
func (t *Timer) Resume() {
	if t.State == StatePaused {
		t.State = StateRunning
	}
}

// Tick advances the timer by one second. Returns true if the timer just
// reached zero (caller should transition to the next phase or show prompt).
func (t *Timer) Tick() bool {
	if t.State != StateRunning {
		return false
	}
	if t.Remaining <= time.Second {
		t.Remaining = 0
		t.State = StateDone
		return true
	}
	t.Remaining -= time.Second
	return false
}

// Extend adds extra minutes to a running or done timer and resumes it.
func (t *Timer) Extend(minutes int) {
	t.Remaining += time.Duration(minutes) * time.Minute
	t.Total += time.Duration(minutes) * time.Minute
	t.State = StateRunning
}

// StartBreak resets the timer for a break phase.
func (t *Timer) StartBreak(breakMinutes int) {
	d := time.Duration(breakMinutes) * time.Minute
	t.Phase = PhaseBreak
	t.State = StateRunning
	t.Remaining = d
	t.Total = d
}

// Progress returns a value in [0.0, 1.0] representing how much of the current
// phase has elapsed (0.0 = just started, 1.0 = complete).
func (t *Timer) Progress() float64 {
	if t.Total == 0 {
		return 0
	}
	elapsed := t.Total - t.Remaining
	p := float64(elapsed) / float64(t.Total)
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return p
}

// HHMMString returns the remaining time formatted as MM:SS.
func (t *Timer) HHMMString() string {
	d := t.Remaining
	if d < 0 {
		d = 0
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return padTwo(h) + ":" + padTwo(m) + ":" + padTwo(s)
	}
	return padTwo(m) + ":" + padTwo(s)
}

func padTwo(n int) string {
	if n < 10 {
		return "0" + intStr(n)
	}
	return intStr(n)
}

func intStr(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
