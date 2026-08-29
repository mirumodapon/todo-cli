package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// confirmState is a pending action waiting on a yes/no answer.
// action is the tea.Cmd to run once the user says yes.
type confirmState struct {
	prompt string
	action tea.Cmd
}

// askConfirm arms a confirmation and switches to it. The action does not run
// until the user answers yes, so nothing reaches the database before then.
func (m Model) askConfirm(prompt string, action tea.Cmd) Model {
	m.confirm = confirmState{prompt: prompt, action: action}
	m.mode = modeConfirm
	m.status = ""
	return m
}

// updateConfirm answers the pending question. Only y accepts; every other key
// cancels, so a mistyped key can never confirm a destructive action.
func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	action := m.confirm.action
	m.confirm = confirmState{}
	m.mode = modeList
	switch msg.String() {
	case "y", "Y":
		return m, action
	}
	m.status = "已取消"
	return m, nil
}
