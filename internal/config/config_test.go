package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
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
	if !cfg.Display.ShowHints || !cfg.Display.ShowLogo {
		t.Error("expected hints and logo to be enabled by default")
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

func TestQuoteRoundTripsWindowsPaths(t *testing.T) {
	cases := []string{
		`C:\Users\x\AppData\Roaming\delbysoft\themes.toml`,
		`\\server\share\ticky-src`,
		`say "hi"`,
		"tab\there\nnewline\x01",
		`trailing\`,
	}
	for _, in := range cases {
		var out struct{ V string }
		if _, err := toml.Decode("V = "+quote(in), &out); err != nil {
			t.Fatalf("decode %q: %v", in, err)
		}
		if out.V != in {
			t.Fatalf("round trip: got %q want %q", out.V, in)
		}
	}
}

func TestMigratedTOMLWithWindowsPathsParses(t *testing.T) {
	cfg := Default()
	cfg.Themes.ThemeFile = `C:\Users\x\AppData\Roaming\delbysoft\themes.toml`
	cfg.Updates.RepoPath = `C:\Users\x\src\ticky`
	var got Config
	if _, err := toml.Decode(migratedTOML(cfg), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Themes.ThemeFile != cfg.Themes.ThemeFile || got.Updates.RepoPath != cfg.Updates.RepoPath {
		t.Fatalf("paths not preserved: %+v %+v", got.Themes, got.Updates)
	}
}

func TestRepairConfigFileFixesUnescapedPaths(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ticky.toml")
	broken := "[updates]\nrepo_path = \"C:\\Users\\x\\src\"   # source checkout\n\n" +
		"[themes]\ntheme_name = \"terminal\"\ntheme_file = \"C:\\Users\\x\\AppData\\Roaming\\delbysoft\\themes.toml\"   # shared\n"
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	var probe map[string]any
	if _, err := toml.DecodeFile(path, &probe); err == nil {
		t.Fatal("expected fixture to be invalid TOML")
	}
	repaired, ok := repairConfigFile(path)
	if !ok {
		t.Fatal("expected repair to succeed")
	}
	var cfg Config
	if _, err := toml.Decode(repaired, &cfg); err != nil {
		t.Fatalf("repaired content invalid: %v", err)
	}
	if cfg.Themes.ThemeFile != `C:\Users\x\AppData\Roaming\delbysoft\themes.toml` {
		t.Fatalf("theme_file = %q", cfg.Themes.ThemeFile)
	}
	if cfg.Updates.RepoPath != `C:\Users\x\src` {
		t.Fatalf("repo_path = %q", cfg.Updates.RepoPath)
	}
	if bak, err := os.ReadFile(path + ".bak"); err != nil || string(bak) != broken {
		t.Fatalf("backup missing or wrong: %v", err)
	}
}

func TestRepairBackslashesLeavesValidLinesAlone(t *testing.T) {
	in := "theme_file = \"C:\\\\ok\\\\path\"\n"
	if _, changed := repairBackslashes(in); changed {
		t.Fatal("valid escaped line should not change")
	}
}

func TestMigratedTOMLKeepsThemeOverridesAndWindowsShell(t *testing.T) {
	cfg := Default()
	cfg.Themes.Primary = "#112233"
	cfg.Updates.WindowsShell = "cmd"
	var got Config
	if _, err := toml.Decode(migratedTOML(cfg), &got); err != nil {
		t.Fatal(err)
	}
	if got.Themes.Primary != "#112233" {
		t.Fatalf("primary override lost: %q", got.Themes.Primary)
	}
	if got.Updates.WindowsShell != "cmd" {
		t.Fatalf("windows_shell = %q", got.Updates.WindowsShell)
	}
}

func TestNormalizeWindowsShell(t *testing.T) {
	for in, want := range map[string]string{"": "auto", "AUTO": "auto", "pwsh": "powershell", "Cmd": "cmd", "zsh": "auto"} {
		if got := NormalizeWindowsShell(in); got != want {
			t.Errorf("NormalizeWindowsShell(%q) = %q, want %q", in, got, want)
		}
	}
}
