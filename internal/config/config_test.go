package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Keybinds.Up != "up" {
		t.Errorf("expected up keybind 'up', got %q", cfg.Keybinds.Up)
	}
	if cfg.Keybinds.Close != "q" {
		t.Errorf("expected close keybind 'q', got %q", cfg.Keybinds.Close)
	}
	if cfg.Display.TimeFormat != "minutes" {
		t.Errorf("expected time_format 'minutes', got %q", cfg.Display.TimeFormat)
	}
	if cfg.Keybinds.ShowUpdates != "U" {
		t.Errorf("expected show_updates keybind 'U', got %q", cfg.Keybinds.ShowUpdates)
	}
}

func TestLoadCreatesDefaultFile(t *testing.T) {
	// Override UserConfigDir by writing into a temp dir.
	dir := t.TempDir()
	path := filepath.Join(dir, "ticky.toml")

	// Write default to temp path directly.
	if err := WriteDefault(path); err != nil {
		t.Fatalf("WriteDefault: %v", err)
	}

	_, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected config file to exist after WriteDefault: %v", err)
	}
}

func TestWriteDefaultContainsKeybinds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ticky.toml")

	if err := WriteDefault(path); err != nil {
		t.Fatalf("WriteDefault: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	content := string(data)
	// Check section headers are present.
	for _, key := range []string{"[keybinds]", "[display]", "[updates]", "time_format"} {
		if !strings.Contains(content, key) {
			t.Errorf("expected %q in default config output", key)
		}
	}
	// Check every keybind entry is present.
	for _, e := range keybindEntries {
		if !fileContainsKey(content, e.key) {
			t.Errorf("expected keybind %q in default config output", e.key)
		}
	}
	for _, key := range updateEntries {
		if !fileContainsKey(content, key) {
			t.Errorf("expected update key %q in default config output", key)
		}
	}
}

func TestApplyKeybindDefaults(t *testing.T) {
	// A config with some fields zeroed out should get defaults filled in.
	cfg := &Config{}
	applyKeybindDefaults(cfg)
	d := Default().Keybinds
	if cfg.Keybinds.Up != d.Up {
		t.Errorf("Up: got %q, want %q", cfg.Keybinds.Up, d.Up)
	}
	if cfg.Keybinds.Increase != d.Increase {
		t.Errorf("Increase: got %q, want %q", cfg.Keybinds.Increase, d.Increase)
	}
	if cfg.Keybinds.Completed != d.Completed {
		t.Errorf("Completed: got %q, want %q", cfg.Keybinds.Completed, d.Completed)
	}
	if cfg.Keybinds.ShowUpdates != d.ShowUpdates {
		t.Errorf("ShowUpdates: got %q, want %q", cfg.Keybinds.ShowUpdates, d.ShowUpdates)
	}
}

func TestFileContainsKey(t *testing.T) {
	content := `[keybinds]
up    = "up"
stop  = "x"
# this comment mentions stop but shouldn't count
`
	if !fileContainsKey(content, "up") {
		t.Error("expected to find 'up' key")
	}
	if !fileContainsKey(content, "stop") {
		t.Error("expected to find 'stop' key")
	}
	if fileContainsKey(content, "down") {
		t.Error("should not find 'down' key")
	}
}
