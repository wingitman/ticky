package config

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/BurntSushi/toml"
)

// Keybinds holds all configurable key bindings.
type Keybinds struct {
	Up        string `toml:"up"`
	Down      string `toml:"down"`
	Edit      string `toml:"edit"`
	Confirm   string `toml:"confirm"`
	Start     string `toml:"start"`
	Close     string `toml:"close"`
	Format    string `toml:"format"`
	Options   string `toml:"options"`
	Pause     string `toml:"pause"`
	Stop      string `toml:"stop"`
	New       string `toml:"new"`
	Delete    string `toml:"delete"`
	Group     string `toml:"group"`
	Report    string `toml:"report"`
	Completed string `toml:"completed"`
}

// Display holds display preferences.
type Display struct {
	// TimeFormat controls how durations are shown.
	// Valid values: minutes | seconds | hhmm | tshirt | points
	TimeFormat string `toml:"time_format"`

	// ShowTaskName renders the active task name in a terminal corner.
	// Only has effect when a task timer is running.
	ShowTaskName bool `toml:"show_task_name"`

	// ShowTimeLeft renders the remaining timer time in a terminal corner.
	// Only has effect when a task timer is running.
	ShowTimeLeft bool `toml:"show_time_left"`

	// OverlayCorner controls which corner the status overlay appears in.
	// Valid values: top-left | top-right | bottom-left | bottom-right
	OverlayCorner string `toml:"overlay_corner"`
}

// Config is the root configuration structure.
type Config struct {
	Keybinds Keybinds `toml:"keybinds"`
	Display  Display  `toml:"display"`
}

// Default returns a Config populated with sensible defaults.
func Default() *Config {
	return &Config{
		Keybinds: Keybinds{
			Up:        "up",
			Down:      "down",
			Edit:      "e",
			Confirm:   "enter",
			Start:     "enter",
			Close:     "q",
			Format:    "f",
			Options:   "o",
			Pause:     "p",
			Stop:      "x",
			New:       "n",
			Delete:    "d",
			Group:     "g",
			Report:    "r",
			Completed: "h",
		},
		Display: Display{
			TimeFormat:    "minutes",
			ShowTaskName:  false,
			ShowTimeLeft:  false,
			OverlayCorner: "top-right",
		},
	}
}

// ConfigDir returns the platform-appropriate config directory for ticky.
// Linux:   ~/.config/delbysoft
// macOS:   ~/Library/Application Support/delbysoft
// Windows: %AppData%\Roaming\delbysoft
func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "delbysoft"), nil
}

// ConfigPath returns the full path to ticky.toml.
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ticky.toml"), nil
}

// Load reads the config file, creating it with defaults if it does not exist.
// Config load errors are non-fatal: defaults are returned alongside the error
// so the caller can choose to warn and continue.
func Load() (*Config, error) {
	cfg := Default()

	path, err := ConfigPath()
	if err != nil {
		return cfg, err
	}

	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		// First launch — create the config directory and write defaults.
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return cfg, err
		}
		if err := WriteDefault(path); err != nil {
			return cfg, err
		}
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}

	// File exists — decode into cfg (unknown fields are silently ignored).
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return cfg, err
	}

	// Validate / clamp values.
	if cfg.Display.TimeFormat == "" {
		cfg.Display.TimeFormat = "minutes"
	}
	if cfg.Display.OverlayCorner == "" {
		cfg.Display.OverlayCorner = "top-right"
	}

	// Migration: if the file is missing new display keys (added after initial
	// release), rewrite it so the user can see and edit them. Keybind
	// customisations are preserved because cfg was decoded from the file first.
	if needsMigration(path) {
		_ = writeMigrated(path, cfg) // non-fatal
	}

	return cfg, nil
}

// needsMigration returns true if the config file is missing any keys added
// after the initial release.
func needsMigration(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	content := string(data)
	return !containsStr(content, "show_task_name") ||
		!containsStr(content, "completed") ||
		!containsStr(content, "stop")
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// writeMigrated rewrites the config file with all current keys and comments,
// preserving the user's existing values by encoding the already-loaded cfg.
func writeMigrated(path string, cfg *Config) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(migratedTOML(cfg))
	return err
}

// migratedTOML produces a commented TOML string with the user's values baked in.
func migratedTOML(cfg *Config) string {
	k := cfg.Keybinds
	d := cfg.Display
	return "# ticky configuration file\n" +
		"# Edit keybinds and display preferences below.\n" +
		"# Restart ticky for changes to take effect.\n\n" +
		"[keybinds]\n" +
		"up        = " + quote(k.Up) + "      # move selection up\n" +
		"down      = " + quote(k.Down) + "    # move selection down\n" +
		"edit      = " + quote(k.Edit) + "       # edit selected task\n" +
		"confirm   = " + quote(k.Confirm) + "   # save / confirm\n" +
		"start     = " + quote(k.Start) + "   # start selected task timer\n" +
		"close     = " + quote(k.Close) + "        # quit ticky\n" +
		"format    = " + quote(k.Format) + "        # cycle time format\n" +
		"options   = " + quote(k.Options) + "        # open this config file in your editor\n" +
		"pause     = " + quote(k.Pause) + "        # pause the running timer\n" +
		"stop      = " + quote(k.Stop) + "        # stop the running timer and reset the task\n" +
		"new       = " + quote(k.New) + "        # create a new task\n" +
		"delete    = " + quote(k.Delete) + "        # delete selected task\n" +
		"group     = " + quote(k.Group) + "        # open group list\n" +
		"report    = " + quote(k.Report) + "        # open report view\n" +
		"completed = " + quote(k.Completed) + "        # view completed tasks\n\n" +
		"[display]\n" +
		"# How task durations are displayed.\n" +
		"# Options: minutes | seconds | hhmm | tshirt | points\n" +
		"time_format = " + quote(d.TimeFormat) + "\n\n" +
		"# Show the active task name in a terminal corner when a timer is running.\n" +
		"# Use 'ticky --status' in your shell prompt to display this outside ticky.\n" +
		"show_task_name = " + boolStr(d.ShowTaskName) + "\n\n" +
		"# Show the remaining timer time in a terminal corner when a timer is running.\n" +
		"show_time_left = " + boolStr(d.ShowTimeLeft) + "\n\n" +
		"# Which corner to render the status overlay in (inside the ticky TUI).\n" +
		"# Options: top-left | top-right | bottom-left | bottom-right\n" +
		"overlay_corner = " + quote(d.OverlayCorner) + "\n"
}

func quote(s string) string { return `"` + s + `"` }
func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// WriteDefault writes a fully-commented default config to path.
func WriteDefault(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(defaultTOML())
	return err
}

// ResolveEditor returns the best editor for the current environment.
// Exported so main.go and model.go can use it with tea.ExecProcess.
func ResolveEditor() string {
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	if e := os.Getenv("VISUAL"); e != "" {
		return e
	}
	switch runtime.GOOS {
	case "windows":
		return "notepad"
	case "darwin":
		return "nano"
	default:
		return "nano"
	}
}

func defaultTOML() string {
	return `# ticky configuration file
# Edit keybinds and display preferences below.
# Restart ticky for changes to take effect.

[keybinds]
up        = "up"      # move selection up
down      = "down"    # move selection down
edit      = "e"       # edit selected task
confirm   = "enter"   # save / confirm
start     = "enter"   # start selected task timer
close     = "q"       # quit ticky
format    = "f"       # cycle time format
options   = "o"       # open this config file in your editor
pause     = "p"       # pause the running timer
stop      = "x"       # stop the running timer and reset the task
new       = "n"       # create a new task
delete    = "d"       # delete selected task
group     = "g"       # open group list
report    = "r"       # open report view
completed = "h"       # view completed tasks

[display]
# How task durations are displayed.
# Options: minutes | seconds | hhmm | tshirt | points
time_format = "minutes"

# Show the active task name in a terminal corner when a timer is running.
# Use 'ticky --status' in your shell prompt to display this outside ticky.
show_task_name = false

# Show the remaining timer time in a terminal corner when a timer is running.
show_time_left = false

# Which corner to render the status overlay in (inside the ticky TUI).
# Options: top-left | top-right | bottom-left | bottom-right
overlay_corner = "top-right"
`
}
