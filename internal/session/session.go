// Package session manages the persistent timer state written to disk when
// a focus session starts and ticky exits. This lets the background watcher
// process know when the timer is due, and lets ticky re-open directly into
// the break prompt when relaunched.
package session

import (
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/wingitman/ticky/internal/config"
)

// Phase mirrors timer.Phase but is kept as a plain string for TOML clarity.
const (
	PhaseFocus = "focus"
	PhaseBreak = "break"
)

// Session represents an in-progress pomodoro session persisted to disk.
type Session struct {
	TaskID   string    `toml:"task_id"`
	EndTime  time.Time `toml:"end_time"`  // wall-clock time the current phase ends
	Phase    string    `toml:"phase"`     // "focus" or "break"
	WatchPID int       `toml:"watch_pid"` // PID of the --watch subprocess (0 = not started)
	TTY      string    `toml:"tty"`       // controlling TTY path (Unix only; empty on Windows)
	BreakMin int       `toml:"break_min"` // break duration to use after focus ends

	// Pause state — written when the user pauses so other clients can sync
	// and the --watch subprocess knows not to fire while paused.
	Paused   bool      `toml:"paused"`    // true while the timer is paused
	PausedAt time.Time `toml:"paused_at"` // wall-clock time the pause began

	// Environment snapshot — captured at task-start time so the watcher
	// knows how to notify the user when the timer fires.
	InTmux     bool   `toml:"in_tmux"`     // $TMUX was set when the task started
	NvimSocket string `toml:"nvim_socket"` // $NVIM socket path (empty = not in nvim)
	VimContext bool   `toml:"vim_context"` // $VIM or $VIMRUNTIME was set (plain vim)
}

// sessionPath returns the path to session.toml.
func sessionPath() (string, error) {
	dir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "session.toml"), nil
}

// Load reads the session file. Returns an empty Session and no error if the
// file does not exist.
func Load() (*Session, error) {
	path, err := sessionPath()
	if err != nil {
		return &Session{}, err
	}

	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		return &Session{}, nil
	}
	if err != nil {
		return &Session{}, err
	}

	var s Session
	if _, err := toml.DecodeFile(path, &s); err != nil {
		return &Session{}, err
	}
	return &s, nil
}

// Save writes the session to disk atomically.
func Save(s *Session) error {
	path, err := sessionPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

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

// Delete removes the session file. Safe to call when no session exists.
func Delete() error {
	path, err := sessionPath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// IsActive returns true if the session file exists and has a non-zero TaskID.
func IsActive(s *Session) bool {
	return s.TaskID != ""
}

// IsDue returns true if the session's end time has passed, meaning the timer
// has fired and ticky should show the break/done prompt.
func IsDue(s *Session) bool {
	return IsActive(s) && !s.EndTime.IsZero() && time.Now().After(s.EndTime)
}

// Remaining returns how much time is left in the current phase.
// Returns 0 if the session is due or inactive.
func Remaining(s *Session) time.Duration {
	if !IsActive(s) || s.EndTime.IsZero() {
		return 0
	}
	d := time.Until(s.EndTime)
	if d < 0 {
		return 0
	}
	return d
}
