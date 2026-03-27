// Package overlay formats the status line printed by 'ticky --status'.
// It reads the active session and config, then produces a short string
// suitable for embedding in a shell prompt (PS1, RPROMPT, fish_right_prompt,
// PowerShell prompt function, etc.).
//
// Output examples (depending on config):
//
//	▶ Write unit tests  ⏱ 23:45   (show_task_name + show_time_left)
//	▶ Write unit tests             (show_task_name only)
//	⏱ 23:45                        (show_time_left only)
//	                               (both false — empty string)
package overlay

import (
	"fmt"
	"strings"
	"time"

	"github.com/wingitman/ticky/internal/config"
	"github.com/wingitman/ticky/internal/session"
	"github.com/wingitman/ticky/internal/storage"
)

// StatusLine returns the formatted status string for the current session.
// Returns an empty string if the session is inactive or both display options
// are disabled.
func StatusLine(cfg *config.Config, sess *session.Session, store *storage.Store) string {
	if !session.IsActive(sess) {
		return ""
	}
	if !cfg.Display.ShowTaskName && !cfg.Display.ShowTimeLeft {
		return ""
	}

	var parts []string

	if cfg.Display.ShowTaskName {
		name := taskName(sess, store)
		if name != "" {
			parts = append(parts, "▶ "+name)
		}
	}

	if cfg.Display.ShowTimeLeft {
		rem := session.Remaining(sess)
		if rem > 0 {
			parts = append(parts, "⏱ "+formatRemaining(rem, cfg.Display.TimeFormat))
		} else {
			// Timer is due — show a prompt cue.
			parts = append(parts, "⏱ done")
		}
	}

	return strings.Join(parts, "  ")
}

// CornerWidget returns a short multi-line string (at most 2 lines) suitable
// for rendering in a corner of the ticky TUI. Returns empty string if nothing
// should be shown.
func CornerWidget(cfg *config.Config, sess *session.Session, store *storage.Store) string {
	if !session.IsActive(sess) {
		return ""
	}
	if !cfg.Display.ShowTaskName && !cfg.Display.ShowTimeLeft {
		return ""
	}

	var lines []string

	if cfg.Display.ShowTaskName {
		name := taskName(sess, store)
		if name != "" {
			lines = append(lines, "▶ "+name)
		}
	}

	if cfg.Display.ShowTimeLeft {
		rem := session.Remaining(sess)
		if rem > 0 {
			lines = append(lines, "⏱ "+formatRemaining(rem, cfg.Display.TimeFormat))
		} else {
			lines = append(lines, "⏱ done")
		}
	}

	return strings.Join(lines, "\n")
}

// ShellPromptHints returns shell-specific integration snippets for the
// install completion message, based on the configured overlay_corner.
func ShellPromptHints(corner string) string {
	// left-side corners → left prompt; right-side corners → right prompt
	isRight := corner == "top-right" || corner == "bottom-right"

	if isRight {
		return `
Shell prompt integration (add to your shell config):

  bash:        export RPROMPT='$(ticky --status)'   # if using bash-preexec
  zsh:         RPROMPT='$(ticky --status)'
  fish:        function fish_right_prompt; ticky --status; end
  PowerShell:  # append to your prompt function: "$(ticky --status) "
`
	}
	return `
Shell prompt integration (add to your shell config):

  bash:        export PS1='$(ticky --status) '$PS1
  zsh:         export PROMPT='$(ticky --status) '$PROMPT
  fish:        function fish_prompt; ticky --status; end
  PowerShell:  # prepend to your prompt function: "$(ticky --status) "
`
}

// taskName looks up the task name from the store using the session's TaskID.
func taskName(sess *session.Session, store *storage.Store) string {
	if store == nil {
		return sess.TaskID // fallback
	}
	for _, t := range store.Tasks {
		if t.ID == sess.TaskID {
			return t.Name
		}
	}
	return ""
}

// formatRemaining formats a duration according to the configured time_format.
func formatRemaining(d time.Duration, format string) string {
	total := int(d.Round(time.Second).Seconds())
	if total < 0 {
		total = 0
	}

	switch format {
	case "seconds":
		return fmt.Sprintf("%ds", total)
	case "hhmm", "minutes":
		// Both display as MM:SS for live countdowns (hhmm is clearer here).
		m := total / 60
		s := total % 60
		if m >= 60 {
			h := m / 60
			m = m % 60
			return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
		}
		return fmt.Sprintf("%02d:%02d", m, s)
	case "tshirt":
		minutes := total / 60
		switch {
		case minutes <= 15:
			return "XS"
		case minutes <= 30:
			return "S"
		case minutes <= 60:
			return "M"
		case minutes <= 120:
			return "L"
		case minutes <= 240:
			return "XL"
		default:
			return "XXL"
		}
	case "points":
		minutes := total / 60
		switch {
		case minutes <= 15:
			return "1"
		case minutes <= 30:
			return "2"
		case minutes <= 60:
			return "3"
		case minutes <= 120:
			return "5"
		case minutes <= 240:
			return "8"
		default:
			return "13"
		}
	default:
		m := total / 60
		return fmt.Sprintf("%dm", m)
	}
}
