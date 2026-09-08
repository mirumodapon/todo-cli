package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"todo.mirumo.net/internal/task"
)

// suspends reports whether a key hands the terminal back to the shell.
func suspends(t *testing.T, m Model) bool {
	t.Helper()
	next, cmd := m.Update(key("ctrl+z"))
	_ = next
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.SuspendMsg)
	return ok
}

// ctrl+z is job control, not a binding: it belongs to the terminal and has to
// work wherever you happen to be when you reach for it.
func TestCtrlZSuspendsFromAnyMode(t *testing.T) {
	base, _ := newModel(t)
	for _, c := range []struct{ name, key string }{
		{"the list", ""},
		{"a form", "a"},
		{"a search", "/"},
		{"a confirmation", " "},
		{"the help", "?"},
		{"a detail view", "enter"},
		{"a menu", "P"},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := base
			if c.key != "" {
				m = press(t, m, c.key)
			}
			if !suspends(t, m) {
				t.Errorf("ctrl+z should suspend from %s", c.name)
			}
		})
	}
}

// Suspending is not cancelling: what you were in the middle of is still there
// when you come back.
func TestSuspendingLeavesTheModeAlone(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, " ") // a confirmation is armed
	next, _ := m.Update(key("ctrl+z"))
	if next.(Model).mode != modeConfirm {
		t.Errorf("mode = %v, the prompt should still be waiting", next.(Model).mode)
	}
}

// Coming back from the shell is exactly when the database has changed: that is
// what people go to the shell for.
func TestResumingRereadsTheDatabase(t *testing.T) {
	m, s := newModel(t)
	before := len(m.tasks)
	if _, err := s.Add(task.Task{Title: "added while away", CreatedAt: refTime(), UpdatedAt: refTime()}); err != nil {
		t.Fatal(err)
	}
	m = feed(t, m, tea.ResumeMsg{})
	if len(m.tasks) != before+1 {
		t.Errorf("resuming should reread, got %d tasks", len(m.tasks))
	}
}
