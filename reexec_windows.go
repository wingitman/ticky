//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
)

// reexecOnTTY on Windows relaunches ticky in the current console.
// Windows console handles stdio attachment automatically for child processes.
func reexecOnTTY(_ string) {
	self, err := os.Executable()
	if err != nil {
		writePlainNotification("")
		return
	}
	cmd := exec.Command(self)
	_ = cmd.Run()
}

// notifyTmux is a no-op on Windows — tmux is not a native Windows tool.
// WSL users running tmux will have $TMUX set but their shell is Linux,
// so the Linux build handles that case correctly.
func notifyTmux() {}

// writePlainNotification writes a plain-text alert to stderr.
func writePlainNotification(_ string) {
	fmt.Fprint(os.Stderr, "\n\nticky: focus session complete — open ticky to handle your break.\n\n")
}
