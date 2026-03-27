package storage

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/wingitman/ticky/internal/config"
)

// Interrupt records a single pause reason and its timestamp.
type Interrupt struct {
	Time   time.Time `toml:"time"`
	Reason string    `toml:"reason"`
}

// Task is the core unit of work tracked by ticky.
type Task struct {
	ID         string      `toml:"id"`
	GroupID    string      `toml:"group_id"`
	Name       string      `toml:"name"`
	FocusTime  int         `toml:"focus_time"` // minutes
	BreakTime  int         `toml:"break_time"` // minutes
	StartedAt  time.Time   `toml:"started_at"`
	EndedAt    time.Time   `toml:"ended_at"`
	Completed  bool        `toml:"completed"`
	Abandoned  bool        `toml:"abandoned"`
	Interrupts []Interrupt `toml:"interrupts"`
}

// Group organises tasks into named collections.
type Group struct {
	ID   string `toml:"id"`
	Name string `toml:"name"`
}

// Store is the in-memory representation of the tasks data file.
type Store struct {
	Groups []Group `toml:"groups"`
	Tasks  []Task  `toml:"tasks"`
}

// dataPath returns the full path to tasks.toml, stored alongside ticky.toml.
func dataPath() (string, error) {
	dir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tasks.toml"), nil
}

// Load reads tasks and groups from disk. If the file does not exist an empty
// Store is returned without error.
func Load() (*Store, error) {
	path, err := dataPath()
	if err != nil {
		return &Store{}, err
	}

	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		return &Store{}, nil
	}
	if err != nil {
		return &Store{}, err
	}

	var s Store
	if _, err := toml.DecodeFile(path, &s); err != nil {
		return &Store{}, err
	}
	return &s, nil
}

// Save writes the store to disk atomically (write temp file + rename).
func Save(s *Store) error {
	path, err := dataPath()
	if err != nil {
		return err
	}

	// Ensure the directory exists.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	// Write to a temp file in the same directory so rename is same-drive (safe on Windows).
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}

	if err := toml.NewEncoder(f).Encode(s); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}

	return os.Rename(tmp, path)
}

// newTOMLEncoder returns a TOML encoder writing to w. Exposed for tests.
func newTOMLEncoder(w interface{ Write([]byte) (int, error) }) *toml.Encoder {
	return toml.NewEncoder(w)
}

// decodeTOMLFile decodes a TOML file at path into v. Exposed for tests.
func decodeTOMLFile(path string, v interface{}) error {
	_, err := toml.DecodeFile(path, v)
	return err
}

// NewID generates a random 8-byte hex string suitable for use as an ID.
func NewID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails.
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", b)
}

// TotalFocusMinutes returns the sum of FocusTime across all tasks in the store
// that belong to a given group ID.
func TotalFocusMinutes(s *Store, groupID string) int {
	total := 0
	for _, t := range s.Tasks {
		if t.GroupID == groupID {
			total += t.FocusTime
		}
	}
	return total
}

// ActiveTasks returns tasks that are neither completed nor abandoned.
func ActiveTasks(s *Store) []Task {
	var out []Task
	for _, t := range s.Tasks {
		if !t.Completed && !t.Abandoned {
			out = append(out, t)
		}
	}
	return out
}

// FinishedTasks returns tasks that are completed or abandoned.
// Used by the report, which covers both outcomes.
func FinishedTasks(s *Store) []Task {
	var out []Task
	for _, t := range s.Tasks {
		if t.Completed || t.Abandoned {
			out = append(out, t)
		}
	}
	return out
}

// CompletedTasks returns only successfully completed tasks (not abandoned).
// Used by the completed-tasks view.
func CompletedTasks(s *Store) []Task {
	var out []Task
	for _, t := range s.Tasks {
		if t.Completed && !t.Abandoned {
			out = append(out, t)
		}
	}
	return out
}

// FindGroup returns the group with the given ID, or nil.
func FindGroup(s *Store, id string) *Group {
	for i := range s.Groups {
		if s.Groups[i].ID == id {
			return &s.Groups[i]
		}
	}
	return nil
}

// UpsertTask inserts or updates a task by ID.
func UpsertTask(s *Store, t Task) {
	for i := range s.Tasks {
		if s.Tasks[i].ID == t.ID {
			s.Tasks[i] = t
			return
		}
	}
	s.Tasks = append(s.Tasks, t)
}

// DeleteTask removes a task by ID.
func DeleteTask(s *Store, id string) {
	tasks := s.Tasks[:0]
	for _, t := range s.Tasks {
		if t.ID != id {
			tasks = append(tasks, t)
		}
	}
	s.Tasks = tasks
}

// UpsertGroup inserts or updates a group by ID.
func UpsertGroup(s *Store, g Group) {
	for i := range s.Groups {
		if s.Groups[i].ID == g.ID {
			s.Groups[i] = g
			return
		}
	}
	s.Groups = append(s.Groups, g)
}

// DeleteGroup removes a group by ID and ungroups its tasks.
func DeleteGroup(s *Store, id string) {
	groups := s.Groups[:0]
	for _, g := range s.Groups {
		if g.ID != id {
			groups = append(groups, g)
		}
	}
	s.Groups = groups

	for i := range s.Tasks {
		if s.Tasks[i].GroupID == id {
			s.Tasks[i].GroupID = ""
		}
	}
}
