package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// patchDataPath temporarily redirects dataPath() by pointing config.ConfigDir
// to a temp directory via environment manipulation is not straightforward, so
// we test the helpers directly rather than through Load/Save here.

func TestNewID(t *testing.T) {
	a := NewID()
	b := NewID()
	if a == "" {
		t.Error("NewID returned empty string")
	}
	if a == b {
		t.Error("NewID returned duplicate IDs")
	}
}

func TestUpsertAndDeleteTask(t *testing.T) {
	s := &Store{}

	task := Task{ID: "t1", Name: "Test task", FocusTime: 25, BreakTime: 5}
	UpsertTask(s, task)

	if len(s.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(s.Tasks))
	}

	// Update.
	task.Name = "Updated task"
	UpsertTask(s, task)
	if len(s.Tasks) != 1 {
		t.Fatalf("expected 1 task after update, got %d", len(s.Tasks))
	}
	if s.Tasks[0].Name != "Updated task" {
		t.Errorf("expected updated name, got %q", s.Tasks[0].Name)
	}

	// Delete.
	DeleteTask(s, "t1")
	if len(s.Tasks) != 0 {
		t.Fatalf("expected 0 tasks after delete, got %d", len(s.Tasks))
	}
}

func TestUpsertAndDeleteGroup(t *testing.T) {
	s := &Store{}

	g := Group{ID: "g1", Name: "Sprint 1"}
	UpsertGroup(s, g)

	// Assign a task to the group.
	UpsertTask(s, Task{ID: "t1", GroupID: "g1", Name: "Task"})

	if len(s.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(s.Groups))
	}

	// Delete group — tasks should be ungrouped.
	DeleteGroup(s, "g1")
	if len(s.Groups) != 0 {
		t.Fatalf("expected 0 groups after delete, got %d", len(s.Groups))
	}
	if s.Tasks[0].GroupID != "" {
		t.Errorf("expected task GroupID to be empty after group delete, got %q", s.Tasks[0].GroupID)
	}
}

func TestActiveTasks(t *testing.T) {
	s := &Store{
		Tasks: []Task{
			{ID: "1", Name: "Active"},
			{ID: "2", Name: "Done", Completed: true},
			{ID: "3", Name: "Abandoned", Abandoned: true},
		},
	}

	active := ActiveTasks(s)
	if len(active) != 1 {
		t.Errorf("expected 1 active task, got %d", len(active))
	}
	if active[0].ID != "1" {
		t.Errorf("expected active task id '1', got %q", active[0].ID)
	}
}

func TestFinishedTasks(t *testing.T) {
	s := &Store{
		Tasks: []Task{
			{ID: "1", Name: "Active"},
			{ID: "2", Name: "Done", Completed: true},
			{ID: "3", Name: "Abandoned", Abandoned: true},
		},
	}

	finished := FinishedTasks(s)
	if len(finished) != 2 {
		t.Errorf("expected 2 finished tasks, got %d", len(finished))
	}
}

func TestTotalFocusMinutes(t *testing.T) {
	s := &Store{
		Tasks: []Task{
			{ID: "1", GroupID: "g1", FocusTime: 25},
			{ID: "2", GroupID: "g1", FocusTime: 50},
			{ID: "3", GroupID: "g2", FocusTime: 30},
		},
	}

	total := TotalFocusMinutes(s, "g1")
	if total != 75 {
		t.Errorf("expected 75 focus minutes for group g1, got %d", total)
	}
}

func TestSaveAndLoad(t *testing.T) {
	// Use a temp file to exercise Save/Load round-trip directly.
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.toml")

	s := &Store{
		Groups: []Group{
			{ID: "g1", Name: "Sprint 1"},
		},
		Tasks: []Task{
			{
				ID:        "t1",
				GroupID:   "g1",
				Name:      "Write tests",
				FocusTime: 25,
				BreakTime: 5,
				StartedAt: time.Now().Round(time.Second),
				Completed: true,
				Interrupts: []Interrupt{
					{Time: time.Now().Round(time.Second), Reason: "Phone call"},
				},
			},
		},
	}

	// Write directly to the temp path (bypassing dataPath()).
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}

	// We can't call Save directly since it uses dataPath() which reads the real
	// config dir. Encode to the temp file manually using the same logic.
	// This tests that the TOML encoding round-trips correctly.
	enc := newTOMLEncoder(f)
	if err := enc.Encode(s); err != nil {
		f.Close()
		t.Fatalf("encode: %v", err)
	}
	f.Close()

	// Read it back.
	var loaded Store
	if err := decodeTOMLFile(path, &loaded); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(loaded.Groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(loaded.Groups))
	}
	if len(loaded.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(loaded.Tasks))
	}
	if len(loaded.Tasks[0].Interrupts) != 1 {
		t.Errorf("expected 1 interrupt, got %d", len(loaded.Tasks[0].Interrupts))
	}
	if loaded.Tasks[0].Interrupts[0].Reason != "Phone call" {
		t.Errorf("interrupt reason mismatch: %q", loaded.Tasks[0].Interrupts[0].Reason)
	}
}
