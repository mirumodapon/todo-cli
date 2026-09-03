package tui

import "testing"

// ctrl+n and ctrl+p are the emacs and readline motions. They matter most in
// the modes that hold a text input, where j and k are literal characters.

func TestCtrlNAndCtrlPMoveTheList(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "ctrl+n")
	if m.cursor != 1 {
		t.Errorf("ctrl+n should move down, cursor = %d", m.cursor)
	}
	m = press(t, m, "ctrl+n")
	m = press(t, m, "ctrl+p")
	if m.cursor != 1 {
		t.Errorf("ctrl+p should move up, cursor = %d", m.cursor)
	}
}

func TestCtrlNAndCtrlPMoveWhileSearching(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "/")
	m = press(t, m, "ctrl+n")
	if m.mode != modeSearch {
		t.Fatalf("moving must not leave the search, mode = %v", m.mode)
	}
	if m.cursor != 1 {
		t.Errorf("ctrl+n should move the list while typing, cursor = %d", m.cursor)
	}
	if v := m.search.Value(); v != "" {
		t.Errorf("the motion must not be typed into the field, value = %q", v)
	}
	m = press(t, m, "ctrl+p")
	if m.cursor != 0 {
		t.Errorf("ctrl+p should move back up while typing, cursor = %d", m.cursor)
	}
}

func TestSearchingKeepsTheCursorInRange(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "/")
	m = press(t, m, "ctrl+n")
	m = press(t, m, "ctrl+n")
	// Narrowing to one result must not leave the cursor pointing past the end.
	m = press(t, m, "first")
	if m.cursor >= len(m.tasks) {
		t.Errorf("cursor = %d with %d tasks", m.cursor, len(m.tasks))
	}
}

func TestCtrlNAndCtrlPMoveThePicker(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "P")
	if m.mode != modePicker {
		t.Fatal("P should open the project picker")
	}
	m = press(t, m, "ctrl+n")
	if m.picker.cursor != 1 {
		t.Errorf("ctrl+n should move down the menu, cursor = %d", m.picker.cursor)
	}
	m = press(t, m, "ctrl+p")
	if m.picker.cursor != 0 {
		t.Errorf("ctrl+p should move up the menu, cursor = %d", m.picker.cursor)
	}
}

func TestCtrlNAndCtrlPMoveBetweenFormFields(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "a")
	m = press(t, m, "ctrl+n")
	if m.form.focus != fieldProject {
		t.Errorf("ctrl+n should move to the next field, focus = %d", m.form.focus)
	}
	m = press(t, m, "ctrl+p")
	if m.form.focus != fieldTitle {
		t.Errorf("ctrl+p should move to the previous field, focus = %d", m.form.focus)
	}
	if v := m.form.inputs[fieldTitle].Value(); v != "" {
		t.Errorf("the motions must not be typed into the field, value = %q", v)
	}
}
