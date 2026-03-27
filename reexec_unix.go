//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
)

// reexecOnTTY relaunches ticky on the given TTY path so the break prompt
// appears in the user's terminal. Performs an isRealTTY check first —
// if the target isn't a proper character device (e.g. we're inside neovim's
// internal PTY), it falls back to a plain-text notification instead of
// attempting to put a non-interactive FD into raw mode.
func reexecOnTTY(tty string) {
	self, err := os.Executable()
	if err != nil {
		return
	}

	if tty == "" {
		tty = "/dev/tty"
	}

	ttyFile, err := os.OpenFile(tty, os.O_RDWR, 0)
	if err != nil {
		writePlainNotification("")
		return
	}
	defer ttyFile.Close()

	// Guard: only attempt TUI relaunch if the FD is a real character device
	// that supports raw mode. neovim's internal PTY passes the open() but
	// fails tcsetattr, producing the "error entering raw mode" crash.
	if !isRealTTY(ttyFile) {
		writePlainNotification(tty)
		return
	}

	// Pass --tty so runTUI() feeds this FD directly to tea.WithInput /
	// tea.WithOutput instead of letting BubbleTea open /dev/tty a second time.
	cmd := exec.Command(self, "--tty", tty)
	cmd.Stdin = ttyFile
	cmd.Stdout = ttyFile
	cmd.Stderr = ttyFile
	_ = cmd.Run()
}

// notifyTmux opens a tmux display-popup running ticky in the current client.
// The -E flag closes the popup automatically when ticky exits.
func notifyTmux() {
	self, err := os.Executable()
	if err != nil {
		return
	}
	// -E: exit popup when command exits
	// -w / -h: reasonable default size; tmux will constrain to terminal bounds
	cmd := exec.Command("tmux", "display-popup", "-E",
		"-w", "70%", "-h", "50%",
		"-T", " ticky — focus complete ",
		self,
	)
	// display-popup doesn't need stdin/stdout from us — tmux manages it.
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Run()
}

// writePlainNotification writes a simple plain-text alert to the TTY (or
// stderr as fallback). Used when TUI relaunch isn't safe — e.g. inside vim,
// inside neovim without ticky.nvim, or in a non-interactive environment.
// No ANSI sequences, no raw mode — just plain bytes.
func writePlainNotification(tty string) {
	msg := "\n\nticky: focus session complete — open ticky to handle your break.\n\n"

	if tty != "" {
		if f, err := os.OpenFile(tty, os.O_WRONLY, 0); err == nil {
			defer f.Close()
			fmt.Fprint(f, msg)
			return
		}
	}
	fmt.Fprint(os.Stderr, msg)
}

// isRealTTY returns true if f is a character device that supports terminal
// operations. This filters out neovim's internal PTY pseudodevices, pipes,
// and regular files — all of which would fail tcsetattr with EIO.
func isRealTTY(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	// ModeCharDevice is set for /dev/pts/N, /dev/tty, etc.
	// Neovim's internal PTY shows as a char device but tcsetattr fails on it;
	// however the session InTmux/NvimSocket routing already handles that case
	// before we ever reach reexecOnTTY, so this check is a safety net for
	// any remaining edge cases (pipes, regular files, etc.).
	return fi.Mode()&os.ModeCharDevice != 0
}
