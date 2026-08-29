package tui

import (
	"errors"
	"strings"
	"testing"

	"todo.mirumo.net/internal/store"
	"todo.mirumo.net/internal/task"
)

func TestDeleteThenUndo(t *testing.T) {
	m, s := newModel(t)
	victim := m.tasks[0]

	m = press(t, m, "d")
	m = press(t, m, "y")
	if _, err := s.Get(victim.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("d should delete the task, err = %v", err)
	}
	if len(m.tasks) != 2 {
		t.Errorf("after the delete %d remain, want 2", len(m.tasks))
	}
	if !strings.Contains(m.View(), "u to undo") {
		t.Errorf("the footer should offer the undo:\n%s", m.View())
	}

	m = press(t, m, "u")
	back, err := s.Get(victim.ID)
	if err != nil {
		t.Fatalf("u should restore under the original id, err = %v", err)
	}
	if back.Title != victim.Title || len(back.Tags) != len(victim.Tags) {
		t.Errorf("restored content does not match: %+v", back)
	}
	if len(m.tasks) != 3 {
		t.Errorf("after the undo %d remain, want 3", len(m.tasks))
	}
}

func TestUndoOnlyKeepsOneLevel(t *testing.T) {
	m, s := newModel(t)
	first := m.tasks[0]
	m = press(t, m, "d")
	m = press(t, m, "y")
	second := m.tasks[0]
	m = press(t, m, "d")
	m = press(t, m, "y")
	m = press(t, m, "u")

	if _, err := s.Get(second.ID); err != nil {
		t.Errorf("the most recent delete should be restored: %v", err)
	}
	if _, err := s.Get(first.ID); !errors.Is(err, store.ErrNotFound) {
		t.Error("undo keeps one level; an earlier delete must not come back")
	}
	m = press(t, m, "u")
	if !strings.Contains(m.View(), "nothing to undo") {
		t.Errorf("it should say so when there is nothing to undo:\n%s", m.View())
	}
}

func TestSearchFiltersIncrementally(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "/")
	if m.mode != modeSearch {
		t.Fatal("/ should enter search mode")
	}
	m = press(t, m, "second")
	if len(m.tasks) != 1 || m.tasks[0].Title != "second" {
		t.Errorf("typing should filter as you go, got %d", len(m.tasks))
	}
	m = press(t, m, "enter")
	if m.mode != modeList {
		t.Error("enter should return to the list and keep the filter")
	}
	if len(m.tasks) != 1 {
		t.Errorf("the filter should survive enter, got %d", len(m.tasks))
	}
}

func TestSearchEscCancels(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "/")
	m = press(t, m, "second")
	m = press(t, m, "esc")
	if m.mode != modeList {
		t.Error("esc should return to the list")
	}
	if len(m.tasks) != 3 {
		t.Errorf("esc should cancel the filter, got %d", len(m.tasks))
	}
}

func TestToggleIncludeDone(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, " ")
	m = press(t, m, "y")
	if len(m.tasks) != 2 {
		t.Fatalf("want 2 left, got %d", len(m.tasks))
	}
	m = press(t, m, "A")
	if len(m.tasks) != 3 {
		t.Errorf("A should also show done tasks, got %d", len(m.tasks))
	}
	m = press(t, m, "A")
	if len(m.tasks) != 2 {
		t.Errorf("pressing A again should go back to open only, got %d", len(m.tasks))
	}
}

func TestSortCycles(t *testing.T) {
	m, _ := newModel(t)
	if m.filter.Sort != task.SortDue {
		t.Fatal("the default should be due")
	}
	m = press(t, m, "s")
	if m.filter.Sort != task.SortPriority {
		t.Errorf("after s = %v, want pri", m.filter.Sort)
	}
	m = press(t, m, "s")
	if m.filter.Sort != task.SortCreated {
		t.Errorf("pressing s again = %v, want created", m.filter.Sort)
	}
	m = press(t, m, "s")
	if m.filter.Sort != task.SortDue {
		t.Errorf("cycling back = %v, want due", m.filter.Sort)
	}
}

func TestEscClearsAllFilters(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "/")
	m = press(t, m, "second")
	m = press(t, m, "enter")
	m = press(t, m, "A")
	m = press(t, m, "esc")
	if len(m.tasks) != 3 {
		t.Errorf("esc should clear every filter, got %d", len(m.tasks))
	}
	if m.filter.Search != "" || m.filter.IncludeDone {
		t.Errorf("the filter should be reset: %+v", m.filter)
	}
}
