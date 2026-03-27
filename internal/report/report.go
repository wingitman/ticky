// Package report generates human-readable summaries of completed tasks,
// comparing expected durations against actual durations and surfacing
// interrupt patterns.
package report

import (
	"time"

	"github.com/wingitman/ticky/internal/storage"
)

// TaskReport holds the report data for a single task.
type TaskReport struct {
	Task             storage.Task
	ExpectedDuration time.Duration
	ActualDuration   time.Duration
	Delta            time.Duration // Actual - Expected (positive = overran)
	InterruptCount   int
	TotalPauseTime   time.Duration // not tracked per-interrupt, reserved for future
}

// Report is the full report for a set of tasks.
type Report struct {
	Tasks      []TaskReport
	GroupName  string
	TotalTasks int
	OnTime     int // tasks where actual <= expected
	Overran    int // tasks where actual > expected
}

// Generate builds a Report from the given tasks. Only completed or abandoned
// tasks are included (active tasks have no ended_at).
func Generate(tasks []storage.Task, groupName string) Report {
	r := Report{GroupName: groupName}

	for _, t := range tasks {
		if !t.Completed && !t.Abandoned {
			continue
		}

		expected := time.Duration(t.FocusTime) * time.Minute

		var actual time.Duration
		if !t.StartedAt.IsZero() && !t.EndedAt.IsZero() {
			actual = t.EndedAt.Sub(t.StartedAt)
		}

		delta := actual - expected

		tr := TaskReport{
			Task:             t,
			ExpectedDuration: expected,
			ActualDuration:   actual,
			Delta:            delta,
			InterruptCount:   len(t.Interrupts),
		}

		r.Tasks = append(r.Tasks, tr)
		r.TotalTasks++
		if delta <= 0 {
			r.OnTime++
		} else {
			r.Overran++
		}
	}

	return r
}

// FormatDelta returns a human-readable delta string like "+7m" or "-5m".
func FormatDelta(d time.Duration) string {
	if d == 0 {
		return "  0m"
	}
	sign := "+"
	if d < 0 {
		sign = "-"
		d = -d
	}
	minutes := int(d.Minutes())
	if minutes == 0 {
		return sign + "<1m"
	}
	return sign + itoa(minutes) + "m"
}

// FormatDuration returns a short duration string like "25m" or "1h02m".
func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "—"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return itoa(h) + "h" + pad(m) + "m"
	}
	return itoa(m) + "m"
}

func itoa(n int) string {
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

func pad(n int) string {
	if n < 10 {
		return "0" + itoa(n)
	}
	return itoa(n)
}
