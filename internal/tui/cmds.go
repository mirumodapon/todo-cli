package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"todo.mirumo.net/internal/store"
	"todo.mirumo.net/internal/task"
)

// Every database action is wrapped in a tea.Cmd whose result returns to Update as a msg.
// Update itself performs no IO and stays pure, so tests only have to feed it messages.
type (
	tasksMsg   []task.Task
	errMsg     struct{ err error }
	savedMsg   struct{ note string }
	deletedMsg struct{ t task.Task }
	// editedMsg carries a task that came back from the user's editor.
	editedMsg struct{ t task.Task }
)

func (m Model) loadCmd() tea.Cmd {
	s, f, now := m.store, m.filter, m.now()
	return func() tea.Msg {
		ts, err := s.List(f, now)
		if err != nil {
			return errMsg{err}
		}
		return tasksMsg(ts)
	}
}

func (m Model) toggleCmd(t task.Task) tea.Cmd {
	s, now := m.store, m.now()
	return func() tea.Msg {
		if err := s.SetDone(t.ID, !t.Done(), now); err != nil {
			return errMsg{err}
		}
		note := `done "` + t.Title + `"`
		if t.Done() {
			note = `reopened "` + t.Title + `"`
		}
		return savedMsg{note: note}
	}
}

// deleteCmd fetches the whole task before deleting it: undo needs the tags too.
func (m Model) deleteCmd(t task.Task) tea.Cmd {
	s := m.store
	return func() tea.Msg {
		full, err := s.Get(t.ID)
		if err != nil {
			return errMsg{err}
		}
		if err := s.Delete(t.ID); err != nil {
			return errMsg{err}
		}
		return deletedMsg{t: full}
	}
}

func (m Model) restoreCmd(t task.Task) tea.Cmd {
	s := m.store
	return func() tea.Msg {
		if err := s.Restore(t); err != nil {
			return errMsg{err}
		}
		return savedMsg{note: `restored "` + t.Title + `"`}
	}
}

type (
	projectsMsg []store.ProjectCount
	tagsMsg     []string
)

func (m Model) projectsCmd() tea.Cmd {
	s := m.store
	return func() tea.Msg {
		ps, err := s.Projects()
		if err != nil {
			return errMsg{err}
		}
		return projectsMsg(ps)
	}
}

func (m Model) tagsCmd() tea.Cmd {
	s := m.store
	return func() tea.Msg {
		ts, err := s.Tags()
		if err != nil {
			return errMsg{err}
		}
		return tagsMsg(ts)
	}
}

func (m Model) saveCmd(t task.Task, editing bool) tea.Cmd {
	s := m.store
	return func() tea.Msg {
		if editing {
			if err := s.Update(t); err != nil {
				return errMsg{err}
			}
			return savedMsg{note: `updated "` + t.Title + `"`}
		}
		if _, err := s.Add(t); err != nil {
			return errMsg{err}
		}
		return savedMsg{note: `added "` + t.Title + `"`}
	}
}
