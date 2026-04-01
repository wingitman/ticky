package app

import (
	"os/exec"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wingitman/ticky/internal/config"
	"github.com/wingitman/ticky/internal/overlay"
	"github.com/wingitman/ticky/internal/report"
	"github.com/wingitman/ticky/internal/session"
	"github.com/wingitman/ticky/internal/storage"
	"github.com/wingitman/ticky/internal/timer"
)

// ─── Mode ─────────────────────────────────────────────────────────────────────

// Mode represents the active TUI screen / overlay.
type Mode int

const (
	ModeTaskList    Mode = iota // default: scrollable task list
	ModeGroupList               // group management view
	ModeEditTask                // create / edit task (tabbed form)
	ModeTimerFocus              // running focus countdown
	ModeTimerBreak              // running break countdown
	ModePausePrompt             // "why are you pausing?" overlay
	ModeBreakPrompt             // focus-done choice overlay
	ModeReport                  // report table
	ModeCompleted               // completed tasks list (bug 3: was ModeHistory)
	ModeError                   // unrecoverable error overlay
)

// editField indexes which field is focused in the task edit form.
type editField int

const (
	fieldName editField = iota
	fieldFocusTime
	fieldBreakTime
	fieldGroup
	fieldCount // sentinel
)

// ─── resolvedKeys ─────────────────────────────────────────────────────────────

type resolvedKeys struct {
	up, down  string
	edit      string
	confirm   string
	start     string
	close     string
	format    string
	options   string
	pause     string
	stop      string
	newTask   string
	delete    string
	group     string
	report    string
	completed string
}

func resolveKeys(k config.Keybinds) resolvedKeys {
	return resolvedKeys{
		up:        k.Up,
		down:      k.Down,
		edit:      k.Edit,
		confirm:   k.Confirm,
		start:     k.Start,
		close:     k.Close,
		format:    k.Format,
		options:   k.Options,
		pause:     k.Pause,
		stop:      k.Stop,
		newTask:   k.New,
		delete:    k.Delete,
		group:     k.Group,
		report:    k.Report,
		completed: k.Completed,
	}
}

func matchKey(pressed, binding string) bool {
	return pressed == binding
}

// ─── Model ────────────────────────────────────────────────────────────────────

// Model is the single source of truth for all TUI state.
type Model struct {
	cfg   *config.Config
	store *storage.Store
	sess  *session.Session // active session (may be empty)
	keys  resolvedKeys

	// Terminal dimensions
	width, height int

	// Mode / screen
	mode Mode

	// Task list state
	cursor int
	offset int

	// Which task is actively being timed (index into store.Tasks, -1 = none).
	// Bug 2: this persists even when returning to ModeTaskList so the task
	// stays visible and can be resumed.
	activeTaskIdx int

	// Timer — persists across mode changes (bug 2)
	tmr timer.Timer

	// Edit form state
	editingTaskID string
	editField     editField
	editInputs    [fieldCount]textinput.Model

	// Group list state
	groupCursor int
	groupOffset int
	editGroupID string

	// Pause prompt input (reused for group name entry)
	pauseInput textinput.Model

	// Completed / report view scroll
	completedCursor int
	completedOffset int
	reportScroll    int

	// Cached report
	cachedReport report.Report

	// Time format cycle index
	timeFormatIdx int

	// Error / status messages
	errorMsg  string
	statusMsg string
}

// timeFormats is the ordered cycle toggled by 'f'.
var timeFormats = []string{"minutes", "seconds", "hhmm", "tshirt", "points"}

// ─── New ──────────────────────────────────────────────────────────────────────

// New constructs a ready-to-use Model.
func New(cfg *config.Config, store *storage.Store, sess *session.Session) Model {
	fmtIdx := 0
	for i, f := range timeFormats {
		if f == cfg.Display.TimeFormat {
			fmtIdx = i
			break
		}
	}

	var inputs [fieldCount]textinput.Model
	for i := range inputs {
		ti := textinput.New()
		ti.CharLimit = 120
		inputs[i] = ti
	}
	inputs[fieldName].Placeholder = "Task name"
	inputs[fieldFocusTime].Placeholder = "Focus minutes (1–480, e.g. 25)"
	inputs[fieldBreakTime].Placeholder = "Break minutes (1–60, e.g. 5)"
	inputs[fieldGroup].Placeholder = "Group name (optional)"

	pauseInput := textinput.New()
	pauseInput.Placeholder = "Reason for pausing…"
	pauseInput.CharLimit = 200

	m := Model{
		cfg:           cfg,
		store:         store,
		sess:          sess,
		keys:          resolveKeys(cfg.Keybinds),
		mode:          ModeTaskList,
		activeTaskIdx: -1,
		editInputs:    inputs,
		pauseInput:    pauseInput,
		timeFormatIdx: fmtIdx,
	}

	// Bug 4: if an active session exists and is due, open directly to break prompt.
	// If still running, resume the timer on the task list with the remaining time.
	if session.IsActive(sess) {
		m = m.resumeFromSession(sess)
	}

	return m
}

// resumeFromSession restores timer state from a persisted session.
func (m Model) resumeFromSession(sess *session.Session) Model {
	// Find the task in the store.
	for i := range m.store.Tasks {
		if m.store.Tasks[i].ID == sess.TaskID {
			m.activeTaskIdx = i

			if sess.Phase == session.PhaseFocus {
				remaining := session.Remaining(sess)
				focusMin := m.store.Tasks[i].FocusTime
				m.tmr = timer.New(focusMin)
				m.tmr.Remaining = remaining
				if session.IsDue(sess) {
					// Timer already fired — go straight to break prompt.
					m.store.Tasks[i].EndedAt = time.Now()
					m.mode = ModeBreakPrompt
				} else {
					// Timer still running — resume it.
					m.tmr.Start()
					m.mode = ModeTimerFocus
				}
			} else {
				// Break phase.
				remaining := session.Remaining(sess)
				m.tmr = timer.New(sess.BreakMin)
				m.tmr.Phase = timer.PhaseBreak
				m.tmr.Remaining = remaining
				if session.IsDue(sess) {
					// Break done — back to task list.
					m.mode = ModeTaskList
					m.activeTaskIdx = -1
					_ = session.Delete()
				} else {
					m.tmr.Start()
					m.mode = ModeTimerBreak
				}
			}
			break
		}
	}
	return m
}

// ─── Init ─────────────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	// If we resumed into an active timer, kick off the tick loop.
	if m.mode == ModeTimerFocus || m.mode == ModeTimerBreak {
		return tickCmd()
	}
	return nil
}

// ─── Update ───────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		return m.handleTick()

	case saveErrMsg:
		m.mode = ModeError
		m.errorMsg = "Failed to save: " + string(msg)
		return m, nil

	case clearStatusMsg:
		m.statusMsg = ""
		return m, nil

	case tea.KeyMsg:
		key := msg.String()
		switch m.mode {
		case ModeTaskList:
			return m.updateTaskList(key)
		case ModeGroupList:
			return m.updateGroupList(key)
		case ModeEditTask:
			return m.updateEditTask(msg)
		case ModeTimerFocus, ModeTimerBreak:
			return m.updateTimer(key)
		case ModePausePrompt:
			return m.updatePausePrompt(msg)
		case ModeBreakPrompt:
			return m.updateBreakPrompt(key)
		case ModeReport:
			return m.updateReport(key)
		case ModeCompleted:
			return m.updateCompleted(key)
		case ModeError:
			m.mode = ModeTaskList
			m.errorMsg = ""
			return m, nil
		}
	}

	return m, nil
}

// ─── handleTick ───────────────────────────────────────────────────────────────

func (m Model) handleTick() (tea.Model, tea.Cmd) {
	if m.mode != ModeTimerFocus && m.mode != ModeTimerBreak {
		return m, nil
	}

	done := m.tmr.Tick()
	if !done {
		return m, tickCmd()
	}

	if m.tmr.Phase == timer.PhaseFocus {
		if m.activeTaskIdx >= 0 && m.activeTaskIdx < len(m.store.Tasks) {
			m.store.Tasks[m.activeTaskIdx].EndedAt = time.Now()
		}
		// Delete session — watcher's job is done.
		_ = session.Delete()
		m.mode = ModeBreakPrompt
		return m, nil
	}

	// Break finished.
	_ = session.Delete()
	m.mode = ModeTaskList
	m.activeTaskIdx = -1
	return m, saveCmd(m.store)
}

// ─── ModeTaskList ─────────────────────────────────────────────────────────────

func (m Model) updateTaskList(key string) (tea.Model, tea.Cmd) {
	tasks := storage.ActiveTasks(m.store)

	switch {
	case matchKey(key, m.keys.up):
		if m.cursor > 0 {
			m.cursor--
			m.clampTaskScroll()
		}

	case matchKey(key, m.keys.down):
		if m.cursor < len(tasks)-1 {
			m.cursor++
			m.clampTaskScroll()
		}

	case matchKey(key, m.keys.newTask):
		m = m.beginEditTask("")

	case matchKey(key, m.keys.edit):
		if len(tasks) > 0 && m.cursor < len(tasks) {
			m = m.beginEditTask(tasks[m.cursor].ID)
		}

	case matchKey(key, m.keys.delete):
		if len(tasks) > 0 && m.cursor < len(tasks) {
			// Don't allow deleting the currently active task.
			if m.activeTaskIdx >= 0 && m.store.Tasks[m.activeTaskIdx].ID == tasks[m.cursor].ID {
				m.statusMsg = "Cannot delete a task with an active timer"
				return m, clearStatusCmd()
			}
			storage.DeleteTask(m.store, tasks[m.cursor].ID)
			if m.cursor > 0 {
				m.cursor--
			}
			return m, saveCmd(m.store)
		}

	case matchKey(key, m.keys.pause):
		// p on the task list — only acts if the selected task is the active one.
		if m.activeTaskIdx >= 0 && len(tasks) > 0 && m.cursor < len(tasks) &&
			m.store.Tasks[m.activeTaskIdx].ID == tasks[m.cursor].ID {
			m.tmr.Pause()
			m.pauseInput.SetValue("")
			m.pauseInput.Focus()
			m.mode = ModePausePrompt
			return m, textinput.Blink
		}

	case matchKey(key, m.keys.stop):
		// x on the task list — only acts if the selected task is the active one.
		if m.activeTaskIdx >= 0 && len(tasks) > 0 && m.cursor < len(tasks) &&
			m.store.Tasks[m.activeTaskIdx].ID == tasks[m.cursor].ID {
			return m.stopTask()
		}

	case matchKey(key, m.keys.start):
		if len(tasks) > 0 && m.cursor < len(tasks) {
			selected := tasks[m.cursor]
			// If selected task is the active one, resume its timer.
			if m.activeTaskIdx >= 0 && m.store.Tasks[m.activeTaskIdx].ID == selected.ID {
				m.tmr.Resume()
				m.mode = ModeTimerFocus
				return m, tickCmd()
			}
			// Otherwise start a new task (writes session + quits TUI).
			return m.startTask(selected.ID)
		}

	case matchKey(key, m.keys.group):
		m.mode = ModeGroupList
		m.groupCursor = 0
		m.groupOffset = 0

	case matchKey(key, m.keys.report):
		m.cachedReport = report.Generate(storage.FinishedTasks(m.store), "")
		m.mode = ModeReport
		m.reportScroll = 0

	case matchKey(key, m.keys.completed): // bug 3: was history
		m.mode = ModeCompleted
		m.completedCursor = 0
		m.completedOffset = 0

	case matchKey(key, m.keys.format):
		m.timeFormatIdx = (m.timeFormatIdx + 1) % len(timeFormats)

	case matchKey(key, m.keys.options):
		// Bug 1: use tea.ExecProcess so BubbleTea suspends and hands the
		// terminal fully to the editor, then resumes cleanly after exit.
		return m, openConfigCmd()

	case matchKey(key, m.keys.close), key == "esc":
		// Bug 2/4: if a timer is running, launch background watcher then quit.
		if m.activeTaskIdx >= 0 {
			return m, tea.Batch(launchWatcherCmd(m.store, m.sess, m.activeTaskIdx, m.tmr), tea.Quit)
		}
		return m, tea.Quit
	}

	return m, nil
}

// stopTask cancels the active timer and resets the task back to its unstarted
// state — StartedAt, EndedAt, and Interrupts are cleared so the task can be
// started fresh. The session file and watcher process are also cleaned up.
// The task remains in the active list; it is not marked completed or abandoned.
func (m Model) stopTask() (Model, tea.Cmd) {
	if m.activeTaskIdx < 0 || m.activeTaskIdx >= len(m.store.Tasks) {
		return m, nil
	}

	// Kill the background watcher if one is running.
	killWatcher(m.sess)

	// Reset the task to a clean, unstarted state.
	m.store.Tasks[m.activeTaskIdx].StartedAt = time.Time{}
	m.store.Tasks[m.activeTaskIdx].EndedAt = time.Time{}
	m.store.Tasks[m.activeTaskIdx].Interrupts = nil

	_ = session.Delete()
	m.sess = &session.Session{}
	m.activeTaskIdx = -1
	m.mode = ModeTaskList

	return m, saveCmd(m.store)
}

// startTask begins a focus session for the given task ID.
// Bug 4: writes session.toml, launches --watch subprocess, then quits the TUI.
func (m Model) startTask(id string) (tea.Model, tea.Cmd) {
	for i := range m.store.Tasks {
		if m.store.Tasks[i].ID == id {
			m.activeTaskIdx = i
			m.store.Tasks[i].StartedAt = time.Now()

			focusMin := m.store.Tasks[i].FocusTime
			breakMin := m.store.Tasks[i].BreakTime

			m.tmr = timer.New(focusMin)
			m.tmr.Start()

			endTime := time.Now().Add(time.Duration(focusMin) * time.Minute)

			env := CaptureEnv()
			sess := &session.Session{
				TaskID:     id,
				EndTime:    endTime,
				Phase:      session.PhaseFocus,
				BreakMin:   breakMin,
				TTY:        resolveTTY(),
				InTmux:     env.InTmux,
				NvimSocket: env.NvimSocket,
				VimContext: env.VimContext,
			}

			return m, tea.Batch(
				saveCmd(m.store),
				saveSessionCmd(sess),
				launchWatcherCmd(m.store, sess, i, m.tmr),
				tea.Quit,
			)
		}
	}
	return m, nil
}

// ─── ModeEditTask ─────────────────────────────────────────────────────────────

func (m Model) beginEditTask(id string) Model {
	m.editingTaskID = id
	m.editField = fieldName

	for i := range m.editInputs {
		m.editInputs[i].SetValue("")
		m.editInputs[i].Blur()
	}

	if id != "" {
		for _, t := range m.store.Tasks {
			if t.ID == id {
				m.editInputs[fieldName].SetValue(t.Name)
				m.editInputs[fieldFocusTime].SetValue(itoa(t.FocusTime))
				m.editInputs[fieldBreakTime].SetValue(itoa(t.BreakTime))
				if g := storage.FindGroup(m.store, t.GroupID); g != nil {
					m.editInputs[fieldGroup].SetValue(g.Name)
				}
				break
			}
		}
	}

	m.editInputs[fieldName].Focus()
	m.mode = ModeEditTask
	return m
}

func (m Model) updateEditTask(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc":
		m.mode = ModeTaskList
		return m, nil

	case "tab", "shift+tab":
		m.editInputs[m.editField].Blur()
		if key == "tab" {
			m.editField = (m.editField + 1) % fieldCount
		} else {
			m.editField = (m.editField + fieldCount - 1) % fieldCount
		}
		m.editInputs[m.editField].Focus()
		var cmd tea.Cmd
		m.editInputs[m.editField], cmd = m.editInputs[m.editField].Update(msg)
		return m, cmd

	case "enter":
		return m.commitEditTask()
	}

	var cmd tea.Cmd
	m.editInputs[m.editField], cmd = m.editInputs[m.editField].Update(msg)
	return m, cmd
}

func (m Model) commitEditTask() (tea.Model, tea.Cmd) {
	name := m.editInputs[fieldName].Value()
	if name == "" {
		m.statusMsg = "Task name is required"
		return m, clearStatusCmd()
	}

	focusMin := parseIntClamped(m.editInputs[fieldFocusTime].Value(), 25, 1, 480)
	breakMin := parseIntClamped(m.editInputs[fieldBreakTime].Value(), 5, 1, 60)

	groupName := m.editInputs[fieldGroup].Value()
	groupID := ""
	if groupName != "" {
		groupID = m.findOrCreateGroup(groupName)
	}

	if m.editingTaskID == "" {
		t := storage.Task{
			ID:        storage.NewID(),
			GroupID:   groupID,
			Name:      name,
			FocusTime: focusMin,
			BreakTime: breakMin,
		}
		storage.UpsertTask(m.store, t)
	} else {
		for i := range m.store.Tasks {
			if m.store.Tasks[i].ID == m.editingTaskID {
				m.store.Tasks[i].Name = name
				m.store.Tasks[i].FocusTime = focusMin
				m.store.Tasks[i].BreakTime = breakMin
				m.store.Tasks[i].GroupID = groupID
				break
			}
		}
	}

	m.mode = ModeTaskList
	return m, saveCmd(m.store)
}

func (m *Model) findOrCreateGroup(name string) string {
	for _, g := range m.store.Groups {
		if g.Name == name {
			return g.ID
		}
	}
	g := storage.Group{ID: storage.NewID(), Name: name}
	storage.UpsertGroup(m.store, g)
	return g.ID
}

// ─── ModeGroupList ────────────────────────────────────────────────────────────

func (m Model) updateGroupList(key string) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(key, m.keys.up):
		if m.groupCursor > 0 {
			m.groupCursor--
		}

	case matchKey(key, m.keys.down):
		if m.groupCursor < len(m.store.Groups)-1 {
			m.groupCursor++
		}

	case matchKey(key, m.keys.newTask):
		ti := textinput.New()
		ti.Placeholder = "Group name"
		ti.CharLimit = 80
		ti.Focus()
		m.pauseInput = ti
		m.editGroupID = "new"

	case matchKey(key, m.keys.delete):
		if m.groupCursor < len(m.store.Groups) {
			storage.DeleteGroup(m.store, m.store.Groups[m.groupCursor].ID)
			if m.groupCursor > 0 {
				m.groupCursor--
			}
			return m, saveCmd(m.store)
		}

	case key == "enter":
		if m.editGroupID == "new" {
			name := m.pauseInput.Value()
			if name != "" {
				g := storage.Group{ID: storage.NewID(), Name: name}
				storage.UpsertGroup(m.store, g)
				m.editGroupID = ""
				m.pauseInput.SetValue("")
				return m, saveCmd(m.store)
			}
			m.editGroupID = ""
		}

	case matchKey(key, m.keys.close), key == "esc":
		m.mode = ModeTaskList
		m.editGroupID = ""
		return m, nil
	}

	if m.editGroupID == "new" {
		var cmd tea.Cmd
		m.pauseInput, cmd = m.pauseInput.Update(tea.KeyMsg{Type: tea.KeyRunes})
		_ = cmd
	}

	return m, nil
}

// ─── ModeTimerFocus / ModeTimerBreak ─────────────────────────────────────────

// updateTimer handles keys while the timer is on screen.
func (m Model) updateTimer(key string) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(key, m.keys.pause):
		// p — pause and prompt for reason.
		m.tmr.Pause()
		m.pauseInput.SetValue("")
		m.pauseInput.Focus()
		m.mode = ModePausePrompt
		return m, textinput.Blink

	case matchKey(key, m.keys.stop):
		// x — stop and reset the task entirely.
		return m.stopTask()

	case matchKey(key, m.keys.close), key == "esc":
		// q/esc — leave timer running in background, return to task list.
		m.mode = ModeTaskList
		if m.sess != nil && session.IsActive(m.sess) {
			return m, launchWatcherCmd(m.store, m.sess, m.activeTaskIdx, m.tmr)
		}
		return m, nil
	}

	return m, tickCmd()
}

// ─── ModePausePrompt ─────────────────────────────────────────────────────────

func (m Model) updatePausePrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "enter":
		reason := m.pauseInput.Value()
		if reason != "" && m.activeTaskIdx >= 0 && m.activeTaskIdx < len(m.store.Tasks) {
			interrupt := storage.Interrupt{
				Time:   time.Now(),
				Reason: reason,
			}
			m.store.Tasks[m.activeTaskIdx].Interrupts = append(
				m.store.Tasks[m.activeTaskIdx].Interrupts, interrupt,
			)
		}
		m.pauseInput.SetValue("")
		m.pauseInput.Blur()
		m.tmr.Resume()
		if m.tmr.Phase == timer.PhaseFocus {
			m.mode = ModeTimerFocus
		} else {
			m.mode = ModeTimerBreak
		}
		return m, tickCmd()

	case "esc":
		m.pauseInput.SetValue("")
		m.pauseInput.Blur()
		m.tmr.Resume()
		if m.tmr.Phase == timer.PhaseFocus {
			m.mode = ModeTimerFocus
		} else {
			m.mode = ModeTimerBreak
		}
		return m, tickCmd()
	}

	var cmd tea.Cmd
	m.pauseInput, cmd = m.pauseInput.Update(msg)
	return m, cmd
}

// ─── ModeBreakPrompt ─────────────────────────────────────────────────────────

func (m Model) updateBreakPrompt(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "e", "E":
		// Extend focus by 5 minutes.
		m.tmr.Extend(5)
		endTime := time.Now().Add(m.tmr.Remaining)
		if m.sess == nil {
			m.sess = &session.Session{}
		}
		m.sess.EndTime = endTime
		m.sess.Phase = session.PhaseFocus
		m.mode = ModeTimerFocus
		return m, tea.Batch(tickCmd(), saveSessionCmd(m.sess), launchWatcherCmd(m.store, m.sess, m.activeTaskIdx, m.tmr))

	case "b", "B":
		// Start break.
		breakMin := 5
		if m.activeTaskIdx >= 0 && m.activeTaskIdx < len(m.store.Tasks) {
			breakMin = m.store.Tasks[m.activeTaskIdx].BreakTime
		}
		m.tmr.StartBreak(breakMin)
		endTime := time.Now().Add(m.tmr.Remaining)
		if m.sess == nil {
			m.sess = &session.Session{}
		}
		m.sess.EndTime = endTime
		m.sess.Phase = session.PhaseBreak
		m.sess.BreakMin = breakMin
		m.mode = ModeTimerBreak
		return m, tea.Batch(tickCmd(), saveSessionCmd(m.sess), launchWatcherCmd(m.store, m.sess, m.activeTaskIdx, m.tmr))

	case "c", "C":
		// Complete task.
		if m.activeTaskIdx >= 0 && m.activeTaskIdx < len(m.store.Tasks) {
			m.store.Tasks[m.activeTaskIdx].Completed = true
			if m.store.Tasks[m.activeTaskIdx].EndedAt.IsZero() {
				m.store.Tasks[m.activeTaskIdx].EndedAt = time.Now()
			}
		}
		_ = session.Delete()
		m.mode = ModeTaskList
		m.activeTaskIdx = -1
		return m, saveCmd(m.store)

	case "a", "A":
		// Abandon task.
		if m.activeTaskIdx >= 0 && m.activeTaskIdx < len(m.store.Tasks) {
			m.store.Tasks[m.activeTaskIdx].Abandoned = true
			if m.store.Tasks[m.activeTaskIdx].EndedAt.IsZero() {
				m.store.Tasks[m.activeTaskIdx].EndedAt = time.Now()
			}
		}
		_ = session.Delete()
		m.mode = ModeTaskList
		m.activeTaskIdx = -1
		return m, saveCmd(m.store)
	}

	return m, nil
}

// ─── ModeReport ───────────────────────────────────────────────────────────────

func (m Model) updateReport(key string) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(key, m.keys.up):
		if m.reportScroll > 0 {
			m.reportScroll--
		}
	case matchKey(key, m.keys.down):
		m.reportScroll++
	case matchKey(key, m.keys.close), key == "esc":
		m.mode = ModeTaskList
	}
	return m, nil
}

// ─── ModeCompleted (bug 3: was ModeHistory) ───────────────────────────────────

func (m Model) updateCompleted(key string) (tea.Model, tea.Cmd) {
	// Bug 3: use CompletedTasks (not FinishedTasks) — no abandoned tasks here.
	completed := storage.CompletedTasks(m.store)
	switch {
	case matchKey(key, m.keys.up):
		if m.completedCursor > 0 {
			m.completedCursor--
			m.clampCompletedScroll()
		}
	case matchKey(key, m.keys.down):
		if m.completedCursor < len(completed)-1 {
			m.completedCursor++
			m.clampCompletedScroll()
		}
	case matchKey(key, m.keys.close), key == "esc":
		m.mode = ModeTaskList
	}
	return m, nil
}

// ─── Scroll helpers ───────────────────────────────────────────────────────────

func (m *Model) visibleTaskRows() int {
	reserved := 5
	return max(1, m.height-reserved)
}

func (m *Model) clampTaskScroll() {
	vis := m.visibleTaskRows()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+vis {
		m.offset = m.cursor - vis + 1
	}
}

func (m *Model) clampCompletedScroll() {
	vis := max(1, m.height-5)
	if m.completedCursor < m.completedOffset {
		m.completedOffset = m.completedCursor
	}
	if m.completedCursor >= m.completedOffset+vis {
		m.completedOffset = m.completedCursor - vis + 1
	}
}

// ─── Overlay helper ───────────────────────────────────────────────────────────

// OverlayWidget returns the corner widget string for the current session,
// respecting display config. Empty string means nothing to render.
func (m Model) OverlayWidget() string {
	return overlay.CornerWidget(m.cfg, m.sess, m.store)
}

// ─── Commands ─────────────────────────────────────────────────────────────────

type tickMsg struct{}
type saveErrMsg string
type clearStatusMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(_ time.Time) tea.Msg {
		return tickMsg{}
	})
}

func saveCmd(s *storage.Store) tea.Cmd {
	return func() tea.Msg {
		if err := storage.Save(s); err != nil {
			return saveErrMsg(err.Error())
		}
		return nil
	}
}

func saveSessionCmd(sess *session.Session) tea.Cmd {
	return func() tea.Msg {
		_ = session.Save(sess)
		return nil
	}
}

func clearStatusCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

// openConfigCmd suspends BubbleTea and hands the terminal to the user's editor.
// Bug 1: replaces the old fire-and-forget cmd.Start() approach.
func openConfigCmd() tea.Cmd {
	path, err := config.ConfigPath()
	if err != nil {
		return func() tea.Msg { return saveErrMsg("cannot resolve config path: " + err.Error()) }
	}
	editor := config.ResolveEditor()
	return tea.ExecProcess(exec.Command(editor, path), func(err error) tea.Msg {
		if err != nil {
			return saveErrMsg("editor error: " + err.Error())
		}
		return nil
	})
}

// launchWatcherCmd starts a background ticky --watch process that will
// re-exec ticky --break on the stored TTY when the timer fires.
// Bug 4: this is what keeps the timer alive after ticky exits.
func launchWatcherCmd(store *storage.Store, sess *session.Session, taskIdx int, tmr timer.Timer) tea.Cmd {
	return func() tea.Msg {
		// Resolve the ticky binary path (re-exec self).
		self := resolveSelf()
		if self == "" {
			return nil
		}
		cmd := exec.Command(self, "--watch")
		cmd.Stdin = nil
		cmd.Stdout = nil
		cmd.Stderr = nil
		// Detach from parent process so it survives ticky exiting.
		setSysProcAttr(cmd)
		if err := cmd.Start(); err != nil {
			return nil // non-fatal — watcher is best-effort
		}
		// Store the watcher PID in the session so we can kill it if needed.
		if sess != nil {
			sess.WatchPID = cmd.Process.Pid
			_ = session.Save(sess)
		}
		return nil
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (m Model) FormatDuration(minutes int) string {
	format := timeFormats[m.timeFormatIdx]
	switch format {
	case "seconds":
		return itoa(minutes*60) + "s"
	case "hhmm":
		h := minutes / 60
		min := minutes % 60
		if h > 0 {
			return itoa(h) + ":" + pad2(min)
		}
		return "00:" + pad2(minutes)
	case "tshirt":
		switch {
		case minutes <= 15:
			return "XS"
		case minutes <= 30:
			return "S"
		case minutes <= 60:
			return "M"
		case minutes <= 120:
			return "L"
		case minutes <= 240:
			return "XL"
		default:
			return "XXL"
		}
	case "points":
		switch {
		case minutes <= 15:
			return "1"
		case minutes <= 30:
			return "2"
		case minutes <= 60:
			return "3"
		case minutes <= 120:
			return "5"
		case minutes <= 240:
			return "8"
		default:
			return "13"
		}
	default:
		return itoa(minutes) + "m"
	}
}

func (m Model) GroupName(groupID string) string {
	if groupID == "" {
		return ""
	}
	g := storage.FindGroup(m.store, groupID)
	if g == nil {
		return ""
	}
	return g.Name
}

// ActiveTaskID returns the ID of the currently timed task, or "".
func (m Model) ActiveTaskID() string {
	if m.activeTaskIdx < 0 || m.activeTaskIdx >= len(m.store.Tasks) {
		return ""
	}
	return m.store.Tasks[m.activeTaskIdx].ID
}

func parseIntClamped(s string, def, minVal, maxVal int) int {
	if s == "" {
		return def
	}
	n := 0
	neg := false
	for i, c := range s {
		if i == 0 && c == '-' {
			neg = true
			continue
		}
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
	}
	if neg {
		n = -n
	}
	if n < minVal {
		return minVal
	}
	if n > maxVal {
		return maxVal
	}
	return n
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}

func pad2(n int) string {
	if n < 10 {
		return "0" + itoa(n)
	}
	return itoa(n)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
