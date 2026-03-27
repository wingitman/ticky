package overlay

import (
	"strings"
	"testing"
	"time"

	"github.com/wingitman/ticky/internal/config"
	"github.com/wingitman/ticky/internal/session"
	"github.com/wingitman/ticky/internal/storage"
)

func makeSession(taskID string, minutesLeft int) *session.Session {
	return &session.Session{
		TaskID:  taskID,
		EndTime: time.Now().Add(time.Duration(minutesLeft) * time.Minute),
		Phase:   session.PhaseFocus,
	}
}

func makeStore(taskID, name string) *storage.Store {
	return &storage.Store{
		Tasks: []storage.Task{
			{ID: taskID, Name: name, FocusTime: 25, BreakTime: 5},
		},
	}
}

func TestStatusLineEmpty_NoSession(t *testing.T) {
	cfg := config.Default()
	cfg.Display.ShowTaskName = true
	cfg.Display.ShowTimeLeft = true

	line := StatusLine(cfg, &session.Session{}, nil)
	if line != "" {
		t.Errorf("expected empty string for inactive session, got %q", line)
	}
}

func TestStatusLineEmpty_BothDisabled(t *testing.T) {
	cfg := config.Default()
	cfg.Display.ShowTaskName = false
	cfg.Display.ShowTimeLeft = false

	sess := makeSession("t1", 10)
	line := StatusLine(cfg, sess, makeStore("t1", "Write tests"))
	if line != "" {
		t.Errorf("expected empty string when both disabled, got %q", line)
	}
}

func TestStatusLineTaskName(t *testing.T) {
	cfg := config.Default()
	cfg.Display.ShowTaskName = true
	cfg.Display.ShowTimeLeft = false

	sess := makeSession("t1", 10)
	line := StatusLine(cfg, sess, makeStore("t1", "Write tests"))
	if !strings.Contains(line, "Write tests") {
		t.Errorf("expected task name in output, got %q", line)
	}
	if !strings.HasPrefix(line, "▶") {
		t.Errorf("expected '▶' prefix, got %q", line)
	}
}

func TestStatusLineTimeLeft(t *testing.T) {
	cfg := config.Default()
	cfg.Display.ShowTaskName = false
	cfg.Display.ShowTimeLeft = true

	sess := makeSession("t1", 5)
	line := StatusLine(cfg, sess, makeStore("t1", "Write tests"))
	if !strings.Contains(line, "⏱") {
		t.Errorf("expected '⏱' in output, got %q", line)
	}
}

func TestStatusLineBoth(t *testing.T) {
	cfg := config.Default()
	cfg.Display.ShowTaskName = true
	cfg.Display.ShowTimeLeft = true

	sess := makeSession("t1", 5)
	line := StatusLine(cfg, sess, makeStore("t1", "Write tests"))
	if !strings.Contains(line, "Write tests") || !strings.Contains(line, "⏱") {
		t.Errorf("expected both task name and timer in output, got %q", line)
	}
}

func TestFormatRemainingMinutes(t *testing.T) {
	d := 25*time.Minute + 30*time.Second
	s := formatRemaining(d, "minutes")
	// Should be MM:SS format for live display.
	if s != "25:30" {
		t.Errorf("expected '25:30', got %q", s)
	}
}

func TestFormatRemainingSeconds(t *testing.T) {
	d := 90 * time.Second
	s := formatRemaining(d, "seconds")
	if s != "90s" {
		t.Errorf("expected '90s', got %q", s)
	}
}

func TestFormatRemainingTShirt(t *testing.T) {
	tests := []struct {
		min  int
		want string
	}{
		{10, "XS"},
		{25, "S"},
		{45, "M"},
		{90, "L"},
		{180, "XL"},
		{300, "XXL"},
	}
	for _, tt := range tests {
		d := time.Duration(tt.min) * time.Minute
		got := formatRemaining(d, "tshirt")
		if got != tt.want {
			t.Errorf("tshirt(%dm) = %q, want %q", tt.min, got, tt.want)
		}
	}
}

func TestStorageCompletedTasks(t *testing.T) {
	s := &storage.Store{
		Tasks: []storage.Task{
			{ID: "1", Completed: true, Abandoned: false},
			{ID: "2", Completed: false, Abandoned: true},
			{ID: "3", Completed: false, Abandoned: false},
		},
	}
	completed := storage.CompletedTasks(s)
	if len(completed) != 1 {
		t.Errorf("expected 1 completed task, got %d", len(completed))
	}
	if completed[0].ID != "1" {
		t.Errorf("expected task id '1', got %q", completed[0].ID)
	}
}
