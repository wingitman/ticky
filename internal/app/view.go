package app

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/wingitman/ticky/internal/report"
	"github.com/wingitman/ticky/internal/storage"
	"github.com/wingitman/ticky/internal/timer"
	"github.com/wingitman/ticky/internal/ui"
)

// ─── View entry point ─────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.width == 0 {
		return "Loading…"
	}

	switch m.mode {
	case ModeTimerFocus, ModeTimerBreak:
		return m.renderTimerScreen()
	case ModePausePrompt:
		return m.renderPausePrompt()
	case ModeBreakPrompt:
		return m.renderBreakPrompt()
	case ModeEditTask:
		return m.renderEditTask()
	case ModeGroupList:
		return m.renderGroupList()
	case ModeTaskActions:
		return m.renderTaskActions()
	case ModeCompletion:
		return m.renderCompletion()
	case ModeReport:
		return m.renderReport()
	case ModeCompleted:
		return m.renderCompleted()
	case ModeError:
		return m.renderError()
	default:
		return m.renderTaskList()
	}
}

// ─── Corner overlay ───────────────────────────────────────────────────────────

func (m Model) renderCornerOverlay() string {
	widget := m.OverlayWidget()
	if widget == "" || m.width == 0 || m.height == 0 {
		return ""
	}

	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7C9EF0")).
		Background(lipgloss.Color("#1a1a2e")).
		Padding(0, 1)

	rendered := style.Render(widget)
	rw := lipgloss.Width(rendered)
	rh := strings.Count(rendered, "\n") + 1

	corner := m.cfg.Display.OverlayCorner

	var row, col int
	switch corner {
	case "top-left":
		row, col = 1, 1
	case "top-right":
		row, col = 1, m.width-rw+1
	case "bottom-left":
		row, col = m.height-rh, 1
	default: // "bottom-right"
		row, col = m.height-rh, m.width-rw+1
	}
	if col < 1 {
		col = 1
	}
	if row < 1 {
		row = 1
	}

	return "\033[s" +
		"\033[" + itoa(row) + ";" + itoa(col) + "H" +
		rendered +
		"\033[u"
}

// ─── Task List ────────────────────────────────────────────────────────────────

func (m Model) renderTaskList() string {
	var b strings.Builder

	b.WriteString(m.renderBanner("TICKY", ui.StyleHeader))
	b.WriteString("\n")

	tasks := storage.ActiveTasks(m.store)

	if len(tasks) == 0 {
		b.WriteString(ui.StyleMuted.Render("  No tasks yet. Press ") +
			ui.StyleStatusKey.Render(m.keys.newTask) +
			ui.StyleMuted.Render(" to create one."))
		b.WriteString("\n")
	} else {
		vis := m.visibleTaskRows()
		end := m.offset + vis
		if end > len(tasks) {
			end = len(tasks)
		}

		if m.offset > 0 {
			b.WriteString(ui.StyleMuted.Render("  ↑ " + itoa(m.offset) + " more above"))
			b.WriteString("\n")
		}

		lastGroup := "__unset__"
		for i := m.offset; i < end; i++ {
			t := tasks[i]
			gName := m.GroupName(t.GroupID)
			if gName != lastGroup {
				lastGroup = gName
				if gName == "" {
					b.WriteString(ui.StyleSubtle.Render("  ── ungrouped ──"))
				} else {
					total := storage.TotalFocusMinutes(m.store, t.GroupID)
					b.WriteString(ui.StyleGroupName.Render("  "+gName) +
						ui.StyleMuted.Render("  "+m.FormatDuration(total)))
				}
				b.WriteString("\n")
			}
			b.WriteString(m.renderTaskRow(t, i == m.cursor))
			b.WriteString("\n")
		}

		below := len(tasks) - end
		if below > 0 {
			b.WriteString(ui.StyleMuted.Render("  ↓ " + itoa(below) + " more below"))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if m.statusMsg != "" {
		b.WriteString(ui.StyleWarning.Render("  " + m.statusMsg))
		b.WriteString("\n")
	}

	// Show active task status bar when a timer is running or paused.
	if m.activeTaskIdx >= 0 && m.activeTaskIdx < len(m.store.Tasks) {
		t := m.store.Tasks[m.activeTaskIdx]
		rem := m.tmr.HHMMString()
		if m.tmr.State == timer.StatePaused {
			b.WriteString(ui.StyleWarning.Render("  ⏸ " + t.Name + "  " + rem + " remaining — enter to resume · e for actions · x to stop"))
		} else {
			b.WriteString(ui.StyleSuccess.Render("  ▶ " + t.Name + "  " + rem + " remaining — enter to view timer · e for actions · x to stop"))
		}
		b.WriteString("\n")
	}

	b.WriteString(m.renderStatusBar([]string{
		m.keys.newTask + " new",
		m.keys.edit + " edit/actions",
		m.keys.delete + " delete",
		m.keys.start + " start",
		m.keys.group + " groups",
		m.keys.report + " report",
		m.keys.completed + " completed",
		m.keys.format + " fmt:" + timeFormats[m.timeFormatIdx],
		m.keys.options + " config",
		m.keys.close + " quit",
	}))

	b.WriteString(m.renderCornerOverlay())
	return b.String()
}

func (m Model) renderTaskRow(t storage.Task, selected bool) string {
	focusStr := m.FormatDuration(t.FocusTime)
	breakStr := m.FormatDuration(t.BreakTime)

	activeMarker := ""
	if m.activeTaskIdx >= 0 && m.store.Tasks[m.activeTaskIdx].ID == t.ID {
		if m.tmr.State == timer.StatePaused {
			activeMarker = ui.StyleWarning.Render(" ⏸")
		} else {
			activeMarker = ui.StyleSuccess.Render(" ▶")
		}
	}

	interrupts := ""
	if len(t.Interrupts) > 0 {
		interrupts = ui.StyleWarning.Render(" ⚡" + itoa(len(t.Interrupts)))
	}

	line := "  " + t.Name +
		activeMarker +
		ui.StyleMuted.Render("  "+focusStr+" / "+breakStr) +
		interrupts

	if selected {
		lineWidth := lipgloss.Width(line)
		if lineWidth < m.width {
			line = line + strings.Repeat(" ", m.width-lineWidth)
		}
		return ui.StyleSelected.Render(line)
	}
	return line
}

// ─── Task Actions Sub-menu ────────────────────────────────────────────────────

func (m Model) renderTaskActions() string {
	var b strings.Builder

	taskName := ""
	if m.activeTaskIdx >= 0 && m.activeTaskIdx < len(m.store.Tasks) {
		taskName = m.store.Tasks[m.activeTaskIdx].Name
	}

	var content string
	switch m.actionsConfirm {
	case confirmComplete:
		content = ui.StyleHeader.Render("TASK ACTIONS") + "\n\n" +
			ui.StyleMuted.Render(taskName) + "\n\n" +
			ui.StyleWarning.Render("Mark task as complete?") + "\n\n" +
			ui.StyleStatusKey.Render("["+m.keys.confirm+"]") + " Yes, complete it\n" +
			ui.StyleStatusKey.Render("["+m.keys.close+"]") + " No, go back"

	case confirmAbandon:
		content = ui.StyleHeader.Render("TASK ACTIONS") + "\n\n" +
			ui.StyleMuted.Render(taskName) + "\n\n" +
			ui.StyleWarning.Render("Abandon this task?") + "\n\n" +
			ui.StyleStatusKey.Render("["+m.keys.confirm+"]") + " Yes, abandon it\n" +
			ui.StyleStatusKey.Render("["+m.keys.close+"]") + " No, go back"

	default:
		pauseOrResume := "Pause timer"
		if m.tmr.State == timer.StatePaused {
			pauseOrResume = "Resume timer"
		}
		opts := []string{pauseOrResume, "Stop & reset", "Complete task", "Abandon task"}
		var menu string
		for i, opt := range opts {
			if i == m.actionsCursor {
				menu += ui.StyleStatusKey.Render("> ") + opt + "\n"
			} else {
				menu += ui.StyleMuted.Render("  ") + opt + "\n"
			}
		}
		content = ui.StyleHeader.Render("TASK ACTIONS") + "\n\n" +
			ui.StyleMuted.Render(taskName) + "\n\n" +
			menu + "\n" +
			ui.StyleMuted.Render(m.keys.up+"/"+m.keys.down+" select  ·  "+m.keys.confirm+" confirm  ·  "+m.keys.close+" back")
	}

	box := ui.StyleBreakBox.Width(m.width - 8).Render(content)
	b.WriteString("\n")
	b.WriteString(centerStr(box, m.width))
	b.WriteString("\n")
	return b.String()
}

// ─── Timer Screen ─────────────────────────────────────────────────────────────

func (m Model) renderTimerScreen() string {
	var b strings.Builder

	taskName := "Unnamed Task"
	if m.activeTaskIdx >= 0 && m.activeTaskIdx < len(m.store.Tasks) {
		taskName = m.store.Tasks[m.activeTaskIdx].Name
	}

	phaseLabel := "FOCUS"
	timeStyle := ui.StyleTimerFocus
	if m.tmr.Phase == timer.PhaseBreak {
		phaseLabel = "BREAK"
		timeStyle = ui.StyleTimerBreak
	}
	if m.tmr.State == timer.StatePaused {
		timeStyle = ui.StyleTimerPaused
		phaseLabel = "PAUSED"
	}

	b.WriteString("\n")
	b.WriteString(centerStr(ui.StyleTimerLabel.Render(phaseLabel), m.width))
	b.WriteString("\n\n")
	b.WriteString(centerStr(timeStyle.Render(bigTime(m.tmr.HHMMString())), m.width))
	b.WriteString("\n\n")
	b.WriteString(centerStr(ui.StyleMuted.Render(taskName), m.width))
	b.WriteString("\n\n")
	b.WriteString(centerStr(m.renderProgressBar(m.tmr.Progress(), m.width/2), m.width))
	b.WriteString("\n\n")

	if m.activeTaskIdx >= 0 && m.activeTaskIdx < len(m.store.Tasks) {
		n := len(m.store.Tasks[m.activeTaskIdx].Interrupts)
		if n > 0 {
			b.WriteString(centerStr(ui.StyleWarning.Render("⚡ "+itoa(n)+" interrupt(s)"), m.width))
			b.WriteString("\n\n")
		}
	}

	b.WriteString(m.renderStatusBar([]string{
		m.keys.pause + " pause",
		m.keys.stop + " stop",
		m.keys.close + " back to list",
	}))

	b.WriteString(m.renderCornerOverlay())
	return b.String()
}

func bigTime(s string) string {
	var sb strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		sb.WriteRune(r)
		if i < len(runes)-1 {
			sb.WriteString("  ")
		}
	}
	return sb.String()
}

// ─── Pause Prompt ─────────────────────────────────────────────────────────────

func (m Model) renderPausePrompt() string {
	var b strings.Builder

	taskName := ""
	if m.activeTaskIdx >= 0 && m.activeTaskIdx < len(m.store.Tasks) {
		taskName = m.store.Tasks[m.activeTaskIdx].Name
	}

	content := ui.StyleWarning.Render("TIMER PAUSED") + "\n\n" +
		ui.StyleMuted.Render(taskName) + "\n\n" +
		"Why are you pausing?\n" +
		m.pauseInput.View() + "\n\n" +
		ui.StyleMuted.Render(m.keys.confirm) + " resume  " +
		ui.StyleMuted.Render(m.keys.close) + " resume without recording"

	box := ui.StyleBox.Width(m.width - 8).Render(content)
	b.WriteString("\n")
	b.WriteString(centerStr(box, m.width))
	b.WriteString("\n")
	return b.String()
}

// ─── Break Prompt ─────────────────────────────────────────────────────────────

func (m Model) renderBreakPrompt() string {
	var b strings.Builder

	taskName := ""
	elapsed := ""
	if m.activeTaskIdx >= 0 && m.activeTaskIdx < len(m.store.Tasks) {
		t := m.store.Tasks[m.activeTaskIdx]
		taskName = t.Name
		if !t.StartedAt.IsZero() {
			dur := time.Since(t.StartedAt)
			elapsed = report.FormatDuration(dur)
		}
	}

	// Header changes based on whether we just finished a break or a focus session.
	var header string
	if m.afterBreak {
		header = ui.StyleSuccess.Render("BREAK COMPLETE — What's next?")
	} else {
		header = ui.StyleSuccess.Render("FOCUS SESSION COMPLETE")
	}

	// During debounce, show a countdown and grey out the actions.
	debounceRem := m.BreakPromptDebounceRemaining()

	var actions string
	if debounceRem > 0 {
		actions = ui.StyleMuted.Render("Keys active in " + itoa(debounceRem) + "s…")
	} else if m.afterBreak {
		// After break: 0=Complete, 1=Abandon
		opts := []string{"Complete task", "Abandon task"}
		cursor := m.breakPromptCursor
		if cursor >= len(opts) {
			cursor = len(opts) - 1
		}
		for i, opt := range opts {
			if i == cursor {
				actions += ui.StyleStatusKey.Render("> ") + opt + "\n"
			} else {
				actions += ui.StyleMuted.Render("  ") + opt + "\n"
			}
		}
	} else {
		// Focus prompt: 0=Break, 1=Extend, 2=Complete, 3=Abandon
		type option struct {
			label      string
			adjustable bool
			value      int
		}
		opts := []option{
			{"Start break", true, m.breakDurationMins},
			{"Extend focus", true, m.breakExtendMins},
			{"Complete task", false, 0},
			{"Abandon task", false, 0},
		}
		for i, opt := range opts {
			var line string
			if i == m.breakPromptCursor {
				if opt.adjustable {
					line = ui.StyleStatusKey.Render("> ") + opt.label + "  " +
						ui.StyleMuted.Render("◀") + " " +
						ui.StyleStatusKey.Render(itoa(opt.value)+"m") + " " +
						ui.StyleMuted.Render("▶")
				} else {
					line = ui.StyleStatusKey.Render("> ") + opt.label
				}
			} else {
				if opt.adjustable {
					line = ui.StyleMuted.Render("  " + opt.label + "  " + itoa(opt.value) + "m")
				} else {
					line = ui.StyleMuted.Render("  " + opt.label)
				}
			}
			actions += line + "\n"
		}
	}

	// Show auto-break hint if configured.
	autoHint := ""
	if m.cfg.Display.AutoStartBreak && !m.afterBreak && debounceRem > 0 && m.autoBreakScheduled {
		autoHint = "\n" + ui.StyleMuted.Render("Break auto-starts after debounce — press any key to cancel")
	}

	var hint string
	if debounceRem <= 0 {
		adjHint := ""
		if !m.afterBreak && m.breakPromptCursor <= 1 {
			adjHint = "  ·  ◀/▶ or -/+ adjust"
		}
		hint = m.keys.up + "/" + m.keys.down + " select  ·  " + m.keys.confirm + " confirm" + adjHint
	}

	content := header + "\n\n" +
		ui.StyleHeader.Render(taskName) + "\n" +
		ui.StyleMuted.Render("Elapsed: "+elapsed) + "\n\n" +
		actions + autoHint + "\n\n" +
		ui.StyleMuted.Render(hint)

	box := ui.StyleBreakBox.Width(m.width - 8).Render(content)
	b.WriteString("\n")
	b.WriteString(centerStr(box, m.width))
	b.WriteString("\n")
	return b.String()
}

// ─── Edit Task Form ───────────────────────────────────────────────────────────

func (m Model) renderEditTask() string {
	var b strings.Builder

	title := "NEW TASK"
	if m.editingTaskID != "" {
		title = "EDIT TASK"
	}

	b.WriteString(m.renderBanner(title, ui.StyleHeader))
	b.WriteString("\n")

	labels := []string{"Group", "Name", "Focus (min)", "Break (min)"}
	for i, inp := range m.editInputs {
		label := labels[i]
		isSelected := editField(i) == m.editField
		isEditing := isSelected && m.editActive

		if isEditing {
			b.WriteString(ui.StyleStatusKey.Render("  ✎ " + label + ": "))
		} else if isSelected {
			b.WriteString(ui.StyleStatusKey.Render("  > " + label + ": "))
		} else {
			b.WriteString(ui.StyleMuted.Render("    " + label + ": "))
		}

		if isEditing {
			b.WriteString(inp.View())
		} else {
			// In navigation mode, show the current value as plain text (no cursor).
			val := inp.Value()
			if val == "" {
				b.WriteString(ui.StyleMuted.Render(inp.Placeholder))
			} else {
				b.WriteString(val)
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	if m.editField == fieldGroup {
		b.WriteString("\n")
		if len(m.groupSuggestions) > 0 {
			if m.editActive && m.editInputs[fieldGroup].Value() != "" {
				b.WriteString(ui.StyleMuted.Render("  Matches:") + "\n")
			} else {
				b.WriteString(ui.StyleMuted.Render("  Groups:") + "\n")
			}
			for _, sug := range m.groupSuggestions {
				b.WriteString(ui.StyleMuted.Render("    · ") + ui.StyleStatusKey.Render(sug) + "\n")
			}
		} else if len(m.store.Groups) == 0 {
			b.WriteString(ui.StyleMuted.Render("  No groups yet — type a name to create one") + "\n")
		} else {
			b.WriteString(ui.StyleMuted.Render("  No matches") + "\n")
		}
	}

	b.WriteString("\n")

	var hint string
	if m.editActive {
		hint = m.keys.confirm + " confirm  ·  " + m.keys.close + " cancel"
	} else {
		hint = m.keys.up + "/" + m.keys.down + " select  ·  " + m.keys.edit + " edit  ·  " + m.keys.confirm + " save  ·  " + m.keys.close + " cancel"
	}
	b.WriteString(m.renderStatusBar([]string{hint}))
	return b.String()
}

// ─── Group List ───────────────────────────────────────────────────────────────

func (m Model) renderGroupList() string {
	var b strings.Builder

	b.WriteString(m.renderBanner("GROUPS", ui.StyleSubHeader))
	b.WriteString("\n")

	if len(m.store.Groups) == 0 {
		b.WriteString(ui.StyleMuted.Render("  No groups yet."))
		b.WriteString("\n")
	} else {
		for i, g := range m.store.Groups {
			total := storage.TotalFocusMinutes(m.store, g.ID)
			line := "  " + g.Name + ui.StyleMuted.Render("  "+m.FormatDuration(total))
			if i == m.groupCursor {
				lw := lipgloss.Width(line)
				if lw < m.width {
					line = line + strings.Repeat(" ", m.width-lw)
				}
				line = ui.StyleSelected.Render(line)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	if m.editGroupID == "new" {
		b.WriteString("\n")
		b.WriteString(ui.StyleMuted.Render("  New group name: "))
		b.WriteString(m.pauseInput.View())
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.renderStatusBar([]string{
		m.keys.newTask + " new group",
		m.keys.delete + " delete",
		m.keys.close + " back",
	}))
	return b.String()
}

// ─── Completion Animation ─────────────────────────────────────────────────────

// confettiChars is the palette of characters used in the animation.
var confettiChars = []string{"✦", "✧", "*", "·", "+", "✨", "★", "•", "◆", "◇"}

// confettiColors cycles through celebratory colors.
var confettiColors = []lipgloss.Color{
	"#FFD700", // gold
	"#FF69B4", // hot pink
	"#00CED1", // dark turquoise
	"#7CFC00", // lawn green
	"#FF6347", // tomato
	"#9370DB", // medium purple
	"#40E0D0", // turquoise
	"#FFA500", // orange
}

func (m Model) renderCompletion() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	// Build a canvas of spaces.
	canvas := make([][]rune, m.height)
	for i := range canvas {
		canvas[i] = make([]rune, m.width)
		for j := range canvas[i] {
			canvas[i][j] = ' '
		}
	}

	// Scatter confetti using frame as a pseudo-random seed.
	numParticles := (m.width * m.height) / 30
	if numParticles > 120 {
		numParticles = 120
	}
	var b strings.Builder

	// Render background with ANSI positioning for confetti.
	b.WriteString("\033[2J\033[H") // clear screen, home cursor

	// Scatter confetti.
	for i := 0; i < numParticles; i++ {
		// Deterministic pseudo-random from frame and particle index.
		seed := (m.completionFrame*7 + i*13 + i*i*3) % 1000
		row := 1 + (seed*m.height/1000+m.completionFrame+i*3)%m.height
		col := 1 + ((seed*m.width/1000 + i*7 + m.completionFrame*2) % m.width)
		charIdx := (i + m.completionFrame) % len(confettiChars)
		colorIdx := (i*3 + m.completionFrame) % len(confettiColors)

		ch := confettiChars[charIdx]
		color := confettiColors[colorIdx]

		style := lipgloss.NewStyle().Foreground(color)
		b.WriteString("\033[" + itoa(row) + ";" + itoa(col) + "H")
		b.WriteString(style.Render(ch))
	}

	// Render the centered completion message on top of confetti.
	centerRow := m.height/2 - 2
	if centerRow < 1 {
		centerRow = 1
	}

	taskDisplay := m.completionTaskName
	if len([]rune(taskDisplay)) > m.width-8 {
		runes := []rune(taskDisplay)
		taskDisplay = string(runes[:m.width-9]) + "…"
	}

	completeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFD700")).
		Bold(true)
	taskStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true)

	completeLine := completeStyle.Render("  ✦ TASK COMPLETE ✦  ")
	taskLine := taskStyle.Render(taskDisplay)

	// Center the completion label.
	clw := lipgloss.Width(completeLine)
	clCol := max(1, (m.width-clw)/2+1)
	b.WriteString("\033[" + itoa(centerRow) + ";" + itoa(clCol) + "H")
	b.WriteString(completeLine)

	// Center the task name below.
	tlw := lipgloss.Width(taskLine)
	tlCol := max(1, (m.width-tlw)/2+1)
	b.WriteString("\033[" + itoa(centerRow+2) + ";" + itoa(tlCol) + "H")
	b.WriteString(taskLine)

	return b.String()
}

// ─── Report ───────────────────────────────────────────────────────────────────

func (m Model) renderReport() string {
	var b strings.Builder

	r := m.cachedReport

	b.WriteString(m.renderBanner("REPORT", ui.StyleReportHeader))
	b.WriteString("\n")
	b.WriteString("  " + ui.StyleMuted.Render(
		itoa(r.TotalTasks)+" tasks  "+
			itoa(r.OnTime)+" on-time  "+
			itoa(r.Overran)+" overran",
	) + "\n")
	b.WriteString(ui.StyleDivider.Render(strings.Repeat("─", m.width-2)))
	b.WriteString("\n")
	b.WriteString(ui.StyleMuted.Render(
		padRight("Task", 28) +
			padRight("Expected", 10) +
			padRight("Actual", 10) +
			padRight("Delta", 8) +
			"Interrupts",
	))
	b.WriteString("\n")
	b.WriteString(ui.StyleDivider.Render(strings.Repeat("─", m.width-2)))
	b.WriteString("\n")

	vis := max(1, m.height-10)
	start := m.reportScroll
	if start > len(r.Tasks) {
		start = len(r.Tasks)
	}
	end := start + vis
	if end > len(r.Tasks) {
		end = len(r.Tasks)
	}

	for _, tr := range r.Tasks[start:end] {
		name := truncate(tr.Task.Name, 26)
		expected := report.FormatDuration(tr.ExpectedDuration)
		actual := report.FormatDuration(tr.ActualDuration)
		delta := report.FormatDelta(tr.Delta)

		deltaStyle := ui.StyleOnTime
		if tr.Delta > 0 {
			deltaStyle = ui.StyleOverrun
		}

		row := padRight(name, 28) +
			padRight(expected, 10) +
			padRight(actual, 10) +
			deltaStyle.Render(padRight(delta, 8)) +
			itoa(tr.InterruptCount)

		b.WriteString("  " + row + "\n")
	}

	b.WriteString(ui.StyleDivider.Render(strings.Repeat("─", m.width-2)))
	b.WriteString("\n\n")
	b.WriteString(ui.StyleInterruptLabel.Render("  Interrupts:"))
	b.WriteString("\n")

	hasInterrupts := false
	for _, tr := range r.Tasks {
		if len(tr.Task.Interrupts) == 0 {
			continue
		}
		hasInterrupts = true
		b.WriteString(ui.StyleSubHeader.Render("  " + tr.Task.Name))
		b.WriteString("\n")
		for _, iv := range tr.Task.Interrupts {
			ts := iv.Time.Format("15:04")
			b.WriteString(ui.StyleMuted.Render("    " + ts + " — " + iv.Reason))
			b.WriteString("\n")
		}
	}
	if !hasInterrupts {
		b.WriteString(ui.StyleMuted.Render("  None."))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.renderStatusBar([]string{
		m.keys.up + "/" + m.keys.down + " scroll",
		m.keys.close + " back",
	}))
	return b.String()
}

// ─── Completed ────────────────────────────────────────────────────────────────

func (m Model) renderCompleted() string {
	var b strings.Builder

	b.WriteString(m.renderBanner("COMPLETED", ui.StyleHeader))
	b.WriteString("\n")

	completed := storage.CompletedTasks(m.store)

	if len(completed) == 0 {
		b.WriteString(ui.StyleMuted.Render("  No completed tasks yet."))
		b.WriteString("\n")
	} else {
		vis := max(1, m.height-5)
		end := m.completedOffset + vis
		if end > len(completed) {
			end = len(completed)
		}

		if m.completedOffset > 0 {
			b.WriteString(ui.StyleMuted.Render("  ↑ " + itoa(m.completedOffset) + " more above"))
			b.WriteString("\n")
		}

		for i := m.completedOffset; i < end; i++ {
			t := completed[i]

			elapsed := "—"
			if !t.StartedAt.IsZero() && !t.EndedAt.IsZero() {
				elapsed = report.FormatDuration(t.EndedAt.Sub(t.StartedAt))
			}

			line := "  " + ui.StyleSuccess.Render("✓") + " " +
				ui.StyleCompleted.Render(t.Name) +
				ui.StyleMuted.Render("  "+m.FormatDuration(t.FocusTime)+" → "+elapsed)

			if i == m.completedCursor {
				lw := lipgloss.Width(line)
				if lw < m.width {
					line = line + strings.Repeat(" ", m.width-lw)
				}
				line = ui.StyleSelected.Render(line)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}

		below := len(completed) - end
		if below > 0 {
			b.WriteString(ui.StyleMuted.Render("  ↓ " + itoa(below) + " more below"))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(m.renderStatusBar([]string{
		m.keys.up + "/" + m.keys.down + " scroll",
		m.keys.close + " back",
	}))
	return b.String()
}

// ─── Error overlay ────────────────────────────────────────────────────────────

func (m Model) renderError() string {
	content := ui.StyleError.Render("ERROR") + "\n\n" +
		m.errorMsg + "\n\n" +
		ui.StyleMuted.Render("Press any key to continue")
	box := ui.StyleErrorBox.Width(m.width - 8).Render(content)

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(centerStr(box, m.width))
	b.WriteString("\n")
	return b.String()
}

// ─── Shared helpers ───────────────────────────────────────────────────────────

func (m Model) renderBanner(title string, style lipgloss.Style) string {
	t := style.Render(title)
	tLen := lipgloss.Width(t)
	pad := ""
	if m.width > tLen+4 {
		dashes := (m.width - tLen - 2) / 2
		pad = ui.StyleDivider.Render(strings.Repeat("─", dashes))
	}
	return pad + " " + t + " " + pad
}

func (m Model) renderStatusBar(hints []string) string {
	bar := strings.Join(hints, "  ·  ")
	if lipgloss.Width(bar) > m.width-2 {
		bar = string([]rune(bar)[:max(0, m.width-3)]) + "…"
	}
	return ui.StyleStatusBar.Render("  " + bar)
}

func (m Model) renderProgressBar(progress float64, width int) string {
	if width < 4 {
		return ""
	}
	filled := int(float64(width) * progress)
	if filled > width {
		filled = width
	}
	full := ui.StyleProgressFull.Render(strings.Repeat("█", filled))
	empty := ui.StyleProgressEmpty.Render(strings.Repeat("░", width-filled))
	return full + empty
}

func centerStr(s string, totalWidth int) string {
	w := lipgloss.Width(s)
	if w >= totalWidth {
		return s
	}
	pad := (totalWidth - w) / 2
	return strings.Repeat(" ", pad) + s
}

func padRight(s string, w int) string {
	l := len([]rune(s))
	if l >= w {
		return string([]rune(s)[:w])
	}
	return s + strings.Repeat(" ", w-l)
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}
