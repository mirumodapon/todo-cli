package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"todo.mirumo.net/internal/store"
	"todo.mirumo.net/internal/task"
)

func refTime() time.Time { return time.Date(2026, 8, 29, 15, 0, 0, 0, time.Local) }

func day(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	return &t
}

// newModel builds a Model backed by an in-memory database with its tasks already loaded.
func newModel(t *testing.T) (Model, store.Store) {
	t.Helper()
	s, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	for _, ti := range []task.Task{
		{Title: "first", Due: day(2026, 8, 29), Priority: task.PriHigh, Tags: []string{"urgent"}},
		{Title: "second", Project: "/p/work"},
		{Title: "third"},
	} {
		ti.CreatedAt, ti.UpdatedAt = refTime(), refTime()
		if _, err := s.Add(ti); err != nil {
			t.Fatal(err)
		}
	}
	m := New(s, refTime, t.TempDir())
	m, _ = send(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, msg := run(t, m, m.Init())
	m, _ = send(t, m, msg)
	return m, s
}

// key turns a key name into a tea.KeyMsg.
func key(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "ctrl+p":
		return tea.KeyMsg{Type: tea.KeyCtrlP}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// send feeds one msg and returns the new model plus the msg its cmd produced, or nil when there is no cmd.
func send(t *testing.T, m Model, msg tea.Msg) (Model, tea.Msg) {
	t.Helper()
	next, cmd := m.Update(msg)
	return run(t, next.(Model), cmd)
}

func run(t *testing.T, m Model, cmd tea.Cmd) (Model, tea.Msg) {
	t.Helper()
	if cmd == nil {
		return m, nil
	}
	return m, cmd()
}

// press sends one key and feeds back whatever its cmd produced, mimicking one turn of the Bubble Tea loop.
func press(t *testing.T, m Model, k string) Model {
	t.Helper()
	m, msg := send(t, m, key(k))
	for i := 0; msg != nil && i < 4; i++ {
		m, msg = send(t, m, msg)
	}
	return m
}

func TestInitLoadsTasks(t *testing.T) {
	m, _ := newModel(t)
	if len(m.tasks) != 3 {
		t.Fatalf("loaded %d tasks, want 3", len(m.tasks))
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
}

func TestNavigation(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "j")
	if m.cursor != 1 {
		t.Errorf("after j cursor = %d, want 1", m.cursor)
	}
	m = press(t, m, "down")
	m = press(t, m, "down")
	if m.cursor != 2 {
		t.Errorf("at the bottom cursor = %d, want it clamped to 2", m.cursor)
	}
	m = press(t, m, "k")
	if m.cursor != 1 {
		t.Errorf("after k cursor = %d, want 1", m.cursor)
	}
	m = press(t, m, "g")
	if m.cursor != 0 {
		t.Errorf("after g cursor = %d, want 0", m.cursor)
	}
	m = press(t, m, "G")
	if m.cursor != 2 {
		t.Errorf("after G cursor = %d, want 2", m.cursor)
	}
	m = press(t, m, "k")
	m = press(t, m, "k")
	m = press(t, m, "k")
	if m.cursor != 0 {
		t.Errorf("at the top cursor = %d, want it clamped to 0", m.cursor)
	}
}

func TestSpaceTogglesDone(t *testing.T) {
	m, s := newModel(t)
	id := m.tasks[0].ID
	m = press(t, m, " ")
	m = press(t, m, "y")

	got, err := s.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Done() {
		t.Error("space should mark the task done")
	}
	if len(m.tasks) != 2 {
		t.Errorf("after the reload %d remain, want 2", len(m.tasks))
	}
}

func TestQuit(t *testing.T) {
	m, _ := newModel(t)
	_, cmd := m.Update(key("q"))
	if cmd == nil {
		t.Fatal("q should return a cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q should quit the program")
	}
}

func TestErrMsgShowsWithoutCrashing(t *testing.T) {
	m, _ := newModel(t)
	m, _ = send(t, m, errMsg{err: errFake})
	if m.err == nil {
		t.Fatal("the error should be recorded")
	}
	if !strings.Contains(m.View(), "broke") {
		t.Errorf("the error should be visible on screen: %q", m.View())
	}
}

func TestViewShowsTasksAndCursor(t *testing.T) {
	m, _ := newModel(t)
	v := m.View()
	for _, want := range []string{"first", "second", "third", "today", "!high", "@urgent", "work"} {
		if !strings.Contains(v, want) {
			t.Errorf("the view is missing %q:\n%s", want, v)
		}
	}
	if !strings.Contains(v, "▸") {
		t.Errorf("the view should carry a cursor marker:\n%s", v)
	}
}

var errFake = fakeErr{}

type fakeErr struct{}

func (fakeErr) Error() string { return "the database broke" }
