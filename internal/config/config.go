package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
)

// keybindEntries is the single authoritative list of every keybind TOML key
// name paired with its comment. Order here controls the order written to the
// config file. When adding a new keybind, add it here — migration and default-
// filling are derived automatically from this list and from Default().
var keybindEntries = []struct{ key, comment string }{
	{"up", "move selection up"},
	{"down", "move selection down"},
	{"increase", "increase value (timer +1m, adjust numeric fields)"},
	{"decrease", "decrease value (timer -1m, adjust numeric fields)"},
	{"edit", "edit selected task"},
	{"confirm", "save / confirm"},
	{"start", "start selected task timer"},
	{"close", "quit ticky"},
	{"format", "cycle time format"},
	{"options", "open this config file in your editor"},
	{"pause", "pause the running timer"},
	{"stop", "stop the running timer and reset the task"},
	{"new", "create a new task"},
	{"delete", "delete selected task"},
	{"group", "open group list"},
	{"report", "open report view"},
	{"completed", "view completed tasks"},
	{"show_updates", "show update history and installers"},
}

// displayEntries is the authoritative list of every display TOML key used for
// migration checks (values are written from the Config struct directly).
var displayEntries = []string{
	"time_format",
	"show_task_name",
	"show_time_left",
	"overlay_corner",
	"break_prompt_debounce",
	"show_completion_animation",
	"auto_start_break",
}

// updateEntries is the authoritative list of every [updates] TOML key.
var updateEntries = []string{
	"disable_checks",
	"current_commit",
	"repo_path",
	"terminal",
}

// Keybinds holds all configurable key bindings.
type Keybinds struct {
	Up          string `toml:"up"`
	Down        string `toml:"down"`
	Edit        string `toml:"edit"`
	Confirm     string `toml:"confirm"`
	Start       string `toml:"start"`
	Close       string `toml:"close"`
	Format      string `toml:"format"`
	Options     string `toml:"options"`
	Pause       string `toml:"pause"`
	Stop        string `toml:"stop"`
	New         string `toml:"new"`
	Delete      string `toml:"delete"`
	Group       string `toml:"group"`
	Report      string `toml:"report"`
	Completed   string `toml:"completed"`
	ShowUpdates string `toml:"show_updates"`
	Increase    string `toml:"increase"`
	Decrease    string `toml:"decrease"`
}

// Updates holds update-check and installer preferences.
type Updates struct {
	DisableChecks bool   `toml:"disable_checks"`
	CurrentCommit string `toml:"current_commit"`
	RepoPath      string `toml:"repo_path"`
	Terminal      string `toml:"terminal"`
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

	// BreakPromptDebounce is the number of seconds after the break prompt
	// appears before it accepts keystrokes. Prevents accidental input if the
	// timer fires while the user is mid-typing. 0 disables the debounce.
	BreakPromptDebounce int `toml:"break_prompt_debounce"`

	// ShowCompletionAnimation plays a brief ASCII confetti animation when a
	// task is marked as complete.
	ShowCompletionAnimation bool `toml:"show_completion_animation"`

	// AutoStartBreak automatically begins the break timer when a focus
	// session ends, after the break prompt debounce period elapses.
	// The break prompt is still shown briefly; if the user presses a key
	// during the debounce window, auto-start is cancelled.
	AutoStartBreak bool `toml:"auto_start_break"`
}

// Config is the root configuration structure.
type Config struct {
	Keybinds Keybinds `toml:"keybinds"`
	Display  Display  `toml:"display"`
	Updates  Updates  `toml:"updates"`
}

// Default returns a Config populated with sensible defaults.
func Default() *Config {
	return &Config{
		Keybinds: Keybinds{
			Up:          "up",
			Down:        "down",
			Edit:        "e",
			Confirm:     "enter",
			Start:       "enter",
			Close:       "q",
			Format:      "f",
			Options:     "o",
			Pause:       "p",
			Stop:        "x",
			New:         "n",
			Delete:      "d",
			Group:       "g",
			Report:      "r",
			Completed:   "h",
			ShowUpdates: "U",
			Increase:    "right",
			Decrease:    "left",
		},
		Display: Display{
			TimeFormat:              "minutes",
			ShowTaskName:            false,
			ShowTimeLeft:            false,
			OverlayCorner:           "top-right",
			BreakPromptDebounce:     2,
			ShowCompletionAnimation: true,
			AutoStartBreak:          false,
		},
		Updates: Updates{
			DisableChecks: false,
			CurrentCommit: "",
			RepoPath:      "",
			Terminal:      "",
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

	// Fill any keybind fields that were absent in the file with their defaults.
	// This is the primary safety net: even if migration fails, the app runs.
	applyKeybindDefaults(cfg)

	// Validate / clamp display values.
	if cfg.Display.TimeFormat == "" {
		cfg.Display.TimeFormat = "minutes"
	}
	if cfg.Display.OverlayCorner == "" {
		cfg.Display.OverlayCorner = "top-right"
	}
	if cfg.Display.BreakPromptDebounce < 0 {
		cfg.Display.BreakPromptDebounce = 0
	}

	// Migration: if the file is missing any known key, rewrite it in full so
	// the user can see and edit the new entries. User values are preserved
	// because cfg was decoded (and defaults applied) before this point.
	if needsMigration(path) {
		_ = writeMigrated(path, cfg) // non-fatal
	}

	return cfg, nil
}

// applyKeybindDefaults fills every empty keybind field with its default value.
// TOML leaves fields absent from the file as zero-value (""), so this ensures
// the app always has a working binding regardless of what the file contains.
func applyKeybindDefaults(cfg *Config) {
	d := Default().Keybinds
	if cfg.Keybinds.Up == "" {
		cfg.Keybinds.Up = d.Up
	}
	if cfg.Keybinds.Down == "" {
		cfg.Keybinds.Down = d.Down
	}
	if cfg.Keybinds.Increase == "" {
		cfg.Keybinds.Increase = d.Increase
	}
	if cfg.Keybinds.Decrease == "" {
		cfg.Keybinds.Decrease = d.Decrease
	}
	if cfg.Keybinds.Edit == "" {
		cfg.Keybinds.Edit = d.Edit
	}
	if cfg.Keybinds.Confirm == "" {
		cfg.Keybinds.Confirm = d.Confirm
	}
	if cfg.Keybinds.Start == "" {
		cfg.Keybinds.Start = d.Start
	}
	if cfg.Keybinds.Close == "" {
		cfg.Keybinds.Close = d.Close
	}
	if cfg.Keybinds.Format == "" {
		cfg.Keybinds.Format = d.Format
	}
	if cfg.Keybinds.Options == "" {
		cfg.Keybinds.Options = d.Options
	}
	if cfg.Keybinds.Pause == "" {
		cfg.Keybinds.Pause = d.Pause
	}
	if cfg.Keybinds.Stop == "" {
		cfg.Keybinds.Stop = d.Stop
	}
	if cfg.Keybinds.New == "" {
		cfg.Keybinds.New = d.New
	}
	if cfg.Keybinds.Delete == "" {
		cfg.Keybinds.Delete = d.Delete
	}
	if cfg.Keybinds.Group == "" {
		cfg.Keybinds.Group = d.Group
	}
	if cfg.Keybinds.Report == "" {
		cfg.Keybinds.Report = d.Report
	}
	if cfg.Keybinds.Completed == "" {
		cfg.Keybinds.Completed = d.Completed
	}
	if cfg.Keybinds.ShowUpdates == "" {
		cfg.Keybinds.ShowUpdates = d.ShowUpdates
	}
}

// needsMigration returns true if the config file is missing any known keybind
// or display key. Derived from keybindEntries and displayEntries so it stays
// in sync automatically when new keys are added to those lists.
func needsMigration(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	content := string(data)
	for _, e := range keybindEntries {
		if !fileContainsKey(content, e.key) {
			return true
		}
	}
	for _, key := range displayEntries {
		if !fileContainsKey(content, key) {
			return true
		}
	}
	for _, key := range updateEntries {
		if !fileContainsKey(content, key) {
			return true
		}
	}
	return false
}

// fileContainsKey returns true when the TOML file content contains a line
// where the given key appears as an actual assignment (key = ...), not just
// as part of a comment or a value string. This prevents false positives from
// keys whose names appear inside other values or comments.
func fileContainsKey(content, key string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		// Skip comment lines.
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Match "key = " or "key=" at the start of the trimmed line.
		if strings.HasPrefix(trimmed, key+"=") || strings.HasPrefix(trimmed, key+" ") {
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

// migratedTOML produces a fully-commented TOML string with the user's values
// baked in. The keybind section is derived from keybindEntries so it stays in
// sync with the authoritative list automatically.
func migratedTOML(cfg *Config) string {
	d := cfg.Display
	u := cfg.Updates
	out := "# ticky configuration file\n" +
		"# Edit keybinds and display preferences below.\n" +
		"# Restart ticky for changes to take effect.\n\n" +
		"[keybinds]\n"
	out += keybindsTOML(&cfg.Keybinds)
	out += "\n[display]\n" +
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
		"overlay_corner = " + quote(d.OverlayCorner) + "\n\n" +
		"# Seconds to wait after the break prompt appears before accepting keystrokes.\n" +
		"# Prevents accidentally triggering an action if the timer fires mid-typing.\n" +
		"# Set to 0 to disable.\n" +
		"break_prompt_debounce = " + itoa(d.BreakPromptDebounce) + "\n\n" +
		"# Play a brief ASCII confetti animation when a task is marked complete.\n" +
		"show_completion_animation = " + boolStr(d.ShowCompletionAnimation) + "\n\n" +
		"# Automatically start the break timer when a focus session ends.\n" +
		"# The break prompt is shown during the debounce window; pressing any key cancels.\n" +
		"auto_start_break = " + boolStr(d.AutoStartBreak) + "\n\n" +
		"[updates]\n" +
		"disable_checks = " + boolStr(u.DisableChecks) + "   # true disables startup update checks\n" +
		"current_commit = " + quote(u.CurrentCommit) + "   # installed app commit, maintained by ticky\n" +
		"repo_path = " + quote(u.RepoPath) + "   # source checkout used for updates\n" +
		"terminal = " + quote(u.Terminal) + "   # optional terminal command for detached updates\n"
	return out
}

// keybindsTOML renders the [keybinds] section body from keybindEntries and the
// provided Keybinds values. Column-aligns the values for readability.
func keybindsTOML(k *Keybinds) string {
	// Build a map from TOML key name to current value.
	vals := keybindValues(k)

	// Find the longest key name for alignment.
	maxLen := 0
	for _, e := range keybindEntries {
		if len(e.key) > maxLen {
			maxLen = len(e.key)
		}
	}

	var out string
	for _, e := range keybindEntries {
		val := vals[e.key]
		pad := strings.Repeat(" ", maxLen-len(e.key))
		out += e.key + pad + " = " + quote(val) + "  # " + e.comment + "\n"
	}
	return out
}

// keybindValues returns a map of TOML key name → current value for all keybinds.
// This is the one place that maps struct fields to their TOML names for writing.
func keybindValues(k *Keybinds) map[string]string {
	return map[string]string{
		"up":           k.Up,
		"down":         k.Down,
		"increase":     k.Increase,
		"decrease":     k.Decrease,
		"edit":         k.Edit,
		"confirm":      k.Confirm,
		"start":        k.Start,
		"close":        k.Close,
		"format":       k.Format,
		"options":      k.Options,
		"pause":        k.Pause,
		"stop":         k.Stop,
		"new":          k.New,
		"delete":       k.Delete,
		"group":        k.Group,
		"report":       k.Report,
		"completed":    k.Completed,
		"show_updates": k.ShowUpdates,
	}
}

// RecordUpdateMetadata stores the installed commit and source repo path without
// changing user-facing preferences.
func RecordUpdateMetadata(commit, repoPath string) error {
	cfg, err := Load()
	if err != nil {
		cfg = Default()
	}
	if commit != "" {
		cfg.Updates.CurrentCommit = commit
	}
	if repoPath != "" {
		cfg.Updates.RepoPath = repoPath
	}
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return writeMigrated(path, cfg)
}

func quote(s string) string { return `"` + s + `"` }
func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		return "-" + string(buf[pos:])
	}
	return string(buf[pos:])
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

// defaultTOML generates the default config file content from Default() and
// keybindEntries, so new keybinds appear automatically in fresh installs.
func defaultTOML() string {
	cfg := Default()
	return migratedTOML(cfg)
}
