package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// confirmState is a pending action waiting on a yes/no answer.
// action is the tea.Cmd to run once the user says yes, and back is the mode to
// return to on either answer: asking is not leaving, so a form that was open
// when the question came up is still open after it.
type confirmState struct {
	prompt string
	action tea.Cmd
	back   mode
}

// askConfirm arms a confirmation and switches to it. The action does not run
// until the user answers yes, so nothing reaches the database before then.
func (m Model) askConfirm(prompt string, action tea.Cmd) Model {
	m.confirm = confirmState{prompt: prompt, action: action, back: m.mode}
	m.mode = modeConfirm
	m.status = ""
	return m
}

// askQuit puts the way out behind the same question as everything else that
// cannot be taken back. Leaving is a decision, not a keystroke.
func (m Model) askQuit() Model {
	return m.askConfirm("Quit? (y/n)", tea.Quit)
}

// updateConfirm answers the pending question. Only y accepts; every other key
// cancels, so a mistyped key can never confirm a destructive action.
func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	action, back := m.confirm.action, m.confirm.back
	m.confirm = confirmState{}
	m.mode = back
	switch msg.String() {
	case "y", "Y":
		return m, action
	}
	m.status = "cancelled"
	return m, nil
}
