package app

import (
	"os/exec"
	"strings"
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
	ModeEditTask                // create / edit task (navigable form)
	ModeTaskActions             // action sub-menu for active task
	ModeTimerFocus              // running focus countdown
	ModeTimerBreak              // running break countdown
	ModePausePrompt             // "why are you pausing?" overlay
	ModeBreakPrompt             // focus-done / break-done choice overlay
	ModeCompletion              // celebratory animation on task complete
	ModeReport                  // report table
	ModeCompleted               // completed tasks list
	ModeError                   // unrecoverable error overlay
)

// editField indexes which field is focused in the task edit form.
type editField int

const (
	fieldGroup editField = iota
	fieldName
	fieldFocusTime
	fieldBreakTime
	fieldCount // sentinel
)

// taskActionsConfirm tracks which action the actions sub-menu is confirming.
type taskActionsConfirm int

const (
	confirmNone     taskActionsConfirm = iota
	confirmComplete                    // waiting for y/n to complete task
	confirmAbandon                     // waiting for y/n to abandon task
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
	increase  string
	decrease  string
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
		increase:  k.Increase,
		decrease:  k.Decrease,
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
	activeTaskIdx int

	// Timer — persists across mode changes
	tmr timer.Timer

	// Edit form state
	editingTaskID    string
	editField        editField
	editActive       bool // true while a text input is being edited (vs. navigating)
	editInputs       [fieldCount]textinput.Model
	groupSuggestions []string

	// Group list state
	groupCursor int
	groupOffset int
	editGroupID string

	// Pause prompt input (reused for group name entry)
	pauseInput textinput.Model

	// Task actions sub-menu
	actionsConfirm taskActionsConfirm
	actionsCursor  int // selected item in the actions menu (0=pause/resume, 1=stop, 2=complete, 3=abandon)

	// Break prompt state
	breakPromptEnteredAt time.Time // when ModeBreakPrompt was entered (for debounce)
	afterBreak           bool      // true when coming from a completed break (not focus)
	autoBreakScheduled   bool      // true when auto_start_break countdown is running
	// Cursor: 0=break, 1=extend, 2=complete, 3=abandon  (afterBreak: 0=complete, 1=abandon)
	breakPromptCursor int
	breakExtendMins   int // minutes to extend focus (default 5)
	breakDurationMins int // override break duration (0 = use task default)

	// Completion animation state
	completionFrame    int
	completionTaskName string

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
	inputs[fieldGroup].Placeholder = "Group name (optional)"
	inputs[fieldName].Placeholder = "Task name"
	inputs[fieldFocusTime].Placeholder = "Focus minutes (0–480, e.g. 25)"
	inputs[fieldBreakTime].Placeholder = "Break minutes (0–60, e.g. 5)"

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

	if session.IsActive(sess) {
		m = m.resumeFromSession(sess)
	}

	return m
}

// resumeFromSession restores timer state from a persisted session.
func (m Model) resumeFromSession(sess *session.Session) Model {
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
					m.breakPromptEnteredAt = time.Now()
				} else if sess.Paused {
					// Session is paused — show timer in paused state.
					m.tmr.Start()
					m.tmr.Pause()
					m.mode = ModeTimerFocus
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
					// Break done — show the re-prompt.
					_ = session.Delete()
					m.mode = ModeBreakPrompt
					m.breakPromptEnteredAt = time.Now()
					m.afterBreak = true
				} else if sess.Paused {
					m.tmr.Start()
					m.tmr.Pause()
					m.mode = ModeTimerBreak
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
	if m.mode == ModeTimerFocus || m.mode == ModeTimerBreak {
		if m.tmr.State == timer.StateRunning {
			return tea.Batch(tickCmd(), pollSessionCmd())
		}
		// Paused on resume — still poll so other clients can sync.
		return pollSessionCmd()
	}
	if m.mode == ModeBreakPrompt {
		return m.breakPromptInitCmd()
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

	case sessionPollMsg:
		return m.handleSessionPoll()

	case completionTickMsg:
		return m.handleCompletionTick()

	case autoStartBreakMsg:
		return m.handleAutoStartBreak()

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
		case ModeTaskActions:
			return m.updateTaskActions(key)
		case ModeTimerFocus, ModeTimerBreak:
			return m.updateTimer(key)
		case ModePausePrompt:
			return m.updatePausePrompt(msg)
		case ModeBreakPrompt:
			return m.updateBreakPrompt(key)
		case ModeCompletion:
			// No key handling during animation — it auto-exits.
			return m, nil
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

	// If paused, don't advance the timer — but keep polling for sync.
	if m.tmr.State == timer.StatePaused {
		return m, pollSessionCmd()
	}

	done := m.tmr.Tick()
	if !done {
		return m, tea.Batch(tickCmd(), pollSessionCmd())
	}

	if m.tmr.Phase == timer.PhaseFocus {
		if m.activeTaskIdx >= 0 && m.activeTaskIdx < len(m.store.Tasks) {
			m.store.Tasks[m.activeTaskIdx].EndedAt = time.Now()
		}
		// Delete session — watcher's job is done.
		_ = session.Delete()
		m.mode = ModeBreakPrompt
		m.breakPromptEnteredAt = time.Now()
		m.afterBreak = false
		m.autoBreakScheduled = false
		m.breakPromptCursor = 0 // default: Break
		m.breakExtendMins = 5
		m.breakDurationMins = m.taskBreakMin()
		return m, m.breakPromptInitCmd()
	}

	// Break finished — delete session and re-show break prompt so the
	// user can resolve the task (complete / abandon / extend).
	_ = session.Delete()
	m.mode = ModeBreakPrompt
	m.breakPromptEnteredAt = time.Now()
	m.afterBreak = true
	m.autoBreakScheduled = false
	m.breakPromptCursor = 0 // default: Complete
	m.breakExtendMins = 5
	m.breakDurationMins = m.taskBreakMin()
	return m, m.breakPromptInitCmd()
}

// ─── handleSessionPoll ────────────────────────────────────────────────────────

// handleSessionPoll re-reads session.toml once per second so that changes
// made by another ticky client (pause, extend, stop) are reflected here.
func (m Model) handleSessionPoll() (tea.Model, tea.Cmd) {
	disk, err := session.Load()
	if err != nil {
		return m, pollSessionCmd()
	}

	// Session was cleared by another client (or by this client completing the
	// timer). Only reset to task list if we're still on a timer screen — don't
	// clobber ModeBreakPrompt or any other post-timer mode.
	if !session.IsActive(disk) && m.activeTaskIdx >= 0 {
		m.tmr.Pause() // stop ticking
		m.activeTaskIdx = -1
		m.sess = &session.Session{}
		if m.mode == ModeTimerFocus || m.mode == ModeTimerBreak {
			m.mode = ModeTaskList
		}
		return m, nil
	}

	if !session.IsActive(disk) {
		return m, nil
	}

	// Another client paused the session.
	if disk.Paused && m.tmr.State == timer.StateRunning {
		m.tmr.Pause()
		m.sess.Paused = true
		m.sess.PausedAt = disk.PausedAt
		if m.mode == ModeTimerFocus || m.mode == ModeTimerBreak {
			// Stay on the timer screen but paused.
			return m, pollSessionCmd()
		}
		return m, pollSessionCmd()
	}

	// Another client resumed the session.
	if !disk.Paused && m.tmr.State == timer.StatePaused && m.sess != nil && m.sess.Paused {
		m.tmr.Remaining = session.Remaining(disk)
		m.sess.Paused = false
		m.sess.EndTime = disk.EndTime
		m.tmr.Resume()
		return m, tea.Batch(tickCmd(), pollSessionCmd())
	}

	// Another client extended the timer — update our remaining time.
	if m.sess != nil && !disk.EndTime.IsZero() && disk.EndTime != m.sess.EndTime {
		m.tmr.Remaining = session.Remaining(disk)
		m.tmr.Total = m.tmr.Remaining + (m.tmr.Total - m.tmr.Remaining) // keep progress sensible
		m.sess.EndTime = disk.EndTime
	}

	return m, pollSessionCmd()
}

// ─── handleCompletionTick ─────────────────────────────────────────────────────

func (m Model) handleCompletionTick() (tea.Model, tea.Cmd) {
	m.completionFrame++
	if m.completionFrame >= completionTotalFrames {
		m.mode = ModeTaskList
		return m, nil
	}
	return m, completionTickCmd()
}

// ─── handleAutoStartBreak ─────────────────────────────────────────────────────

func (m Model) handleAutoStartBreak() (tea.Model, tea.Cmd) {
	// Only fire if we're still in the break prompt and no key was pressed
	// (autoBreakScheduled is cleared when the user presses any key).
	if m.mode != ModeBreakPrompt || !m.autoBreakScheduled || m.afterBreak {
		return m, nil
	}
	m.autoBreakScheduled = false
	return m.startBreak()
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
			selected := tasks[m.cursor]
			// If the selected task is the active one, open actions sub-menu.
			if m.activeTaskIdx >= 0 && m.store.Tasks[m.activeTaskIdx].ID == selected.ID {
				m.mode = ModeTaskActions
				m.actionsConfirm = confirmNone
				return m, nil
			}
			// Otherwise open the edit form.
			m = m.beginEditTask(selected.ID)
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
			return m.pauseTimer()
		}

	case matchKey(key, m.keys.stop):
		// x on the task list — only acts if the selected task is the active one.
		if m.activeTaskIdx >= 0 && len(tasks) > 0 && m.cursor < len(tasks) &&
			m.store.Tasks[m.activeTaskIdx].ID == tasks[m.cursor].ID {
			return m.stopTask()
		}

	case matchKey(key, m.keys.increase) || key == "+":
		if len(tasks) > 0 && m.cursor < len(tasks) {
			selected := tasks[m.cursor]
			if m.activeTaskIdx >= 0 && m.store.Tasks[m.activeTaskIdx].ID == selected.ID {
				// Active task — adjust the running timer.
				m, cmd := m.adjustRunningTimer(+1)
				return m, cmd
			}
			// Inactive task — adjust its stored focus time.
			for i := range m.store.Tasks {
				if m.store.Tasks[i].ID == selected.ID {
					if m.store.Tasks[i].FocusTime < 480 {
						m.store.Tasks[i].FocusTime++
					}
					break
				}
			}
			return m, saveCmd(m.store)
		}

	case matchKey(key, m.keys.decrease) || key == "-":
		if len(tasks) > 0 && m.cursor < len(tasks) {
			selected := tasks[m.cursor]
			if m.activeTaskIdx >= 0 && m.store.Tasks[m.activeTaskIdx].ID == selected.ID {
				// Active task — adjust the running timer.
				m, cmd := m.adjustRunningTimer(-1)
				return m, cmd
			}
			// Inactive task — adjust its stored focus time.
			for i := range m.store.Tasks {
				if m.store.Tasks[i].ID == selected.ID {
					if m.store.Tasks[i].FocusTime > 0 {
						m.store.Tasks[i].FocusTime--
					}
					break
				}
			}
			return m, saveCmd(m.store)
		}

	case matchKey(key, m.keys.start):
		if len(tasks) > 0 && m.cursor < len(tasks) {
			selected := tasks[m.cursor]
			// If selected task is the active one, navigate to its timer screen.
			if m.activeTaskIdx >= 0 && m.store.Tasks[m.activeTaskIdx].ID == selected.ID {
				if m.tmr.State == timer.StatePaused {
					// Resume from paused state.
					return m.resumeTimer()
				}
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

	case matchKey(key, m.keys.completed):
		m.mode = ModeCompleted
		m.completedCursor = 0
		m.completedOffset = 0

	case matchKey(key, m.keys.format):
		m.timeFormatIdx = (m.timeFormatIdx + 1) % len(timeFormats)

	case matchKey(key, m.keys.options):
		return m, openConfigCmd()

	case matchKey(key, m.keys.close), key == "esc":
		// If a timer is running, ensure watcher is alive then quit.
		if m.activeTaskIdx >= 0 {
			return m, tea.Batch(launchWatcherCmd(m.sess), tea.Quit)
		}
		return m, tea.Quit
	}

	return m, nil
}

// stopTask cancels the active timer and resets the task.
func (m Model) stopTask() (Model, tea.Cmd) {
	if m.activeTaskIdx < 0 || m.activeTaskIdx >= len(m.store.Tasks) {
		return m, nil
	}

	killWatcher(m.sess)

	m.store.Tasks[m.activeTaskIdx].StartedAt = time.Time{}
	m.store.Tasks[m.activeTaskIdx].EndedAt = time.Time{}
	m.store.Tasks[m.activeTaskIdx].Interrupts = nil

	_ = session.Delete()
	m.sess = &session.Session{}
	m.activeTaskIdx = -1
	m.mode = ModeTaskList

	return m, saveCmd(m.store)
}

// pauseTimer pauses the in-memory timer, writes pause state to disk, and
// kills the --watch subprocess (it must not fire while paused).
func (m Model) pauseTimer() (Model, tea.Cmd) {
	m.tmr.Pause()
	m.pauseInput.SetValue("")
	m.pauseInput.Focus()
	m.mode = ModePausePrompt

	// Persist pause state and kill watcher.
	if m.sess != nil {
		m.sess.Paused = true
		m.sess.PausedAt = time.Now()
		killWatcher(m.sess)
		m.sess.WatchPID = 0
	}

	return m, tea.Batch(
		saveSessionCmd(m.sess),
		textinput.Blink,
	)
}

// resumeTimer resumes from a paused state, shifting EndTime forward to
// account for the time spent paused, and launching a new watcher.
func (m Model) resumeTimer() (Model, tea.Cmd) {
	if m.sess != nil && m.sess.Paused && !m.sess.PausedAt.IsZero() {
		pausedFor := time.Since(m.sess.PausedAt)
		m.sess.EndTime = m.sess.EndTime.Add(pausedFor)
		m.sess.Paused = false
		m.sess.PausedAt = time.Time{}
		m.tmr.Remaining = session.Remaining(m.sess)
	}
	m.tmr.Resume()
	if m.tmr.Phase == timer.PhaseBreak {
		m.mode = ModeTimerBreak
	} else {
		m.mode = ModeTimerFocus
	}
	return m, tea.Batch(
		tickCmd(),
		pollSessionCmd(),
		saveSessionCmd(m.sess),
		launchWatcherCmd(m.sess),
	)
}

// startTask begins a focus session for the given task ID.
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
			m.sess = sess

			return m, tea.Batch(
				saveCmd(m.store),
				saveSessionCmd(sess),
				launchWatcherCmd(sess),
				tea.Quit,
			)
		}
	}
	return m, nil
}

// ─── ModeTaskActions ──────────────────────────────────────────────────────────

// taskActionsOptions returns the ordered list of actions for the current state.
// 0 = Pause or Resume, 1 = Stop, 2 = Complete, 3 = Abandon
func (m Model) taskActionsOptionCount() int { return 4 }

func (m Model) updateTaskActions(key string) (tea.Model, tea.Cmd) {
	// Confirm sub-prompts: use confirm/close keys.
	switch m.actionsConfirm {
	case confirmComplete:
		switch {
		case matchKey(key, m.keys.confirm):
			return m.completeTask()
		case matchKey(key, m.keys.close):
			m.actionsConfirm = confirmNone
		}
		return m, nil

	case confirmAbandon:
		switch {
		case matchKey(key, m.keys.confirm):
			return m.abandonTask()
		case matchKey(key, m.keys.close):
			m.actionsConfirm = confirmNone
		}
		return m, nil
	}

	// Main navigable menu.
	switch {
	case matchKey(key, m.keys.up):
		if m.actionsCursor > 0 {
			m.actionsCursor--
		}
		return m, nil

	case matchKey(key, m.keys.down):
		if m.actionsCursor < m.taskActionsOptionCount()-1 {
			m.actionsCursor++
		}
		return m, nil

	case matchKey(key, m.keys.increase) || key == "+":
		m, cmd := m.adjustRunningTimer(+1)
		return m, cmd

	case matchKey(key, m.keys.decrease) || key == "-":
		m, cmd := m.adjustRunningTimer(-1)
		return m, cmd

	case matchKey(key, m.keys.confirm):
		return m.executeTaskAction(m.actionsCursor)

	case matchKey(key, m.keys.close):
		m.mode = ModeTaskList
	}

	return m, nil
}

func (m Model) executeTaskAction(idx int) (tea.Model, tea.Cmd) {
	switch idx {
	case 0: // Pause or Resume
		m.mode = ModeTaskList
		if m.tmr.State == timer.StatePaused {
			return m.resumeTimer()
		}
		return m.pauseTimer()
	case 1: // Stop
		m.mode = ModeTaskList
		return m.stopTask()
	case 2: // Complete
		m.actionsConfirm = confirmComplete
	case 3: // Abandon
		m.actionsConfirm = confirmAbandon
	}
	return m, nil
}

// completeTask marks the active task as completed and cleans up.
func (m Model) completeTask() (Model, tea.Cmd) {
	if m.activeTaskIdx >= 0 && m.activeTaskIdx < len(m.store.Tasks) {
		m.store.Tasks[m.activeTaskIdx].Completed = true
		if m.store.Tasks[m.activeTaskIdx].EndedAt.IsZero() {
			m.store.Tasks[m.activeTaskIdx].EndedAt = time.Now()
		}
	}
	killWatcher(m.sess)
	_ = session.Delete()
	m.sess = &session.Session{}

	taskName := ""
	if m.activeTaskIdx >= 0 && m.activeTaskIdx < len(m.store.Tasks) {
		taskName = m.store.Tasks[m.activeTaskIdx].Name
	}
	m.activeTaskIdx = -1

	if m.cfg.Display.ShowCompletionAnimation {
		m.mode = ModeCompletion
		m.completionFrame = 0
		m.completionTaskName = taskName
		return m, tea.Batch(saveCmd(m.store), completionTickCmd())
	}
	m.mode = ModeTaskList
	return m, saveCmd(m.store)
}

// abandonTask marks the active task as abandoned and cleans up.
func (m Model) abandonTask() (Model, tea.Cmd) {
	if m.activeTaskIdx >= 0 && m.activeTaskIdx < len(m.store.Tasks) {
		m.store.Tasks[m.activeTaskIdx].Abandoned = true
		if m.store.Tasks[m.activeTaskIdx].EndedAt.IsZero() {
			m.store.Tasks[m.activeTaskIdx].EndedAt = time.Now()
		}
	}
	killWatcher(m.sess)
	_ = session.Delete()
	m.sess = &session.Session{}
	m.activeTaskIdx = -1
	m.mode = ModeTaskList
	return m, saveCmd(m.store)
}

// ─── ModeEditTask ─────────────────────────────────────────────────────────────

func (m Model) beginEditTask(id string) Model {
	m.editingTaskID = id
	m.editField = fieldGroup
	m.editActive = false

	for i := range m.editInputs {
		m.editInputs[i].SetValue("")
		m.editInputs[i].Blur()
	}

	if id != "" {
		for _, t := range m.store.Tasks {
			if t.ID == id {
				if g := storage.FindGroup(m.store, t.GroupID); g != nil {
					m.editInputs[fieldGroup].SetValue(g.Name)
				}
				m.editInputs[fieldName].SetValue(t.Name)
				m.editInputs[fieldFocusTime].SetValue(itoa(t.FocusTime))
				m.editInputs[fieldBreakTime].SetValue(itoa(t.BreakTime))
				break
			}
		}
	}

	m.mode = ModeEditTask
	// Pre-populate group suggestions so they show immediately when the form opens.
	m.groupSuggestions = m.getGroupSuggestions(m.editInputs[fieldGroup].Value())
	return m
}

func (m Model) updateEditTask(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.editActive {
		switch {
		case matchKey(key, m.keys.confirm):
			m.editInputs[m.editField].Blur()
			m.editActive = false
			m.groupSuggestions = nil
			return m, nil
		case matchKey(key, m.keys.close):
			m.editInputs[m.editField].Blur()
			m.editActive = false
			m.groupSuggestions = nil
			return m, nil
		}
		var cmd tea.Cmd
		m.editInputs[m.editField], cmd = m.editInputs[m.editField].Update(msg)

		if m.editField == fieldGroup {
			m.groupSuggestions = m.getGroupSuggestions(m.editInputs[fieldGroup].Value())
		}
		return m, cmd
	}

	switch {
	case matchKey(key, m.keys.close):
		m.mode = ModeTaskList
		m.groupSuggestions = nil
		return m, nil

	case matchKey(key, m.keys.edit):
		m.editActive = true
		m.editInputs[m.editField].Focus()
		if m.editField == fieldGroup {
			m.groupSuggestions = m.getGroupSuggestions(m.editInputs[fieldGroup].Value())
		}
		var cmd tea.Cmd
		m.editInputs[m.editField], cmd = m.editInputs[m.editField].Update(msg)
		return m, tea.Batch(cmd, textinput.Blink)

	case matchKey(key, m.keys.confirm):
		m.groupSuggestions = nil
		return m.commitEditTask()

	case matchKey(key, m.keys.increase) || key == "+":
		m.adjustEditNumericField(+1)
		return m, nil

	case matchKey(key, m.keys.decrease) || key == "-":
		m.adjustEditNumericField(-1)
		return m, nil
	}

	if matchKey(key, m.keys.up) {
		if m.editField > 0 {
			m.editField--
		}
		m.groupSuggestions = m.groupSuggestionsForField()
		return m, nil
	}
	if matchKey(key, m.keys.down) {
		if m.editField < fieldCount-1 {
			m.editField++
		}
		m.groupSuggestions = m.groupSuggestionsForField()
		return m, nil
	}

	return m, nil
}

// adjustEditNumericField increments or decrements the value of the currently
// selected numeric field (FocusTime or BreakTime) by delta. No-op on text fields.
func (m *Model) adjustEditNumericField(delta int) {
	switch m.editField {
	case fieldFocusTime:
		v := parseIntClamped(m.editInputs[fieldFocusTime].Value(), 25, 0, 480)
		v += delta
		if v < 0 {
			v = 0
		}
		if v > 480 {
			v = 480
		}
		m.editInputs[fieldFocusTime].SetValue(itoa(v))
	case fieldBreakTime:
		v := parseIntClamped(m.editInputs[fieldBreakTime].Value(), 5, 0, 60)
		v += delta
		if v < 0 {
			v = 0
		}
		if v > 60 {
			v = 60
		}
		m.editInputs[fieldBreakTime].SetValue(itoa(v))
	}
}

func (m Model) commitEditTask() (tea.Model, tea.Cmd) {
	name := m.editInputs[fieldName].Value()
	if name == "" {
		m.statusMsg = "Task name is required"
		return m, clearStatusCmd()
	}

	focusMin := parseIntClamped(m.editInputs[fieldFocusTime].Value(), 25, 0, 480)
	breakMin := parseIntClamped(m.editInputs[fieldBreakTime].Value(), 5, 0, 60)

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

func (m Model) updateTimer(key string) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(key, m.keys.pause):
		return m.pauseTimer()

	case matchKey(key, m.keys.stop):
		return m.stopTask()

	case matchKey(key, m.keys.increase) || key == "+":
		m, cmd := m.adjustRunningTimer(+1)
		return m, tea.Batch(tickCmd(), cmd)

	case matchKey(key, m.keys.decrease) || key == "-":
		m, cmd := m.adjustRunningTimer(-1)
		return m, tea.Batch(tickCmd(), cmd)

	case matchKey(key, m.keys.close), key == "esc":
		// Leave timer running in background, return to task list.
		m.mode = ModeTaskList
		if m.sess != nil && session.IsActive(m.sess) {
			return m, launchWatcherCmd(m.sess)
		}
		return m, nil
	}

	return m, tickCmd()
}

// ─── ModePausePrompt ─────────────────────────────────────────────────────────

func (m Model) updatePausePrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch {
	case matchKey(key, m.keys.confirm):
		reason := m.pauseInput.Value()
		if reason != "" && m.activeTaskIdx >= 0 && m.activeTaskIdx < len(m.store.Tasks) {
			interrupt := storage.Interrupt{
				Time:   time.Now(),
				Reason: reason,
			}
			m.store.Tasks[m.activeTaskIdx].Interrupts = append(
				m.store.Tasks[m.activeTaskIdx].Interrupts, interrupt,
			)
			_ = storage.Save(m.store)
		}
		m.pauseInput.SetValue("")
		m.mode = ModeTaskList
		return m.resumeTimer()

	case matchKey(key, m.keys.close):
		m.pauseInput.SetValue("")
		m.mode = ModeTaskList
		return m.resumeTimer()
	}

	var cmd tea.Cmd
	m.pauseInput, cmd = m.pauseInput.Update(msg)
	return m, cmd
}

// ─── ModeBreakPrompt ─────────────────────────────────────────────────────────

// breakPromptInitCmd schedules the auto-start-break command if configured.
func (m Model) breakPromptInitCmd() tea.Cmd {
	debounce := time.Duration(m.cfg.Display.BreakPromptDebounce) * time.Second
	if debounce <= 0 {
		debounce = 0
	}
	if m.cfg.Display.AutoStartBreak && !m.afterBreak {
		// Auto-start break fires after debounce.
		return tea.Tick(debounce+time.Millisecond*100, func(_ time.Time) tea.Msg {
			return autoStartBreakMsg{}
		})
	}
	return nil
}

// breakPromptOptionCount returns the number of selectable options.
// Focus: 0=Break, 1=Extend, 2=Complete, 3=Abandon
// After break: 0=Complete, 1=Abandon
func (m Model) breakPromptOptionCount() int {
	if m.afterBreak {
		return 2
	}
	return 4
}

func (m Model) updateBreakPrompt(key string) (tea.Model, tea.Cmd) {
	debounce := time.Duration(m.cfg.Display.BreakPromptDebounce) * time.Second
	if debounce > 0 && time.Since(m.breakPromptEnteredAt) < debounce {
		return m, nil
	}

	m.autoBreakScheduled = false

	maxIdx := m.breakPromptOptionCount() - 1

	switch {
	case matchKey(key, m.keys.up):
		if m.breakPromptCursor > 0 {
			m.breakPromptCursor--
		}
		return m, nil

	case matchKey(key, m.keys.down):
		if m.breakPromptCursor < maxIdx {
			m.breakPromptCursor++
		}
		return m, nil

	case matchKey(key, m.keys.decrease) || key == "-":
		m.adjustBreakPromptValue(-1)
		return m, nil

	case matchKey(key, m.keys.increase) || key == "+":
		m.adjustBreakPromptValue(+1)
		return m, nil

	case matchKey(key, m.keys.confirm):
		return m.executeBreakPromptOption()
	}

	return m, nil
}

// adjustBreakPromptValue increments/decrements the adjustable field for the
// currently selected option (Extend or Break duration).
func (m *Model) adjustBreakPromptValue(delta int) {
	if m.afterBreak {
		return
	}
	switch m.breakPromptCursor {
	case 0: // Break duration
		m.breakDurationMins += delta
		if m.breakDurationMins < 1 {
			m.breakDurationMins = 1
		}
		if m.breakDurationMins > 60 {
			m.breakDurationMins = 60
		}
	case 1: // Extend minutes
		m.breakExtendMins += delta
		if m.breakExtendMins < 1 {
			m.breakExtendMins = 1
		}
		if m.breakExtendMins > 60 {
			m.breakExtendMins = 60
		}
	}
}

func (m Model) executeBreakPromptOption() (tea.Model, tea.Cmd) {
	if m.afterBreak {
		// 0=Complete, 1=Abandon
		if m.breakPromptCursor == 0 {
			return m.completeTask()
		}
		return m.abandonTask()
	}

	// Focus prompt: 0=Break, 1=Extend, 2=Complete, 3=Abandon
	switch m.breakPromptCursor {
	case 0:
		return m.startBreakWithDuration(m.breakDurationMins)
	case 1:
		m.tmr.Extend(m.breakExtendMins)
		endTime := time.Now().Add(m.tmr.Remaining)
		if m.sess == nil {
			m.sess = &session.Session{}
		}
		m.sess.EndTime = endTime
		m.sess.Phase = session.PhaseFocus
		m.afterBreak = false
		m.mode = ModeTimerFocus
		return m, tea.Batch(tickCmd(), pollSessionCmd(), saveSessionCmd(m.sess), launchWatcherCmd(m.sess))
	case 2:
		return m.completeTask()
	case 3:
		return m.abandonTask()
	}
	return m, nil
}

// taskBreakMin returns the task's configured break duration, falling back to 5.
func (m Model) taskBreakMin() int {
	if m.activeTaskIdx >= 0 && m.activeTaskIdx < len(m.store.Tasks) {
		return m.store.Tasks[m.activeTaskIdx].BreakTime
	}
	return 5
}

// startBreak transitions to a break timer using the task's default break duration.
func (m Model) startBreak() (Model, tea.Cmd) {
	return m.startBreakWithDuration(m.taskBreakMin())
}

// startBreakWithDuration transitions to a break timer with the given duration.
func (m Model) startBreakWithDuration(breakMin int) (Model, tea.Cmd) {
	if breakMin < 1 {
		breakMin = 1
	}
	m.tmr.StartBreak(breakMin)
	endTime := time.Now().Add(m.tmr.Remaining)
	if m.sess == nil {
		m.sess = &session.Session{}
	}
	m.sess.EndTime = endTime
	m.sess.Phase = session.PhaseBreak
	m.sess.BreakMin = breakMin
	m.sess.Paused = false
	m.afterBreak = false
	m.mode = ModeTimerBreak
	return m, tea.Batch(tickCmd(), pollSessionCmd(), saveSessionCmd(m.sess), launchWatcherCmd(m.sess))
}

// adjustRunningTimer adds deltaMins (positive or negative) minutes to the
// active running timer and persists the new end time to the session file.
// Returns the updated commands or nil if no active session.
func (m Model) adjustRunningTimer(deltaMins int) (Model, tea.Cmd) {
	if m.sess == nil || !session.IsActive(m.sess) {
		return m, nil
	}
	d := time.Duration(deltaMins) * time.Minute
	m.tmr.Remaining += d
	if m.tmr.Remaining < time.Second {
		m.tmr.Remaining = time.Second
	}
	m.sess.EndTime = time.Now().Add(m.tmr.Remaining)
	return m, tea.Batch(saveSessionCmd(m.sess), launchWatcherCmd(m.sess))
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

// ─── ModeCompleted ────────────────────────────────────────────────────────────

func (m Model) updateCompleted(key string) (tea.Model, tea.Cmd) {
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

func (m Model) OverlayWidget() string {
	return overlay.CornerWidget(m.cfg, m.sess, m.store)
}

// ─── Commands ─────────────────────────────────────────────────────────────────

type tickMsg struct{}
type sessionPollMsg struct{}
type completionTickMsg struct{}
type autoStartBreakMsg struct{}
type saveErrMsg string
type clearStatusMsg struct{}

// completionTotalFrames is how many animation frames to show (~2s at 10fps).
const completionTotalFrames = 20

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(_ time.Time) tea.Msg {
		return tickMsg{}
	})
}

func pollSessionCmd() tea.Cmd {
	return tea.Tick(time.Second, func(_ time.Time) tea.Msg {
		return sessionPollMsg{}
	})
}

func completionTickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(_ time.Time) tea.Msg {
		return completionTickMsg{}
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
		if sess != nil {
			_ = session.Save(sess)
		}
		return nil
	}
}

func clearStatusCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

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

// launchWatcherCmd kills any existing watcher, then starts a fresh
// ticky --watch subprocess that will notify when the timer fires.
// The WatchPID in the session is updated after the new process starts.
func launchWatcherCmd(sess *session.Session) tea.Cmd {
	return func() tea.Msg {
		if sess == nil {
			return nil
		}
		// Kill the old watcher before launching a new one.
		killWatcher(sess)
		sess.WatchPID = 0

		self := resolveSelf()
		if self == "" {
			return nil
		}
		cmd := exec.Command(self, "--watch")
		cmd.Stdin = nil
		cmd.Stdout = nil
		cmd.Stderr = nil
		setSysProcAttr(cmd)
		if err := cmd.Start(); err != nil {
			return nil
		}
		sess.WatchPID = cmd.Process.Pid
		_ = session.Save(sess)
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

func (m Model) ActiveTaskID() string {
	if m.activeTaskIdx < 0 || m.activeTaskIdx >= len(m.store.Tasks) {
		return ""
	}
	return m.store.Tasks[m.activeTaskIdx].ID
}

// BreakPromptDebounceRemaining returns how many full seconds remain in the
// debounce window, or 0 if the window has passed.
func (m Model) BreakPromptDebounceRemaining() int {
	debounce := time.Duration(m.cfg.Display.BreakPromptDebounce) * time.Second
	if debounce <= 0 {
		return 0
	}
	rem := debounce - time.Since(m.breakPromptEnteredAt)
	if rem <= 0 {
		return 0
	}
	return int(rem.Seconds()) + 1
}

// groupSuggestionsForField returns suggestions when the group field is selected
// (navigation mode), using the current field value as the query.
func (m Model) groupSuggestionsForField() []string {
	if m.editField != fieldGroup {
		return nil
	}
	return m.getGroupSuggestions(m.editInputs[fieldGroup].Value())
}

// getGroupSuggestions returns group names that fuzzy-match the input.
// When input is empty it returns all group names (up to the cap).
// Matching is case-insensitive; every character in input must appear in order
// somewhere in the group name (standard fuzzy / subsequence match).
func (m Model) getGroupSuggestions(input string) []string {
	const max = 6
	if len(m.store.Groups) == 0 {
		return nil
	}
	if input == "" {
		var all []string
		for _, g := range m.store.Groups {
			all = append(all, g.Name)
			if len(all) >= max {
				break
			}
		}
		return all
	}
	inputLower := strings.ToLower(input)
	var matches []string
	for _, g := range m.store.Groups {
		if fuzzyMatch(inputLower, strings.ToLower(g.Name)) {
			matches = append(matches, g.Name)
			if len(matches) >= max {
				break
			}
		}
	}
	return matches
}

// fuzzyMatch returns true when every rune in needle appears in haystack in order.
func fuzzyMatch(needle, haystack string) bool {
	hi := 0
	for _, n := range needle {
		found := false
		for hi < len(haystack) {
			r := rune(haystack[hi])
			hi++
			if r == n {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
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
