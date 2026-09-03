package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"todo.mirumo.net/internal/task"
)

// typeInto types a string into the focused field.
func typeInto(t *testing.T, m Model, s string) Model {
	t.Helper()
	return press(t, m, s)
}

func TestFormAddCreatesTask(t *testing.T) {
	m, s := newModel(t)
	m = press(t, m, "a")
	if m.mode != modeForm {
		t.Fatal("a should open the form")
	}
	m = typeInto(t, m, "fourth")
	m = press(t, m, "tab")
	m = press(t, m, "tab")
	m = typeInto(t, m, "urgent,misc")
	m = press(t, m, "tab")
	m = typeInto(t, m, "tomorrow")
	m = press(t, m, "tab")
	m = typeInto(t, m, "high")
	m = press(t, m, "enter")

	if m.mode != modeList {
		t.Fatalf("saving should return to the list, mode = %v", m.mode)
	}
	ts, err := s.List(task.Filter{Search: "fourth"}, refTime())
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) != 1 {
		t.Fatalf("one task should have been added, got %d", len(ts))
	}
	got := ts[0]
	if got.Due == nil || got.Due.Format("2006-01-02") != "2026-08-30" {
		t.Errorf("due = %v", got.Due)
	}
	if got.Priority != task.PriHigh {
		t.Errorf("priority = %v", got.Priority)
	}
	if len(got.Tags) != 2 {
		t.Errorf("tags = %v, the comma-separated value should split into two", got.Tags)
	}
}

func TestFormEditPrefillsAndUpdates(t *testing.T) {
	m, s := newModel(t)
	id := m.tasks[0].ID
	m = press(t, m, "e")
	if m.mode != modeForm {
		t.Fatal("e should open the form")
	}
	if !strings.Contains(m.View(), "first") {
		t.Errorf("editing should prefill the current values:\n%s", m.View())
	}
	for range len([]rune("first")) {
		m = press(t, m, "backspace")
	}
	m = typeInto(t, m, "changed")
	m = press(t, m, "enter")

	got, err := s.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "changed" {
		t.Errorf("title = %q", got.Title)
	}
	if got.Due == nil {
		t.Error("fields left alone should keep their value")
	}
}

// The longest label used to touch its input, because the column width was
// hard-coded to exactly that label's length.
func TestFormLabelsAreSeparatedFromTheirInputs(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "a")
	for _, line := range strings.Split(m.View(), "\n") {
		for _, label := range []string{"Title", "Project", "Tags", "Due", "Priority"} {
			if i := strings.Index(line, label); i >= 0 {
				rest := line[i+len(label):]
				if rest != "" && !strings.HasPrefix(rest, " ") {
					t.Errorf("%q runs into its input: %q", label, line)
				}
			}
		}
	}
}

func TestFormRejectsEmptyTitle(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "a")
	m = press(t, m, "enter")
	if m.mode != modeForm {
		t.Error("an empty title must not leave the form")
	}
	if !strings.Contains(m.View(), "title cannot be empty") {
		t.Errorf("it should say why it cannot save:\n%s", m.View())
	}
}

func TestFormRejectsBadDue(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "a")
	m = typeInto(t, m, "test task")
	m = press(t, m, "tab")
	m = press(t, m, "tab")
	m = press(t, m, "tab")
	m = typeInto(t, m, "someday")
	m = press(t, m, "enter")
	if m.mode != modeForm {
		t.Error("an invalid date must not leave the form")
	}
	if !strings.Contains(m.View(), "cannot read date") {
		t.Errorf("it should point at the date:\n%s", m.View())
	}
}

func TestFormEscCancels(t *testing.T) {
	m, s := newModel(t)
	before, _ := s.List(task.Filter{}, refTime())
	m = press(t, m, "a")
	m = typeInto(t, m, "do not save")
	m = press(t, m, "esc")
	if m.mode != modeList {
		t.Error("esc should return to the list")
	}
	after, _ := s.List(task.Filter{}, refTime())
	if len(after) != len(before) {
		t.Error("esc must not save anything")
	}
}

func TestFormFillsProjectFromCwd(t *testing.T) {
	m, _ := newModel(t)
	if err := os.MkdirAll(filepath.Join(m.cwd, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	m = press(t, m, "a")
	m = press(t, m, "ctrl+r")
	if !strings.Contains(m.View(), filepath.Base(m.cwd)) {
		t.Errorf("ctrl+r should fill in the current directory's project:\n%s", m.View())
	}
}

func TestHelpOverlay(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "?")
	if m.mode != modeHelp {
		t.Fatal("? should open the help")
	}
	v := m.View()
	for _, want := range []string{"space", "d", "u", "/", "P", "T"} {
		if !strings.Contains(v, want) {
			t.Errorf("the help is missing %q:\n%s", want, v)
		}
	}
	m = press(t, m, "esc")
	if m.mode != modeList {
		t.Error("esc should close the help")
	}
}
