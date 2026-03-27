package report

import (
	"testing"
	"time"

	"github.com/wingitman/ticky/internal/storage"
)

func makeTask(id string, focusMin int, startedAt, endedAt time.Time, completed bool, interrupts []storage.Interrupt) storage.Task {
	return storage.Task{
		ID:         id,
		Name:       "Task " + id,
		FocusTime:  focusMin,
		BreakTime:  5,
		StartedAt:  startedAt,
		EndedAt:    endedAt,
		Completed:  completed,
		Interrupts: interrupts,
	}
}

func TestGenerateExcludesActiveTasks(t *testing.T) {
	now := time.Now()
	tasks := []storage.Task{
		makeTask("1", 25, now.Add(-30*time.Minute), now, true, nil),
		makeTask("2", 25, time.Time{}, time.Time{}, false, nil), // active
	}

	r := Generate(tasks, "")
	if r.TotalTasks != 1 {
		t.Errorf("expected 1 task in report, got %d", r.TotalTasks)
	}
}

func TestGenerateDeltaOnTime(t *testing.T) {
	now := time.Now()
	start := now.Add(-25 * time.Minute) // exactly on time
	tasks := []storage.Task{
		makeTask("1", 25, start, now, true, nil),
	}

	r := Generate(tasks, "")
	if r.OnTime != 1 {
		t.Errorf("expected 1 on-time task, got %d", r.OnTime)
	}
	if r.Overran != 0 {
		t.Errorf("expected 0 overran tasks, got %d", r.Overran)
	}
}

func TestGenerateDeltaOverran(t *testing.T) {
	now := time.Now()
	start := now.Add(-32 * time.Minute) // 7 min overrun on a 25 min task
	tasks := []storage.Task{
		makeTask("1", 25, start, now, true, nil),
	}

	r := Generate(tasks, "")
	if r.Overran != 1 {
		t.Errorf("expected 1 overran task, got %d", r.Overran)
	}
	if r.Tasks[0].Delta <= 0 {
		t.Errorf("expected positive delta for overrun task, got %v", r.Tasks[0].Delta)
	}
}

func TestGenerateInterruptCount(t *testing.T) {
	now := time.Now()
	interrupts := []storage.Interrupt{
		{Time: now.Add(-20 * time.Minute), Reason: "Phone call"},
		{Time: now.Add(-10 * time.Minute), Reason: "Slack"},
	}
	tasks := []storage.Task{
		makeTask("1", 25, now.Add(-30*time.Minute), now, true, interrupts),
	}

	r := Generate(tasks, "")
	if r.Tasks[0].InterruptCount != 2 {
		t.Errorf("expected 2 interrupts, got %d", r.Tasks[0].InterruptCount)
	}
}

func TestFormatDelta(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "  0m"},
		{7 * time.Minute, "+7m"},
		{-5 * time.Minute, "-5m"},
		{30 * time.Second, "+<1m"},
	}
	for _, tt := range tests {
		got := FormatDelta(tt.d)
		if got != tt.want {
			t.Errorf("FormatDelta(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "—"},
		{25 * time.Minute, "25m"},
		{90 * time.Minute, "1h30m"},
		{1*time.Hour + 2*time.Minute, "1h02m"},
	}
	for _, tt := range tests {
		got := FormatDuration(tt.d)
		if got != tt.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}
