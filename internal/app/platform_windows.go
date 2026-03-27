//go:build windows

package app

import (
	"os"
	"os/exec"

	"github.com/wingitman/ticky/internal/session"
)

// EnvSnapshot holds environment variables captured at task-start time.
type EnvSnapshot struct {
	InTmux     bool
	NvimSocket string
	VimContext bool
}

// CaptureEnv on Windows returns an empty snapshot — tmux/nvim/vim env vars
// are not meaningful on Windows in the same way.
func CaptureEnv() EnvSnapshot {
	return EnvSnapshot{}
}

// resolveTTY returns empty on Windows — TTY reattachment is handled differently.
func resolveTTY() string {
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

// setSysProcAttr is a no-op on Windows; the CREATE_NEW_PROCESS_GROUP flag
// would be set here for full detachment, but basic Start() is sufficient
// for our watcher use case.
func setSysProcAttr(cmd *exec.Cmd) {}

// killWatcher terminates the background --watch process. On Windows we use
// os.Process.Kill since SIGTERM is not available.
func killWatcher(sess *session.Session) {
	if sess == nil || sess.WatchPID <= 0 {
		return
	}
	p, err := os.FindProcess(sess.WatchPID)
	if err != nil {
		return
	}
	_ = p.Kill()
}
