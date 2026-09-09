package tui

import (
	"strings"
	"testing"

	"todo.mirumo.net/internal/task"
)

func TestSOpensASortMenu(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "s")
	if m.mode != modePicker {
		t.Fatalf("s should open the menu, mode = %v", m.mode)
	}
	v := m.View()
	for _, want := range []string{"id", "due date", "priority", "created"} {
		if !strings.Contains(v, want) {
			t.Errorf("the menu is missing %q:\n%s", want, v)
		}
	}
	// Row 0 is id, the one it is already on; row 1 is due date.
	m = press(t, m, "j")
	m = press(t, m, "enter")
	if m.mode != modeList {
		t.Fatalf("enter should return to the list, mode = %v", m.mode)
	}
	if m.filter.Sort != task.SortDue {
		t.Errorf("sort = %v, want due", m.filter.Sort)
	}
	if m.tasks[0].Title != "first" {
		t.Errorf("the list should be in due order, got %q first", m.tasks[0].Title)
	}
}

// Picking an order does not move you off the task you were reading.
func TestPickingAnOrderKeepsTheCursorOnItsTask(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "G")
	on, _ := m.current()
	m = press(t, m, "s")
	m = press(t, m, "j")
	m = press(t, m, "enter")
	if got, ok := m.current(); !ok || got.ID != on.ID {
		t.Errorf("the cursor left %q for %+v", on.Title, got)
	}
}

func TestRReversesTheOrder(t *testing.T) {
	m, _ := newModel(t)
	first := m.tasks[0].ID
	m = press(t, m, "r")
	if !m.filter.Reverse {
		t.Fatal("r should reverse the order")
	}
	if m.tasks[0].ID == first {
		t.Errorf("the list should have turned around: %v", m.tasks)
	}
	m = press(t, m, "r")
	if m.filter.Reverse || m.tasks[0].ID != first {
		t.Error("r again should turn it back")
	}
}

// Eight orderings is more than a status line can be trusted to remember, so the
// header carries the current one: the order is not invisible state either.
func TestTheHeaderNamesTheOrder(t *testing.T) {
	m, _ := newModel(t)
	if !strings.Contains(m.header(), "id") {
		t.Errorf("header = %q, it should name the order", m.header())
	}
	m = press(t, m, "r")
	if !strings.Contains(m.header(), "↓") {
		t.Errorf("header = %q, it should show which way it points", m.header())
	}
	m = press(t, m, "r")
	if !strings.Contains(m.header(), "↑") {
		t.Errorf("header = %q", m.header())
	}
}

// esc puts the order back with the rest of the filter.
func TestEscRestoresTheOrder(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "r")
	m = press(t, m, "esc")
	if m.filter.Reverse {
		t.Error("esc should return to the order it opened on")
	}
}
