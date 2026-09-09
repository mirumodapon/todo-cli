package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"todo.mirumo.net/internal/store"
	"todo.mirumo.net/internal/task"
)

// A reload keeps the cursor on the task it was on, not on the row number it was
// on: the list can change underneath, and then a row points at something else.
func TestReloadKeepsTheCursorOnTheSameTask(t *testing.T) {
	m, s := newModel(t)
	m = press(t, m, "j") // second
	on, _ := m.current()
	if on.Title != "second" {
		t.Fatalf("cursor is on %q", on.Title)
	}
	// Something else removes the task above it.
	if err := s.Delete(m.tasks[0].ID); err != nil {
		t.Fatal(err)
	}
	m = press(t, m, "R")

	got, ok := m.current()
	if !ok || got.Title != "second" {
		t.Errorf("the cursor should still be on second, got %+v", got)
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, it should have followed the task up a row", m.cursor)
	}
}

// Deleting what the cursor is on has nothing to follow, so the row is what it
// keeps: the cursor lands on whatever took its place.
func TestDeletingUnderTheCursorKeepsTheRow(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "j")
	m = press(t, m, "d")
	m = press(t, m, "y")
	if got, ok := m.current(); !ok || got.Title != "third" {
		t.Errorf("the cursor should land on the next task, got %+v", got)
	}
}

// Cycling the sort reorders the same tasks, so the cursor stays with its task.
func TestSortingKeepsTheCursorOnTheSameTask(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "G")
	on, _ := m.current()
	m = press(t, m, "s")
	got, ok := m.current()
	if !ok || got.ID != on.ID {
		t.Errorf("the cursor jumped from %q to %+v when the order changed", on.Title, got)
	}
}

// A filter change is a fresh view, and starts at the top rather than hunting for
// where the old cursor went.
func TestSearchingStartsAtTheTop(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "G")
	m = press(t, m, "/")
	m = press(t, m, "d")
	if m.cursor != 0 {
		t.Errorf("cursor = %d, a new filter should start at the top", m.cursor)
	}
	if !strings.Contains(m.View(), "second") {
		t.Errorf("the search should have narrowed the list:\n%s", m.View())
	}
}

// watched is a store whose change counter the test drives. The counter only
// moves for another connection's writes, which an in-memory database cannot
// have; that the real one moves is store's business, tested there.
type watched struct {
	store.Store
	version int64
}

func (w *watched) DataVersion() (int64, error) { return w.version, nil }

// The poll: a tick asks the database whether anyone else has written, and only a
// changed answer costs a query for the tasks.
func TestPollReloadsOnlyWhenAnotherProcessHasWritten(t *testing.T) {
	m, s := newModel(t)
	m.poll = 0 // no real timers in a test; the messages are fed by hand
	w := &watched{Store: s, version: 1}
	m.store = w

	// The first answer is the baseline, not a change.
	m = feed(t, m, tickMsg{})
	before := len(m.tasks)
	if !m.versionKnown || m.dbVersion != 1 {
		t.Fatalf("the first look should record where the database is, got %d", m.dbVersion)
	}

	// Something else writes, but the counter has not been read again yet.
	if _, err := s.Add(task.Task{Title: "added elsewhere", CreatedAt: refTime(), UpdatedAt: refTime()}); err != nil {
		t.Fatal(err)
	}
	m = feed(t, m, tickMsg{})
	if len(m.tasks) != before {
		t.Fatalf("an unchanged counter means no reload, got %d tasks", len(m.tasks))
	}

	w.version = 2
	m = feed(t, m, tickMsg{})
	if len(m.tasks) != before+1 {
		t.Fatalf("the tick should have noticed the write, got %d tasks", len(m.tasks))
	}
	if !strings.Contains(m.View(), "added elsewhere") {
		t.Errorf("and shown it:\n%s", m.View())
	}
}

// A database that cannot be read for a moment is not a reason to stop watching.
func TestAFailedVersionReadKeepsThePollAlive(t *testing.T) {
	m, _ := newModel(t)
	m.poll = 0
	m = feed(t, m, versionMsg{err: errors.New("locked")})
	if m.versionKnown {
		t.Error("a failed read should not be taken as a baseline")
	}
	if m.err != nil {
		t.Errorf("nor shown as an error to the user: %v", m.err)
	}
}

// Nothing moves under a form, a search, or an armed confirmation: the list is
// not allowed to shift between the key that chose a task and the key that acts
// on it.
func TestPollHoldsOffWhileYouAreBusy(t *testing.T) {
	for _, c := range []struct{ name, key string }{
		{"a form", "a"},
		{"a search", "/"},
		{"a confirmation", " "},
	} {
		t.Run(c.name, func(t *testing.T) {
			m, s := newModel(t)
			m.poll = 0
			w := &watched{Store: s, version: 1}
			m.store = w
			m = feed(t, m, tickMsg{})
			before := len(m.tasks)

			m = press(t, m, c.key)
			if _, err := s.Add(task.Task{Title: "added elsewhere", CreatedAt: refTime(), UpdatedAt: refTime()}); err != nil {
				t.Fatal(err)
			}
			w.version = 2
			m = feed(t, m, tickMsg{})
			if len(m.tasks) != before {
				t.Errorf("the list moved while %s was open", c.name)
			}
		})
	}
}

// The timer itself: armed when there is an interval, silent when there is not.
func TestTickArmsOnlyWhenPollingIsOn(t *testing.T) {
	m, _ := newModel(t)
	if m.tickCmd() != nil {
		t.Error("a zero interval should arm nothing")
	}
	m.poll = time.Millisecond
	cmd := m.tickCmd()
	if cmd == nil {
		t.Fatal("an interval should arm the next look")
	}
	if _, ok := cmd().(tickMsg); !ok {
		t.Error("and it should come back as a tick")
	}
}

// New leaves polling on: noticing another window's write is the default, and
// only a test turns it off.
func TestPollingIsOnByDefault(t *testing.T) {
	s, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if m := New(s, refTime, t.TempDir(), DefaultStart()); m.poll <= 0 {
		t.Errorf("poll = %v", m.poll)
	}
}
