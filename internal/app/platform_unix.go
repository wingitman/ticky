//go:build !windows

package app

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/wingitman/ticky/internal/session"
)

// EnvSnapshot holds environment variables captured at task-start time.
// Stored in session.toml so the background watcher knows how to notify
// the user when the timer fires, regardless of what process is running then.
type EnvSnapshot struct {
	InTmux     bool
	NvimSocket string // value of $NVIM (socket path), empty if not in nvim
	VimContext bool   // $VIM or $VIMRUNTIME was set (plain vim)
}

// CaptureEnv snapshots the current environment for notification routing.
func CaptureEnv() EnvSnapshot {
	return EnvSnapshot{
		InTmux:     os.Getenv("TMUX") != "",
		NvimSocket: os.Getenv("NVIM"),
		VimContext: os.Getenv("VIM") != "" || os.Getenv("VIMRUNTIME") != "",
	}
}

// resolveTTY returns the path to the controlling terminal on Unix systems.
func resolveTTY() string {
	// /dev/tty always refers to the controlling terminal of the current process.
	if _, err := os.Stat("/dev/tty"); err == nil {
		return "/dev/tty"
	}
	return ""
}

// resolveSelf returns the path to the running ticky binary.
func resolveSelf() string {
	path, err := os.Executable()
	if err != nil {
		return ""
	}
	return path
}

// setSysProcAttr configures the command to run in its own process group so it
// is not killed when the parent ticky process exits.
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killWatcher terminates the background --watch process recorded in the session.
// Safe to call when no watcher is running (WatchPID == 0 or process already gone).
func killWatcher(sess *session.Session) {
	if sess == nil || sess.WatchPID <= 0 {
		return
	}
	p, err := os.FindProcess(sess.WatchPID)
	if err != nil {
		return
	}
	_ = p.Signal(syscall.SIGTERM)
}
