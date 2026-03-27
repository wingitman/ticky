package ui

import "github.com/charmbracelet/lipgloss"

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
