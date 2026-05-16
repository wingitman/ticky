package app

import (
	"fmt"
	"strings"

	"github.com/wingitman/ticky/internal/ui"
)

func (m Model) renderUpdatePrompt() string {
	commits := m.updateInfo.Available
	rows := len(commits)
	if rows > 5 {
		rows = 5
	}
	start := m.updateCursor - rows/2
	if start < 0 {
		start = 0
	}
	if start+rows > len(commits) {
		start = len(commits) - rows
		if start < 0 {
			start = 0
		}
	}

	var b strings.Builder
	b.WriteString(ui.StyleSuccess.Render("Update available"))
	b.WriteString("\n\n")
	b.WriteString("Current: " + shortCommit(m.updateInfo.CurrentCommit) + "\n")
	b.WriteString("Latest:  " + shortCommit(m.updateInfo.LatestCommit) + "\n")
	if m.updateInfo.Branch != "" {
		b.WriteString("Branch:  " + m.updateInfo.Branch + " -> " + m.updateInfo.Upstream + "\n")
	}
	b.WriteString("\nRecent changes:\n")
	for i := start; i < start+rows && i < len(commits); i++ {
		c := commits[i]
		prefix := "  "
		if i == m.updateCursor {
			prefix = "> "
		}
		line := fmt.Sprintf("%s%s %s", prefix, c.Short, c.Subject)
		if i == m.updateCursor {
			line = ui.StyleSelected.Render(line)
		}
		b.WriteString(line + "\n")
		if m.updateExpanded[c.Hash] && c.Body != "" {
			b.WriteString(ui.StyleMuted.Render(indentLines(c.Body, "    ")) + "\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(ui.StyleMuted.Render("y install in new terminal and exit · enter show/hide details · n/esc skip"))
	return ui.StyleBox.Render(b.String())
}

func (m Model) renderUpdatesScreen() string {
	var b strings.Builder
	b.WriteString(ui.StyleHeader.Render("UPDATES"))
	b.WriteString("\n\n")
	if m.updateChecking {
		b.WriteString(ui.StyleMuted.Render("Checking for updates..."))
		return b.String()
	}
	if m.updateInfo.CheckError != "" {
		b.WriteString(ui.StyleError.Render("Check failed: ") + m.updateInfo.CheckError)
		return b.String()
	}
	if m.updateInfo.RepoPath == "" {
		b.WriteString(ui.StyleMuted.Render("No update information loaded."))
		b.WriteString("\n\n")
		b.WriteString(m.renderStatusBar([]string{"r refresh", m.keys.close + " back"}))
		return b.String()
	}

	b.WriteString(ui.StyleMuted.Render("Repo: ") + m.updateInfo.RepoPath + "\n")
	b.WriteString(ui.StyleMuted.Render("Branch: ") + m.updateInfo.Branch + " -> " + m.updateInfo.Upstream + "\n")
	b.WriteString(ui.StyleMuted.Render("Current: ") + shortCommit(m.updateInfo.CurrentCommit) + "\n")
	b.WriteString(ui.StyleMuted.Render("Latest: ") + shortCommit(m.updateInfo.LatestCommit) + "\n\n")

	commits := m.updateCommits()
	if len(commits) == 0 {
		b.WriteString(ui.StyleSuccess.Render("No newer commits found."))
		b.WriteString("\n\n")
		b.WriteString(m.renderStatusBar([]string{"r refresh", m.keys.close + " back"}))
		return b.String()
	}
	if len(m.updateInfo.Available) > 0 {
		b.WriteString(ui.StyleSuccess.Render(fmt.Sprintf("%d update(s) available", len(m.updateInfo.Available))) + "\n")
	} else {
		b.WriteString(ui.StyleMuted.Render("Recent history") + "\n")
	}

	rows := m.height - 10
	if rows < 1 {
		rows = 1
	}
	if rows > len(commits) {
		rows = len(commits)
	}
	start := m.updateCursor - rows/2
	if start < 0 {
		start = 0
	}
	if start+rows > len(commits) {
		start = len(commits) - rows
		if start < 0 {
			start = 0
		}
	}
	for i := start; i < start+rows && i < len(commits); i++ {
		c := commits[i]
		line := fmt.Sprintf("  %s  %s  %s", c.Short, c.Date, c.Subject)
		if i == m.updateCursor {
			line = ui.StyleSelected.Render(line)
		}
		b.WriteString(line + "\n")
		if m.updateExpanded[c.Hash] && c.Body != "" {
			b.WriteString(ui.StyleMuted.Render(indentLines(c.Body, "    ")) + "\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(m.renderStatusBar([]string{"y latest", "i install selected", "enter details", "r refresh", m.keys.close + " back"}))
	return b.String()
}

func shortCommit(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	if hash == "" {
		return "unknown"
	}
	return hash
}

func indentLines(s string, prefix string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i, line := range lines {
		lines[i] = prefix + strings.TrimSpace(line)
	}
	return strings.Join(lines, "\n")
}
