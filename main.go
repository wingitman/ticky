package main

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	s "main.go/Structs"
	"os"
)

type Model s.Model
type Task s.Task
type Timer s.Timer

func (m Model) Init() tea.Cmd {
	return nil
}
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case m.Keys.Close:
			return m, tea.Quit
		}
	}
	return m, nil
}
func (m Model) View() tea.View {
	return tea.NewView("Hello world!")
}

func main() {
	fmt.Println("init")

	m := Model(s.Default())
	p := tea.NewProgram(m)

	rM, err := p.Run()
	if err != nil {
		os.Exit(1)
	}

	fmt.Println(rM)

	os.Exit(0)
}
