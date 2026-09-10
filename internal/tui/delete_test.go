package tui

import (
	"strings"
	"testing"

	"todo.mirumo.net/internal/task"
)

// d puts a task aside rather than destroying it, so u still works after the
// interface has been closed and opened again.
func TestDeleteIsSoft(t *testing.T) {
	m, s := newModel(t)
	victim := m.tasks[0]
	m = press(t, m, "d")
	m = press(t, m, "y")

	if len(m.tasks) != 2 {
		t.Fatalf("it should be out of the list, got %d tasks", len(m.tasks))
	}
	back, err := s.Get(victim.ID)
	if err != nil {
		t.Fatalf("the task should still exist: %v", err)
	}
	if !back.Deleted() {
		t.Error("and be marked deleted")
	}
	if !strings.Contains(m.View(), "u to undo") {
		t.Errorf("the bottom should offer the way back:\n%s", m.View())
	}

	m = press(t, m, "u")
	if len(m.tasks) != 3 {
		t.Errorf("u should bring it back, got %d tasks", len(m.tasks))
	}
	if got, _ := s.Get(victim.ID); got.Deleted() {
		t.Error("and clear the mark")
	}
}

func TestTheBinCanBeOpenedAndEmptied(t *testing.T) {
	m, s := newModel(t)
	victim := m.tasks[1]
	if err := s.SetDeleted(victim.ID, true, refTime()); err != nil {
		t.Fatal(err)
	}
	// Reopen on the bin, as "task tui --deleted" would.
	m = New(s, refTime, t.TempDir(), Start{Filter: task.Filter{OnlyDeleted: true}})
	m.poll = 0
	m, msg := run(t, m, m.Init())
	m, _ = send(t, m, msg)

	if len(m.tasks) != 1 || m.tasks[0].ID != victim.ID {
		t.Fatalf("the bin should hold just the deleted one, got %d", len(m.tasks))
	}
	m = press(t, m, "u")
	if got, _ := s.Get(victim.ID); got.Deleted() {
		t.Error("u should bring back the task under the cursor")
	}
	if len(m.tasks) != 0 {
		t.Errorf("and it should leave the bin, got %d", len(m.tasks))
	}
}

// The detail view says a task is deleted, so looking at one before bringing it
// back tells you what you are looking at.
func TestTheDetailViewSaysDeleted(t *testing.T) {
	m, s := newModel(t)
	victim := m.tasks[0]
	if err := s.SetDeleted(victim.ID, true, refTime()); err != nil {
		t.Fatal(err)
	}
	m = New(s, refTime, t.TempDir(), Start{Filter: task.Filter{OnlyDeleted: true}})
	m.poll = 0
	m, msg := run(t, m, m.Init())
	m, _ = send(t, m, msg)
	m = press(t, m, "enter")
	if !strings.Contains(m.View(), "deleted") {
		t.Errorf("the detail should say so:\n%s", m.View())
	}
}
