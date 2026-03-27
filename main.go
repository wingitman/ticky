package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wingitman/ticky/internal/app"
	"github.com/wingitman/ticky/internal/config"
	"github.com/wingitman/ticky/internal/overlay"
	"github.com/wingitman/ticky/internal/session"
	"github.com/wingitman/ticky/internal/storage"
)

func main() {
	args := os.Args[1:]

	switch {
	case hasFlag(args, "--watch"):
		runWatch()

	case hasFlag(args, "--status"):
		runStatus()

	case hasFlag(args, "--check"):
		runCheck()

	default:
		// Normal TUI launch, optionally with a pre-opened TTY fd.
		ttyPath := flagValue(args, "--tty")
		runTUI(ttyPath)
	}
}

// ─── Normal TUI ───────────────────────────────────────────────────────────────

func runTUI(ttyPath string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ticky: config warning: %v\n", err)
	}

	store, err := storage.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ticky: storage warning: %v\n", err)
	}

	sess, err := session.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ticky: session warning: %v\n", err)
	}

	model := app.New(cfg, store, sess)

	// When relaunched by --watch via reexecOnTTY, a --tty path is passed so
	// BubbleTea uses the already-open FD instead of independently opening
	// /dev/tty (which fails inside neovim and other non-interactive contexts).
	var p *tea.Program
	if ttyPath != "" {
		ttyFile, err := os.OpenFile(ttyPath, os.O_RDWR, 0)
		if err == nil {
			p = tea.NewProgram(model,
				tea.WithAltScreen(),
				tea.WithInput(ttyFile),
				tea.WithOutput(ttyFile),
			)
		}
	}
	if p == nil {
		p = tea.NewProgram(model, tea.WithAltScreen())
	}

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "ticky: %v\n", err)
		os.Exit(1)
	}
}

// ─── --watch ─────────────────────────────────────────────────────────────────

// runWatch sleeps until the session EndTime, then notifies the user via the
// most appropriate method for their current environment.
//
// Priority order:
//  1. $TMUX was set at task-start → tmux display-popup (clean, no PTY fight)
//  2. $NVIM was set at task-start → skip TUI; ticky.nvim handles it via --check
//  3. $VIM/$VIMRUNTIME was set   → plain-text notification, no TUI relaunch
//  4. otherwise                  → reexecOnTTY with isRealTTY guard
func runWatch() {
	sess, err := session.Load()
	if err != nil || !session.IsActive(sess) {
		return
	}

	remaining := time.Until(sess.EndTime)
	if remaining > 0 {
		time.Sleep(remaining)
	}

	// Re-check — user may have already dismissed the session from within ticky.
	sess2, err := session.Load()
	if err != nil || !session.IsActive(sess2) {
		return
	}

	notify(sess2)
}

// notify routes the timer-fired notification to the right mechanism.
func notify(sess *session.Session) {
	switch {
	case sess.InTmux:
		// tmux: open a popup over whatever pane the user is in.
		// The popup runs ticky normally; tmux provides a fresh PTY so there
		// is no raw-mode conflict regardless of what's in the main pane.
		notifyTmux()

	case sess.NvimSocket != "" || sess.VimContext:
		// neovim/vim: ticky.nvim polls --check and handles the notification
		// natively inside the editor. We don't attempt TUI relaunch here to
		// avoid the "error entering raw mode" crash.
		// Write a plain-text message as a best-effort fallback for plain vim
		// or neovim without ticky.nvim installed.
		writePlainNotification(sess.TTY)

	default:
		// Plain terminal: relaunch ticky with --tty so BubbleTea uses the
		// pre-opened FD rather than independently acquiring /dev/tty.
		reexecOnTTY(sess.TTY)
	}
}

// ─── --status ────────────────────────────────────────────────────────────────

// runStatus prints a one-line status string for shell prompt integration.
// Prints nothing and exits 0 when no session is active.
func runStatus() {
	cfg, err := config.Load()
	if err != nil {
		return
	}

	sess, err := session.Load()
	if err != nil || !session.IsActive(sess) {
		return
	}

	store, err := storage.Load()
	if err != nil {
		store = &storage.Store{}
	}

	line := overlay.StatusLine(cfg, sess, store)
	if line != "" {
		fmt.Print(line)
	}
}

// ─── --check ─────────────────────────────────────────────────────────────────

// runCheck reports the current session state for polling by ticky.nvim and
// shell scripts. Prints a human-readable message and exits with a status code:
//
//	0  — timer has fired, break prompt is due
//	1  — timer is actively running
//	2  — no active session (idle)
func runCheck() {
	sess, err := session.Load()
	if err != nil || !session.IsActive(sess) {
		fmt.Println("idle: no active session")
		os.Exit(2)
	}

	store, err := storage.Load()
	if err != nil {
		store = &storage.Store{}
	}

	taskName := taskNameFromStore(store, sess.TaskID)

	if session.IsDue(sess) {
		if sess.Phase == session.PhaseBreak {
			fmt.Printf("due: break session complete — open ticky to continue\n")
		} else {
			fmt.Printf("due: focus session complete for %q — open ticky for your break\n", taskName)
		}
		os.Exit(0)
	}

	rem := session.Remaining(sess)
	mins := int(rem.Minutes())
	secs := int(rem.Seconds()) % 60

	if sess.Phase == session.PhaseBreak {
		fmt.Printf("break: %02d:%02d remaining in break for %q\n", mins, secs, taskName)
	} else {
		fmt.Printf("running: %02d:%02d remaining in focus for %q\n", mins, secs, taskName)
	}

	// Also print the session end time and which context the task was started in,
	// useful for ticky.nvim to decide whether to act.
	fmt.Printf("ends_at: %s\n", sess.EndTime.Format("15:04:05"))
	if sess.NvimSocket != "" {
		fmt.Printf("nvim_socket: %s\n", sess.NvimSocket)
	}

	os.Exit(1)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

// flagValue returns the value following flag in args, or "" if not found.
func flagValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// taskNameFromStore looks up a task name by ID for display in --check output.
func taskNameFromStore(store *storage.Store, taskID string) string {
	for _, t := range store.Tasks {
		if t.ID == taskID {
			return t.Name
		}
	}
	return taskID // fallback to ID if not found
}
