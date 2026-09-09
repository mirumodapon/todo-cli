package tui

import (
	"strings"
	"testing"
)

// ctrl+c is a reflex, not a decision. A modal question in its way is one more
// thing to read at the moment you have already decided to go; the second press
// is the confirmation.
func TestCtrlCQuitsOnTheSecondPress(t *testing.T) {
	for _, key := range []string{"ctrl+c", "ctrl+d"} {
		t.Run(key, func(t *testing.T) {
			m, _ := newModel(t)
			next, cmd := m.Update(keyMsg(key))
			m = next.(Model)
			if quits(t, m, cmd) {
				t.Fatalf("%s should not quit on the first press", key)
			}
			if m.mode != modeList {
				t.Fatalf("and should not open a question, mode = %v", m.mode)
			}
			if !strings.Contains(m.View(), "again") {
				t.Errorf("the hint should say what a second press does:\n%s", m.View())
			}
			_, cmd = m.Update(keyMsg(key))
			if !quits(t, m, cmd) {
				t.Errorf("%s twice should quit", key)
			}
		})
	}
}

// Either key finishes what the other started: they are the same door.
func TestCtrlDThenCtrlCQuits(t *testing.T) {
	m, _ := newModel(t)
	next, _ := m.Update(keyMsg("ctrl+d"))
	_, cmd := next.(Model).Update(keyMsg("ctrl+c"))
	if !quits(t, m, cmd) {
		t.Error("the second press should quit whichever key it was")
	}
}

// Anything in between breaks the sequence: a ctrl+c from five minutes ago must
// not turn an unrelated keystroke into an exit.
func TestAnotherKeyDisarmsIt(t *testing.T) {
	m, _ := newModel(t)
	next, _ := m.Update(keyMsg("ctrl+c"))
	m = press(t, next.(Model), "j")
	if strings.Contains(m.View(), "again") {
		t.Errorf("the hint should be gone:\n%s", m.View())
	}
	_, cmd := m.Update(keyMsg("ctrl+c"))
	if quits(t, m, cmd) {
		t.Error("the sequence was broken, so this is a first press")
	}
}

// It works from anywhere, and what was open is still open while it waits.
func TestCtrlCArmsFromAForm(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "a")
	m = press(t, m, "half typed")
	next, _ := m.Update(keyMsg("ctrl+c"))
	m = next.(Model)
	if m.mode != modeForm {
		t.Fatalf("the form should still be open, mode = %v", m.mode)
	}
	v := m.View()
	if !strings.Contains(v, "half typed") || !strings.Contains(v, "again") {
		t.Errorf("the form and the warning should share the screen:\n%s", v)
	}
	_, cmd := m.Update(keyMsg("ctrl+c"))
	if !quits(t, m, cmd) {
		t.Error("the second press should quit from a form too")
	}
}

// q is a decision rather than a reflex, so it keeps its question.
func TestQStillAsks(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "q")
	if m.mode != modeConfirm {
		t.Fatalf("q should still ask, mode = %v", m.mode)
	}
	if !strings.Contains(m.View(), "Quit?") {
		t.Errorf("with the question on screen:\n%s", m.View())
	}
}
