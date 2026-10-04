package structs

import "time"

type Model struct {
	Keys KeyBinds
}

type Task struct {
	Group string
	Name  string
}

type Timer struct {
	StartTime   time.Time
	EndTime     time.Time
	StoppedTime time.Time
	State       eState
}

type eState int

const (
	New eState = iota
	Active
	Stopped
)

type KeyBinds struct {
	Up          string
	Down        string
	Left        string
	Right       string
	Edit        string
	Confirm     string
	Start       string
	Close       string
	Format      string
	Options     string
	Pause       string
	Stop        string
	New         string
	Delete      string
	Group       string
	Report      string
	Completed   string
	ShowUpdates string
	Theme       string
}

func Default() Model {
	return Model{
		Keys: KeyBinds{
			Up:          "k",
			Down:        "j",
			Left:        "h",
			Right:       "l",
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
			Theme:       "T",
		},
	}
}
