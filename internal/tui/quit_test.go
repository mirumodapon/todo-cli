package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// quits reports whether the model's pending command would end the program.
func quits(t *testing.T, m Model, cmd tea.Cmd) bool {
	t.Helper()
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

// q is the deliberate way out, and it asks.
func TestQuitAsksFirst(t *testing.T) {
	m, _ := newModel(t)
	next, cmd := m.Update(keyMsg("q"))
	m = next.(Model)
	if quits(t, m, cmd) {
		t.Fatal("q should not quit on its own")
	}
	if m.mode != modeConfirm {
		t.Fatalf("q should ask, mode = %v", m.mode)
	}
	if !strings.Contains(m.View(), "Quit?") {
		t.Errorf("the question should be on screen:\n%s", m.View())
	}
	_, cmd = m.Update(keyMsg("y"))
	if !quits(t, m, cmd) {
		t.Error("y should quit")
	}
}

func TestAnyOtherKeyStaysInTheProgram(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "q")
	next, cmd := m.Update(keyMsg("n"))
	if quits(t, next.(Model), cmd) {
		t.Fatal("n should not quit")
	}
	if next.(Model).mode != modeList {
		t.Errorf("it should go back to the list, mode = %v", next.(Model).mode)
	}
}

// ctrl+d is the shell's "nothing more to type", which only means that where
// there is nothing being typed. In a field it stays a text key.
func TestCtrlDIsTextInAField(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "/")
	next, _ := m.Update(keyMsg("ctrl+d"))
	if next.(Model).mode != modeSearch {
		t.Errorf("ctrl+d should not ask to quit while typing, mode = %v", next.(Model).mode)
	}
}

// A question already on screen is answered first: ctrl+c cancels it rather than
// stacking another question on top.
func TestCtrlCCancelsAPendingQuestion(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "d") // delete asks first
	if m.mode != modeConfirm {
		t.Fatal("d should ask")
	}
	next, cmd := m.Update(keyMsg("ctrl+c"))
	m = next.(Model)
	if quits(t, m, cmd) {
		t.Error("ctrl+c should not quit out of a pending question")
	}
	if m.mode != modeList {
		t.Errorf("it should cancel the delete, mode = %v", m.mode)
	}
	if len(m.tasks) != 3 {
		t.Errorf("and delete nothing, got %d tasks", len(m.tasks))
	}
}
