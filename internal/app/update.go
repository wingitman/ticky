package app

import (
	appupdate "github.com/wingitman/ticky/internal/update"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) updateUpdatePrompt(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y", "Y":
		return m, m.launchUpdate(true, "")
	case "enter":
		m.toggleSelectedUpdateDetails()
		return m, nil
	case "esc", "n", "N":
		m.mode = ModeTaskList
		return m, nil
	}
	if matchKey(key, m.keys.up) {
		m.updateCursor--
		m.clampUpdateCursor()
		return m, nil
	}
	if matchKey(key, m.keys.down) {
		m.updateCursor++
		m.clampUpdateCursor()
		return m, nil
	}
	if matchKey(key, m.keys.increase) || matchKey(key, m.keys.decrease) {
		m.toggleSelectedUpdateDetails()
		return m, nil
	}
	return m, nil
}

func (m Model) updateUpdates(key string) (tea.Model, tea.Cmd) {
	switch {
	case key == "esc" || matchKey(key, m.keys.close):
		m.mode = ModeTaskList
		return m, nil
	case matchKey(key, m.keys.up):
		m.updateCursor--
		m.clampUpdateCursor()
		return m, nil
	case matchKey(key, m.keys.down):
		m.updateCursor++
		m.clampUpdateCursor()
		return m, nil
	case matchKey(key, m.keys.confirm) || matchKey(key, m.keys.increase) || matchKey(key, m.keys.decrease):
		m.toggleSelectedUpdateDetails()
		return m, nil
	case key == "r" || key == "R":
		m.updateChecking = true
		return m, checkUpdatesCmd(m.cfg)
	case key == "i" || key == "I":
		if c := m.selectedUpdateCommit(); c != nil {
			return m, m.launchUpdate(false, c.Hash)
		}
		return m, nil
	case key == "y" || key == "Y":
		return m, m.launchUpdate(true, "")
	}
	return m, nil
}

func (m *Model) updateCommits() []appupdate.Commit {
	if len(m.updateInfo.Available) > 0 {
		return m.updateInfo.Available
	}
	return m.updateInfo.History
}

func (m *Model) selectedUpdateCommit() *appupdate.Commit {
	commits := m.updateCommits()
	if len(commits) == 0 || m.updateCursor < 0 || m.updateCursor >= len(commits) {
		return nil
	}
	return &commits[m.updateCursor]
}

func (m *Model) clampUpdateCursor() {
	commits := m.updateCommits()
	if len(commits) == 0 {
		m.updateCursor = 0
		return
	}
	if m.updateCursor < 0 {
		m.updateCursor = 0
	}
	if m.updateCursor >= len(commits) {
		m.updateCursor = len(commits) - 1
	}
}

func (m *Model) toggleSelectedUpdateDetails() {
	c := m.selectedUpdateCommit()
	if c == nil {
		return
	}
	if m.updateExpanded == nil {
		m.updateExpanded = map[string]bool{}
	}
	m.updateExpanded[c.Hash] = !m.updateExpanded[c.Hash]
}
