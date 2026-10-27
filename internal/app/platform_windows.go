//go:build windows

package app

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/wingitman/ticky/internal/session"
	"golang.org/x/sys/windows"
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

// startWatcherProcess launches `ticky --watch` fully detached from the
// current console so it survives the Command Prompt or PowerShell window
// closing. Breaking away from the parent's job object keeps it alive when the
// console host (or Windows Terminal) tears its job down; some managed
// environments forbid breakaway, so the launch is retried without it.
func startWatcherProcess(self string) (int, error) {
	base := uint32(windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP)
	pid, err := startWithFlags(self, base|windows.CREATE_BREAKAWAY_FROM_JOB)
	if err == nil {
		return pid, nil
	}
	return startWithFlags(self, base)
}

func startWithFlags(self string, flags uint32) (int, error) {
	cmd := exec.Command(self, "--watch")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: flags, HideWindow: true}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	return pid, nil
}

func setDetachedProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
	}
}

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
