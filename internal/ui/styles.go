package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Colour palette — all colours defined once and referenced by style vars.
var (
	colorPrimary  = lipgloss.Color("#7C9EF0") // soft blue
	colorAccent   = lipgloss.Color("#F0A47C") // warm orange
	colorGreen    = lipgloss.Color("#7CF09C") // success / complete
	colorRed      = lipgloss.Color("#F07C7C") // error / abandon
	colorMuted    = lipgloss.Color("#666688") // dim text
	colorSubtle   = lipgloss.Color("#444466") // very dim
	colorSelected = lipgloss.Color("#1E1E3A") // selected row bg
	colorBreak    = lipgloss.Color("#A47CF0") // break phase accent
	colorHeader   = lipgloss.Color("#EEEEFF") // bright header text
	colorWarning  = lipgloss.Color("#F0D07C") // warning / paused
	colorBrand    = lipgloss.Color("#5865F2")
)

var (
	BrandDelby = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
	BrandSoft  = lipgloss.NewStyle().Foreground(colorBrand).Bold(true)
)

// ─── Text styles ─────────────────────────────────────────────────────────────

// StyleHeader renders section headings.
var StyleHeader = lipgloss.NewStyle().
	Foreground(colorPrimary).
	Bold(true)

// StyleSubHeader renders secondary headings (group names, etc.).
var StyleSubHeader = lipgloss.NewStyle().
	Foreground(colorAccent).
	Bold(true)

// StyleMuted renders de-emphasised / hint text.
var StyleMuted = lipgloss.NewStyle().
	Foreground(colorMuted)

// StyleSubtle renders very dim decorative text.
var StyleSubtle = lipgloss.NewStyle().
	Foreground(colorSubtle)

// StyleError renders error messages.
var StyleError = lipgloss.NewStyle().
	Foreground(colorRed).
	Bold(true)

// StyleSuccess renders success / completion indicators.
var StyleSuccess = lipgloss.NewStyle().
	Foreground(colorGreen).
	Bold(true)

// StyleWarning renders warning / pause indicators.
var StyleWarning = lipgloss.NewStyle().
	Foreground(colorWarning).
	Bold(true)

// ─── List styles ─────────────────────────────────────────────────────────────

// StyleSelected renders the highlighted row in a list.
var StyleSelected = lipgloss.NewStyle().
	Background(colorSelected).
	Foreground(colorHeader).
	Bold(true)

// StyleCompleted renders a completed task name (dimmed + strikethrough-ish).
var StyleCompleted = lipgloss.NewStyle().
	Foreground(colorMuted)

// StyleAbandoned renders an abandoned task name.
var StyleAbandoned = lipgloss.NewStyle().
	Foreground(colorSubtle)

// StyleGroupName renders a group header row.
var StyleGroupName = lipgloss.NewStyle().
	Foreground(colorAccent).
	Bold(true)

// ─── Timer styles ────────────────────────────────────────────────────────────

// StyleTimerFocus renders the large focus countdown digits.
var StyleTimerFocus = lipgloss.NewStyle().
	Foreground(colorPrimary).
	Bold(true)

// StyleTimerBreak renders the large break countdown digits.
var StyleTimerBreak = lipgloss.NewStyle().
	Foreground(colorBreak).
	Bold(true)

// StyleTimerPaused renders the timer when paused.
var StyleTimerPaused = lipgloss.NewStyle().
	Foreground(colorWarning).
	Bold(true)

// StyleTimerLabel renders the "FOCUS" / "BREAK" phase label.
var StyleTimerLabel = lipgloss.NewStyle().
	Foreground(colorMuted).
	Bold(true)

// ─── Modal / overlay styles ───────────────────────────────────────────────────

// StyleBox renders a rounded-border modal box for prompts.
var StyleBox = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(colorAccent).
	Padding(1, 3).
	Margin(1, 0)

// StyleBreakBox renders the break-prompt modal with a different accent.
var StyleBreakBox = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(colorBreak).
	Padding(1, 3).
	Margin(1, 0)

// StyleErrorBox renders error modals.
var StyleErrorBox = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(colorRed).
	Padding(1, 3).
	Margin(1, 0)

// ─── Status bar ───────────────────────────────────────────────────────────────

// StyleStatusBar renders the bottom hint strip.
var StyleStatusBar = lipgloss.NewStyle().
	Foreground(colorMuted)

// StyleStatusKey renders a key hint label (e.g. "n").
var StyleStatusKey = lipgloss.NewStyle().
	Foreground(colorPrimary)

// ─── Report styles ────────────────────────────────────────────────────────────

// StyleReportHeader renders the report title.
var StyleReportHeader = lipgloss.NewStyle().
	Foreground(colorPrimary).
	Bold(true)

// StyleOverrun renders a positive delta (task overran).
var StyleOverrun = lipgloss.NewStyle().
	Foreground(colorRed)

// StyleOnTime renders a zero-or-negative delta (task on time).
var StyleOnTime = lipgloss.NewStyle().
	Foreground(colorGreen)

// StyleInterruptLabel renders the "Interrupts:" section heading in a report.
var StyleInterruptLabel = lipgloss.NewStyle().
	Foreground(colorWarning).
	Bold(true)

// ─── Progress bar ─────────────────────────────────────────────────────────────

// StyleProgressFull renders the filled portion of a progress bar.
var StyleProgressFull = lipgloss.NewStyle().
	Foreground(colorPrimary)

// StyleProgressEmpty renders the empty portion of a progress bar.
var StyleProgressEmpty = lipgloss.NewStyle().
	Foreground(colorSubtle)

// ─── Divider ─────────────────────────────────────────────────────────────────

// StyleDivider renders horizontal rule lines.
var StyleDivider = lipgloss.NewStyle().
	Foreground(colorSubtle)

// StyleSelector renders the active theme-picker row marker.
var StyleSelector = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FFFFFF")).
	Bold(true)

// ConfigureTheme applies a complete semantic palette. Terminal mode omits
// explicit colors so the terminal's normal foreground and background inherit.
func ConfigureTheme(colors map[string]string, terminal bool) {
	colorPrimary = themedColor(colors, terminal, "primary", "#7C9EF0")
	colorAccent = themedColor(colors, terminal, "accent", "#F0A47C")
	colorGreen = themedColor(colors, terminal, "success", "#7CF09C")
	colorRed = themedColor(colors, terminal, "error", "#F07C7C")
	colorMuted = themedColor(colors, terminal, "muted", "#666688")
	colorSubtle = themedColor(colors, terminal, "border", "#444466")
	colorSelected = themedColor(colors, terminal, "selected_background", "#1E1E3A")
	colorBreak = themedColor(colors, terminal, "accent", "#A47CF0")
	colorHeader = themedColor(colors, terminal, "selected_foreground", "#EEEEFF")
	colorWarning = themedColor(colors, terminal, "clipboard", "#F0D07C")
	colorBrand = themedColor(colors, terminal, "brand_secondary", "#5865F2")

	BrandDelby = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "brand_primary", "#FFFFFF")
	BrandSoft = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "brand_secondary", "#5865F2")
	StyleHeader = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "primary", "#7C9EF0")
	StyleSubHeader = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "accent", "#F0A47C")
	StyleMuted = themedStyle(lipgloss.NewStyle(), colors, terminal, "muted", "#666688")
	StyleSubtle = themedStyle(lipgloss.NewStyle(), colors, terminal, "border", "#444466")
	StyleError = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "error", "#F07C7C")
	StyleSuccess = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "success", "#7CF09C")
	StyleWarning = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "clipboard", "#F0D07C")
	StyleSelected = themedBackground(themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "selected_foreground", "#EEEEFF"), colors, terminal, "selected_background", "#1E1E3A")
	StyleCompleted = themedStyle(lipgloss.NewStyle(), colors, terminal, "muted", "#666688")
	StyleAbandoned = themedStyle(lipgloss.NewStyle(), colors, terminal, "border", "#444466")
	StyleGroupName = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "accent", "#F0A47C")
	StyleTimerFocus = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "primary", "#7C9EF0")
	StyleTimerBreak = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "accent", "#A47CF0")
	StyleTimerPaused = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "clipboard", "#F0D07C")
	StyleTimerLabel = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "muted", "#666688")
	StyleBox = themedBorder(lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 3).Margin(1, 0), colors, terminal, "accent", "#F0A47C")
	StyleBreakBox = themedBorder(lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 3).Margin(1, 0), colors, terminal, "accent", "#A47CF0")
	StyleErrorBox = themedBorder(lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 3).Margin(1, 0), colors, terminal, "error", "#F07C7C")
	StyleStatusBar = themedStyle(lipgloss.NewStyle(), colors, terminal, "muted", "#666688")
	StyleStatusKey = themedStyle(lipgloss.NewStyle(), colors, terminal, "primary", "#7C9EF0")
	StyleReportHeader = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "primary", "#7C9EF0")
	StyleOverrun = themedStyle(lipgloss.NewStyle(), colors, terminal, "error", "#F07C7C")
	StyleOnTime = themedStyle(lipgloss.NewStyle(), colors, terminal, "success", "#7CF09C")
	StyleInterruptLabel = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "clipboard", "#F0D07C")
	StyleProgressFull = themedStyle(lipgloss.NewStyle(), colors, terminal, "primary", "#7C9EF0")
	StyleProgressEmpty = themedStyle(lipgloss.NewStyle(), colors, terminal, "border", "#444466")
	StyleDivider = themedStyle(lipgloss.NewStyle(), colors, terminal, "border", "#444466")
	StyleSelector = themedStyle(lipgloss.NewStyle().Bold(true), colors, terminal, "selector", "#FFFFFF")
}

func themedColor(colors map[string]string, terminal bool, key, fallback string) color.Color {
	if value, ok := themedValue(colors, terminal, key, fallback); ok {
		return lipgloss.Color(value)
	}
	return nil
}

func themedStyle(style lipgloss.Style, colors map[string]string, terminal bool, key, fallback string) lipgloss.Style {
	if value, ok := themedValue(colors, terminal, key, fallback); ok {
		return style.Foreground(lipgloss.Color(value))
	}
	return style
}

func themedBackground(style lipgloss.Style, colors map[string]string, terminal bool, key, fallback string) lipgloss.Style {
	if value, ok := themedValue(colors, terminal, key, fallback); ok {
		return style.Background(lipgloss.Color(value))
	}
	return style
}

func themedBorder(style lipgloss.Style, colors map[string]string, terminal bool, key, fallback string) lipgloss.Style {
	if value, ok := themedValue(colors, terminal, key, fallback); ok {
		return style.BorderForeground(lipgloss.Color(value))
	}
	return style
}

func themedValue(colors map[string]string, terminal bool, key, fallback string) (string, bool) {
	if value := colors[key]; value != "" {
		return value, true
	}
	if terminal {
		return "", false
	}
	return fallback, fallback != ""
}
