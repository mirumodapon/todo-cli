package tui

import (
	"strings"
	"testing"
)

func TestADigitBeforeJMovesThatFar(t *testing.T) {
	m, _ := newModel(t)
	m = bulk(t, m, 12)
	m = press(t, m, "5")
	if !strings.Contains(m.View(), "5") {
		t.Errorf("a count being typed should be on screen:\n%s", m.View())
	}
	m = press(t, m, "j")
	if m.cursor != 5 {
		t.Fatalf("cursor = %d, want 5", m.cursor)
	}
	if m.count != "" {
		t.Errorf("the count should be spent, got %q", m.count)
	}
	m = press(t, m, "2")
	m = press(t, m, "k")
	if m.cursor != 3 {
		t.Errorf("cursor = %d, want 3", m.cursor)
	}
}

func TestATwoDigitCount(t *testing.T) {
	m, _ := newModel(t)
	m = bulk(t, m, 20)
	m = press(t, m, "1")
	m = press(t, m, "2")
	m = press(t, m, "j")
	if m.cursor != 12 {
		t.Errorf("cursor = %d, want 12", m.cursor)
	}
}

// Counting past the end stops at the end rather than doing nothing.
func TestACountBeyondTheListStopsAtTheEnd(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "9")
	m = press(t, m, "j")
	if m.cursor != len(m.tasks)-1 {
		t.Errorf("cursor = %d, want the last row %d", m.cursor, len(m.tasks)-1)
	}
}

// A number then enter goes to that task by id, which is the number the rest of
// the program answers to.
func TestANumberAndEnterGoesToThatID(t *testing.T) {
	m, _ := newModel(t)
	m = bulk(t, m, 12)
	want := m.tasks[9]
	for _, d := range strings.Split(itoa(want.ID), "") {
		m = press(t, m, d)
	}
	m = press(t, m, "enter")
	if m.mode != modeList {
		t.Fatalf("a number and enter is a jump, not a detail view, mode = %v", m.mode)
	}
	got, ok := m.current()
	if !ok || got.ID != want.ID {
		t.Errorf("cursor is on %+v, want #%d", got, want.ID)
	}
}

func TestAnIDThatIsNotInTheListSaysSo(t *testing.T) {
	m, _ := newModel(t)
	at := m.cursor
	m = press(t, m, "9")
	m = press(t, m, "9")
	m = press(t, m, "enter")
	if m.cursor != at {
		t.Errorf("the cursor should not have moved, %d -> %d", at, m.cursor)
	}
	if !strings.Contains(m.View(), "#99") {
		t.Errorf("it should say which id it could not find:\n%s", m.View())
	}
}

// Without a count, enter is what it was.
func TestEnterWithoutACountStillOpensTheDetail(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "enter")
	if m.mode != modeDetail {
		t.Errorf("mode = %v, want the detail view", m.mode)
	}
}

// Anything else discards it: a stray 3 must not make the next j jump three.
func TestAnotherKeyDiscardsTheCount(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "3")
	m = press(t, m, "D") // some unrelated key
	if m.count != "" {
		t.Fatalf("count = %q, it should have been discarded", m.count)
	}
	m = press(t, m, "j")
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want one row down", m.cursor)
	}
}

// A leading zero is not a count, so 0 on its own leaves j alone.
func TestALeadingZeroIsIgnored(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "0")
	if m.count != "" {
		t.Fatalf("count = %q", m.count)
	}
	m = press(t, m, "1")
	m = press(t, m, "0")
	if m.count != "10" {
		t.Errorf("count = %q, a zero after a digit is part of the number", m.count)
	}
}
