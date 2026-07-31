package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/wingitman/ticky/internal/config"
	"github.com/wingitman/ticky/internal/session"
	"github.com/wingitman/ticky/internal/storage"
)

func TestTaskListTasksGroupsWithoutMutatingStore(t *testing.T) {
	cfg := config.Default()
	store := &storage.Store{
		Groups: []storage.Group{{ID: "g1", Name: "Work"}, {ID: "g2", Name: "Learn"}},
		Tasks: []storage.Task{
			{ID: "ungrouped", Name: "Loose"},
			{ID: "learn", GroupID: "g2", Name: "Read"},
			{ID: "work", GroupID: "g1", Name: "Ship"},
		},
	}
	m := New(cfg, store, &session.Session{}, false)
	got := m.taskListTasks()
	if got[0].ID != "work" || got[1].ID != "learn" || got[2].ID != "ungrouped" {
		t.Fatalf("unexpected grouped order: %#v", got)
	}
	if store.Tasks[0].ID != "ungrouped" {
		t.Fatal("group projection mutated persisted task order")
	}
}

func TestDeleteRequiresConfirmation(t *testing.T) {
	cfg := config.Default()
	store := &storage.Store{Tasks: []storage.Task{{ID: "t1", Name: "Remove me"}}}
	m := New(cfg, store, &session.Session{}, false)
	m.width, m.height = 80, 24

	model, _ := m.updateTaskList(cfg.Keybinds.Delete)
	m = model.(Model)
	if m.mode != ModeDeletePrompt {
		t.Fatalf("delete entered mode %v, want confirmation", m.mode)
	}
	model, _ = m.updateDeletePrompt("esc")
	m = model.(Model)
	if len(store.Tasks) != 1 || m.mode != ModeTaskList {
		t.Fatal("cancelled deletion changed task state")
	}

	model, _ = m.updateTaskList(cfg.Keybinds.Delete)
	m = model.(Model)
	model, _ = m.updateDeletePrompt(tea.KeyPressMsg{Code: tea.KeyEnter}.String())
	_ = model
	if len(store.Tasks) != 0 {
		t.Fatal("confirmed deletion did not remove task")
	}
}

func TestClampViewRespectsBounds(t *testing.T) {
	got := clampView("123456789\nsecond\nthird", 5, 2)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	for _, line := range lines {
		if len([]rune(line)) > 4 {
			t.Fatalf("line exceeds width budget: %q", line)
		}
	}
}

func TestGroupCompletionCyclesSuggestionsAndOriginalQuery(t *testing.T) {
	m := New(config.Default(), &storage.Store{}, &session.Session{}, false)
	m.editActive = true
	m.editField = fieldGroup
	m.editInputs[fieldGroup].Focus()
	m.editInputs[fieldGroup].SetValue("d")
	m.groupSuggestions = []string{"delbysoft", "do"}
	m.resetGroupCompletion()

	tab := tea.KeyPressMsg{Code: tea.KeyTab}
	for _, want := range []string{"delbysoft", "do", "d"} {
		model, _ := m.updateEditTask(tab)
		m = model.(Model)
		if got := m.editInputs[fieldGroup].Value(); got != want {
			t.Fatalf("completion value = %q, want %q", got, want)
		}
	}
}

func TestStatusBarStaysWithinWidth(t *testing.T) {
	m := New(config.Default(), &storage.Store{}, &session.Session{}, false)
	m.width = 24
	bar := m.renderStatusBar([]string{"up/down navigate", "enter start", "q quit"})
	if lipgloss.Width(bar) > m.width {
		t.Fatalf("status bar width = %d, want <= %d", lipgloss.Width(bar), m.width)
	}
	if !strings.Contains(bar, "\x1b[") {
		t.Fatal("status bar did not render highlighted ANSI styles")
	}
}

func TestConfigReloadRebuildsRuntimeKeymap(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, &storage.Store{}, &session.Session{}, false)
	m.width = 80
	updated := config.Default()
	updated.Keybinds.Delete = "D"
	model, _ := m.Update(configReloadMsg{cfg: updated})
	m = model.(Model)
	if m.keys.delete != "D" {
		t.Fatalf("reloaded delete key = %q, want D", m.keys.delete)
	}
	if !strings.Contains(m.renderStatusBar([]string{m.keys.delete + " delete"}), "D") {
		t.Fatal("reloaded key was not reflected in rendered hints")
	}
}
