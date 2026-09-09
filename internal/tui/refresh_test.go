package tui

import (
	"strings"
	"testing"

	"todo.mirumo.net/internal/task"
)

// The CLI and the MCP server write to the same database while the interface is
// open, so there has to be a way to see that without restarting it.
func TestRRereadsTheDatabase(t *testing.T) {
	m, s := newModel(t)
	before := len(m.tasks)

	if _, err := s.Add(task.Task{Title: "added elsewhere", CreatedAt: refTime(), UpdatedAt: refTime()}); err != nil {
		t.Fatal(err)
	}
	if len(m.tasks) != before {
		t.Fatal("the interface should not have noticed on its own")
	}

	m = press(t, m, "R")
	if len(m.tasks) != before+1 {
		t.Fatalf("R should reread the database, got %d tasks", len(m.tasks))
	}
	if !strings.Contains(m.View(), "added elsewhere") {
		t.Errorf("the new task should be on screen:\n%s", m.View())
	}
	// A reload that changed nothing visible looks like a key that did nothing.
	if !strings.Contains(m.View(), "reloaded") {
		t.Errorf("it should say it happened:\n%s", m.View())
	}
}

func TestRKeepsTheFilterAndTheCursor(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "j")
	m = press(t, m, "A")
	m = press(t, m, "R")
	if m.cursor != 1 {
		t.Errorf("cursor = %d, a reload should not move it", m.cursor)
	}
	if !m.filter.IncludeDone {
		t.Error("a reload should not drop the filter")
	}
}
