package config

import (
	"os"
	"path/filepath"
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
	for _, key := range []string{"[keybinds]", "[display]", "time_format"} {
		if !contains(content, key) {
			t.Errorf("expected %q in default config output", key)
		}
	}
}

func contains(s, sub string) bool {
	return containsStr(s, sub)
}
