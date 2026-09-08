package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"todo.mirumo.net/internal/store"
	"todo.mirumo.net/internal/task"
)

// Every database action is wrapped in a tea.Cmd whose result returns to Update as a msg.
// Update itself performs no IO and stays pure, so tests only have to feed it messages.
type (
	// tasksMsg is a fresh list. anchor is the task the cursor should stay on;
	// zero leaves the cursor on its row, which is what a new filter wants.
	tasksMsg struct {
		tasks  []task.Task
		anchor int64
	}
	errMsg     struct{ err error }
	savedMsg   struct{ note string }
	deletedMsg struct{ t task.Task }
	// editedMsg carries a task that came back from the user's editor.
	editedMsg struct{ t task.Task }
)

// loadCmd rereads the list, leaving the cursor on its row. That is what a
// changed filter wants: a new view starts where the caller put it.
func (m Model) loadCmd() tea.Cmd { return m.loadAnchored(0) }

// reloadCmd rereads the list and keeps the cursor on the task it is on. Used
// wherever the tasks are the same ones in a different shape — after a save, a
// sort, or a write by something else — because a row number then points at
// whatever moved into that row.
func (m Model) reloadCmd() tea.Cmd {
	var anchor int64
	if t, ok := m.current(); ok {
		anchor = t.ID
	}
	return m.loadAnchored(anchor)
}

func (m Model) loadAnchored(anchor int64) tea.Cmd {
	s, f, now := m.store, m.filter, m.now()
	return func() tea.Msg {
		ts, err := s.List(f, now)
		if err != nil {
			return errMsg{err}
		}
		return tasksMsg{tasks: ts, anchor: anchor}
	}
}

// tickMsg asks whether anything has changed underneath; versionMsg is the answer.
type tickMsg struct{}

type versionMsg struct {
	version int64
	err     error
}

// tickCmd arms the next poll. A zero interval turns polling off, which is what
// the tests use: they feed the messages by hand rather than wait on a clock.
func (m Model) tickCmd() tea.Cmd {
	if m.poll <= 0 {
		return nil
	}
	return tea.Tick(m.poll, func(time.Time) tea.Msg { return tickMsg{} })
}

// versionCmd asks the database whether another process has written. It reads one
// integer, so asking often costs nothing worth counting.
func (m Model) versionCmd() tea.Cmd {
	s := m.store
	return func() tea.Msg {
		v, err := s.DataVersion()
		return versionMsg{version: v, err: err}
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
