package tui

import (
	"errors"
	"strings"
	"testing"

	"todo.mirumo.net/internal/store"
)

func TestSpaceAsksBeforeMarkingDone(t *testing.T) {
	m, s := newModel(t)
	id := m.tasks[0].ID

	m = press(t, m, " ")
	if m.mode != modeConfirm {
		t.Fatalf("space should ask first, mode = %v", m.mode)
	}
	if got, _ := s.Get(id); got.Done() {
		t.Error("no data should change before confirmation")
	}
	v := m.View()
	if !strings.Contains(v, "first") || !strings.Contains(v, "y/n") {
		t.Errorf("the prompt should name the task and the keys: %q", v)
	}

	m = press(t, m, "y")
	if m.mode != modeList {
		t.Errorf("confirming should return to the list, mode = %v", m.mode)
	}
	if got, _ := s.Get(id); !got.Done() {
		t.Error("y should actually mark it done")
	}
}

func TestConfirmCancelLeavesDataUnchanged(t *testing.T) {
	for _, cancelKey := range []string{"n", "esc", "q"} {
		m, s := newModel(t)
		id := m.tasks[0].ID
		m = press(t, m, " ")
		m = press(t, m, cancelKey)
		if m.mode != modeList {
			t.Errorf("%s should return to the list, mode = %v", cancelKey, m.mode)
		}
		if got, _ := s.Get(id); got.Done() {
			t.Errorf("cancelling with %s must change nothing", cancelKey)
		}
	}
}

func TestUnmarkingDoneNeedsNoConfirmation(t *testing.T) {
	// Completing is gated, so a stray keypress cannot complete anything, and
	// reopening is itself the recovery move: asking again would be pure noise.
	m, s := newModel(t)
	id := m.tasks[0].ID
	m = press(t, m, " ")
	m = press(t, m, "y")
	m = press(t, m, "A") // show done tasks so the cursor can reach it
	m = press(t, m, "g") // dated tasks sort first, so "first" is at the top
	if cur, ok := m.current(); !ok || !cur.Done() {
		t.Fatalf("the cursor should sit on the done task, got %+v", cur)
	}

	m = press(t, m, " ")
	if m.mode != modeList {
		t.Fatalf("reopening must not ask again, mode = %v", m.mode)
	}
	if got, _ := s.Get(id); got.Done() {
		t.Error("it should be reopened by now")
	}
}

func TestDeleteAsksBeforeDeleting(t *testing.T) {
	m, s := newModel(t)
	victim := m.tasks[0]

	m = press(t, m, "d")
	if m.mode != modeConfirm {
		t.Fatalf("d should ask first, mode = %v", m.mode)
	}
	if _, err := s.Get(victim.ID); err != nil {
		t.Error("nothing should be deleted before confirmation")
	}
	if !strings.Contains(m.View(), victim.Title) {
		t.Errorf("the prompt should name the task being deleted: %q", m.View())
	}

	m = press(t, m, "y")
	if _, err := s.Get(victim.ID); !errors.Is(err, store.ErrNotFound) {
		t.Error("y should actually delete it")
	}
	if !strings.Contains(m.View(), "u to undo") {
		t.Errorf("undo should still be offered after a delete: %q", m.View())
	}
}

func TestDeleteCancelKeepsTheTask(t *testing.T) {
	m, s := newModel(t)
	victim := m.tasks[0]
	m = press(t, m, "d")
	m = press(t, m, "esc")
	if _, err := s.Get(victim.ID); err != nil {
		t.Errorf("the task should survive a cancel: %v", err)
	}
	if len(m.tasks) != 3 {
		t.Errorf("the list should be unchanged, got %d", len(m.tasks))
	}
}

func TestConfirmOnEmptyListDoesNothing(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "/")
	m = press(t, m, "no such thing")
	m = press(t, m, "enter")
	if len(m.tasks) != 0 {
		t.Fatalf("want an empty list, got %d", len(m.tasks))
	}
	for _, k := range []string{" ", "d"} {
		m2 := press(t, m, k)
		if m2.mode != modeList {
			t.Errorf("pressing %q on an empty list must not open a confirmation", k)
		}
	}
}
