//go:build windows

package main

import (
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

const notificationText = "Focus session complete. Open ticky to handle your break."

// reexecOnTTY on Windows opens ticky in a new console window. The watcher is
// detached from the console that started the timer, so it cannot reuse it;
// a new console works the same whether the user lives in Command Prompt,
// PowerShell, or Windows Terminal (which hosts new consoles when it is the
// default terminal application).
func reexecOnTTY(_ string) {
	self, err := os.Executable()
	if err != nil {
		writePlainNotification("")
		return
	}
	cmd := exec.Command(self)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_CONSOLE | windows.CREATE_NEW_PROCESS_GROUP,
	}
	if err := cmd.Start(); err != nil {
		writePlainNotification("")
		return
	}
	_ = cmd.Process.Release()
}

// notifyTmux is a no-op on Windows — tmux is not a native Windows tool.
// WSL users running tmux will have $TMUX set but their shell is Linux,
// so the Linux build handles that case correctly.
func notifyTmux() {}

// writePlainNotification shows a message box. The watcher has no console, so
// writing to stderr would be invisible.
func writePlainNotification(_ string) {
	text, err := windows.UTF16PtrFromString(notificationText)
	if err != nil {
		return
	}
	caption, err := windows.UTF16PtrFromString("ticky")
	if err != nil {
		return
	}
	_, _ = windows.MessageBox(0, text, caption,
		windows.MB_OK|windows.MB_ICONINFORMATION|windows.MB_SETFOREGROUND|windows.MB_TOPMOST)
}
