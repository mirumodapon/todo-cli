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

// Leaving is a decision, not a keystroke: every way out asks first.
func TestQuitAsksFirst(t *testing.T) {
	for _, key := range []string{"q", "ctrl+c", "ctrl+d"} {
		t.Run(key, func(t *testing.T) {
			m, _ := newModel(t)
			next, cmd := m.Update(keyMsg(key))
			m = next.(Model)
			if quits(t, m, cmd) {
				t.Fatalf("%s should not quit on its own", key)
			}
			if m.mode != modeConfirm {
				t.Fatalf("%s should ask, mode = %v", key, m.mode)
			}
			if !strings.Contains(m.View(), "Quit?") {
				t.Errorf("the question should be on screen:\n%s", m.View())
			}
			_, cmd = m.Update(keyMsg("y"))
			if !quits(t, m, cmd) {
				t.Errorf("y should quit")
			}
		})
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

// ctrl+c is never text, so it reaches for the door from anywhere — and
// cancelling puts back what was open, since asking is not leaving.
func TestCtrlCAsksFromAForm(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "a")
	m = press(t, m, "half typed")
	next, _ := m.Update(keyMsg("ctrl+c"))
	m = next.(Model)
	if m.mode != modeConfirm {
		t.Fatalf("ctrl+c should ask from a form, mode = %v", m.mode)
	}
	// The question is asked at the bottom of the form, not instead of it.
	if v := m.View(); !strings.Contains(v, "half typed") || !strings.Contains(v, "Quit?") {
		t.Errorf("the form should still be on screen under the question:\n%s", v)
	}
	m = press(t, m, "n")
	if m.mode != modeForm {
		t.Fatalf("cancelling should put the form back, mode = %v", m.mode)
	}
	if !strings.Contains(m.View(), "half typed") {
		t.Errorf("with what was typed into it:\n%s", m.View())
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
