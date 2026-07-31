//go:build desktop

// Package desktop provides ticky's optional native window frontend.
package desktop

import (
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"sort"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/wingitman/ticky/internal/session"
	"github.com/wingitman/ticky/internal/storage"
)

const (
	windowWidth  = 960
	windowHeight = 640
)

var (
	background = color.RGBA{R: 24, G: 24, B: 42, A: 255}
	panel      = color.RGBA{R: 30, G: 30, B: 58, A: 255}
	accent     = color.RGBA{R: 124, G: 158, B: 240, A: 255}
	button     = color.RGBA{R: 42, G: 42, B: 74, A: 255}
	green      = color.RGBA{R: 124, G: 240, B: 156, A: 255}
	orange     = color.RGBA{R: 240, G: 164, B: 124, A: 255}
)

// Run opens the detached desktop window and blocks until it closes.
func Run() error {
	ebiten.SetWindowSize(windowWidth, windowHeight)
	ebiten.SetWindowTitle("delbysoft / ticky")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	return ebiten.RunGame(newGame())
}

type game struct {
	store   *storage.Store
	sess    *session.Session
	compact bool
	cursor  int
	width   int
	height  int
	rowY    []int
}

func newGame() *game {
	store, _ := storage.Load()
	sess, _ := session.Load()
	return &game{store: store, sess: sess, width: windowWidth, height: windowHeight}
}

func (g *game) Update() error {
	g.reload()
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyQ) {
		return ebiten.Termination
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyW) {
		g.compact = !g.compact
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) && g.cursor > 0 {
		g.cursor--
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) && g.cursor < len(g.tasks())-1 {
		g.cursor++
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		g.startSelected()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyP) {
		g.pause()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyX) {
		g.stop()
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		g.click(x, y)
	}
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	screen.Fill(background)
	if g.compact {
		g.drawWidget(screen)
		return
	}
	g.drawFull(screen)
}

func (g *game) Layout(width, height int) (int, int) {
	g.width, g.height = width, height
	return width, height
}

func (g *game) reload() {
	if store, err := storage.Load(); err == nil {
		g.store = store
	}
	if sess, err := session.Load(); err == nil {
		g.sess = sess
	}
	if count := len(g.tasks()); count == 0 {
		g.cursor = 0
	} else if g.cursor >= count {
		g.cursor = count - 1
	}
}

func (g *game) drawFull(screen *ebiten.Image) {
	ebitenutil.DrawRect(screen, 0, 0, float64(g.width), 42, panel)
	ebitenutil.DebugPrintAt(screen, "delbysoft / ticky", 18, 14)
	ebitenutil.DebugPrintAt(screen, "desktop", max(20, g.width-100), 14)

	tasks := g.tasks()
	y := 68
	g.rowY = g.rowY[:0]
	if len(tasks) == 0 {
		ebitenutil.DebugPrintAt(screen, "No tasks yet. Create one in the TUI.", 22, y)
	}
	lastGroup := ""
	for i, task := range tasks {
		group := g.groupName(task.GroupID)
		if group != lastGroup {
			lastGroup = group
			label := group
			if label == "" {
				label = "ungrouped"
			}
			ebitenutil.DebugPrintAt(screen, "-- "+label+" --", 22, y)
			y += 24
		}
		if i == g.cursor {
			ebitenutil.DrawRect(screen, 12, float64(y-4), float64(max(1, g.width-24)), 25, button)
		}
		g.rowY = append(g.rowY, y)
		ebitenutil.DebugPrintAt(screen, task.Name, 24, y)
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%dm / %dm", task.FocusTime, task.BreakTime), max(300, g.width/2), y)
		y += 28
		if y > g.height-110 {
			break
		}
	}

	if g.activeTask() != nil {
		task := g.activeTask()
		ebitenutil.DrawRect(screen, 12, float64(g.height-94), float64(max(1, g.width-24)), 42, panel)
		ebitenutil.DebugPrintAt(screen, task.Name+"  "+g.remaining(), 24, g.height-78)
	}
	g.drawButtons(screen, g.height-42)
	ebitenutil.DebugPrintAt(screen, "up/down select   enter start   w widget   q close", 18, g.height-18)
}

func (g *game) drawWidget(screen *ebiten.Image) {
	ebitenutil.DrawRect(screen, 0, 0, float64(g.width), float64(g.height), panel)
	task := g.activeTask()
	if task == nil {
		ebitenutil.DebugPrintAt(screen, "ticky idle", 20, 22)
	} else {
		ebitenutil.DebugPrintAt(screen, task.Name, 20, 18)
		ebitenutil.DebugPrintAt(screen, g.groupName(task.GroupID)+"  "+g.remaining(), 20, 42)
	}
	g.drawButtons(screen, g.height-34)
	ebitenutil.DebugPrintAt(screen, "w full view", 20, g.height-10)
}

func (g *game) drawButtons(screen *ebiten.Image, y int) {
	buttons := []struct {
		label string
		color color.Color
	}{
		{"Pause (P)", orange},
		{"Resume (P)", green},
		{"Stop (X)", color.RGBA{R: 240, G: 124, B: 124, A: 255}},
		{"Widget (W)", accent},
	}
	if g.sess != nil && g.sess.Paused {
		buttons = buttons[1:]
	} else {
		buttons = append(buttons[:1], buttons[2:]...)
	}
	x := 18
	for _, b := range buttons {
		ebitenutil.DrawRect(screen, float64(x), float64(y), 118, 26, b.color)
		ebitenutil.DebugPrintAt(screen, b.label, x+8, y+8)
		x += 130
	}
}

func (g *game) click(x, y int) {
	if y >= g.height-45 {
		switch {
		case x < 130:
			g.pause()
		case x < 260:
			g.stop()
		case x < 390:
			g.compact = !g.compact
		}
		return
	}
	if !g.compact && y >= 60 && y < g.height-110 {
		for i, row := range g.rowY {
			if y >= row-6 && y <= row+22 {
				g.cursor = i
				break
			}
		}
	}
}

func (g *game) tasks() []storage.Task {
	if g.store == nil {
		return nil
	}
	tasks := storage.ActiveTasks(g.store)
	order := make(map[string]int, len(g.store.Groups))
	for i, group := range g.store.Groups {
		order[group.ID] = i
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		left, okLeft := order[tasks[i].GroupID]
		right, okRight := order[tasks[j].GroupID]
		if !okLeft {
			left = len(order)
		}
		if !okRight {
			right = len(order)
		}
		return left < right
	})
	return tasks
}

func (g *game) activeTask() *storage.Task {
	if g.sess == nil || g.sess.TaskID == "" || g.store == nil {
		return nil
	}
	for i := range g.store.Tasks {
		if g.store.Tasks[i].ID == g.sess.TaskID && !g.store.Tasks[i].Completed && !g.store.Tasks[i].Abandoned {
			return &g.store.Tasks[i]
		}
	}
	return nil
}

func (g *game) remaining() string {
	if g.sess == nil || g.sess.TaskID == "" {
		return "idle"
	}
	remaining := session.Remaining(g.sess)
	if remaining < 0 {
		remaining = 0
	}
	return fmt.Sprintf("%02d:%02d", int(remaining/time.Minute), int(remaining/time.Second)%60)
}

func (g *game) groupName(id string) string {
	if g.store == nil {
		return ""
	}
	if group := storage.FindGroup(g.store, id); group != nil {
		return group.Name
	}
	return ""
}

func (g *game) startSelected() {
	tasks := g.tasks()
	if g.cursor < 0 || g.cursor >= len(tasks) || g.sess != nil && g.sess.TaskID != "" {
		return
	}
	task := tasks[g.cursor]
	now := time.Now()
	for i := range g.store.Tasks {
		if g.store.Tasks[i].ID == task.ID {
			g.store.Tasks[i].StartedAt = now
		}
	}
	g.sess = &session.Session{TaskID: task.ID, EndTime: now.Add(time.Duration(task.FocusTime) * time.Minute), Phase: session.PhaseFocus, BreakMin: task.BreakTime}
	_ = storage.Save(g.store)
	_ = session.Save(g.sess)
	g.launchWatcher()
}

func (g *game) pause() {
	if g.sess == nil || g.sess.TaskID == "" {
		return
	}
	if g.sess.Paused {
		if !g.sess.PausedAt.IsZero() {
			g.sess.EndTime = g.sess.EndTime.Add(time.Since(g.sess.PausedAt))
		}
		g.sess.Paused = false
		g.sess.PausedAt = time.Time{}
		_ = session.Save(g.sess)
		g.launchWatcher()
		return
	}
	g.sess.Paused = true
	g.sess.PausedAt = time.Now()
	_ = session.Save(g.sess)
}

func (g *game) launchWatcher() {
	self, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(self, "--watch")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Start()
}

func (g *game) stop() {
	if g.sess == nil || g.sess.TaskID == "" {
		return
	}
	for i := range g.store.Tasks {
		if g.store.Tasks[i].ID == g.sess.TaskID {
			g.store.Tasks[i].StartedAt = time.Time{}
			g.store.Tasks[i].EndedAt = time.Time{}
			g.store.Tasks[i].Interrupts = nil
		}
	}
	_ = storage.Save(g.store)
	_ = session.Delete()
	g.sess = &session.Session{}
}
