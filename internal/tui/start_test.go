package tui

import (
	"strings"
	"testing"

	"todo.mirumo.net/internal/task"
)

// workStart is what "task tui -p /p/work" hands the interface.
func workStart() Start {
	work := "/p/work"
	return Start{Filter: task.Filter{Project: &work}}
}

func TestOpensOnTheFilterItWasGiven(t *testing.T) {
	m, _ := newModelStart(t, workStart())
	if len(m.tasks) != 1 || m.tasks[0].Title != "work one" {
		t.Fatalf("it should open on the project's tasks, got %d", len(m.tasks))
	}
	if !strings.Contains(m.View(), "work") {
		t.Errorf("the header should name the scope it opened on:\n%s", m.View())
	}
}

// esc returns to the filter the command line asked for. What you typed to open
// the interface is this session's default, not a starting point it forgets.
func TestEscReturnsToTheFilterItOpenedOn(t *testing.T) {
	m, _ := newModelStart(t, workStart())
	m = press(t, m, "A")
	if !m.filter.IncludeDone {
		t.Fatal("A should include done tasks")
	}
	m = press(t, m, "esc")
	if m.filter.Project == nil || *m.filter.Project != "/p/work" {
		t.Errorf("project = %v, want the one it opened on", m.filter.Project)
	}
	if m.filter.IncludeDone {
		t.Error("esc should drop what was toggled since")
	}
}

func TestDatesCanStartOn(t *testing.T) {
	start := DefaultStart()
	start.Dates = true
	m, _ := newModelStart(t, start)
	if !m.dates {
		t.Fatal("--dates should start the interface on calendar dates")
	}
	// D still toggles from there rather than being overridden by the flag.
	m = press(t, m, "D")
	if m.dates {
		t.Error("D should turn it back off")
	}
}
