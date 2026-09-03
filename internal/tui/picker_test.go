package tui

import (
	"strings"
	"testing"

	"todo.mirumo.net/internal/store"
	"todo.mirumo.net/internal/task"
)

func TestProjectPickerFiltersByProject(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "P")
	if m.mode != modePicker {
		t.Fatal("P should open the menu")
	}
	v := m.View()
	for _, want := range []string{"All projects", "(uncategorized)", "work"} {
		if !strings.Contains(v, want) {
			t.Errorf("the menu is missing %q:\n%s", want, v)
		}
	}
	// Row 0 is all projects, row 1 is uncategorized, row 2 is work.
	m = press(t, m, "j")
	m = press(t, m, "j")
	m = press(t, m, "enter")
	if m.mode != modeList {
		t.Fatal("enter should return to the list")
	}
	if len(m.tasks) != 1 || m.tasks[0].Title != "work one" {
		t.Errorf("picking work should leave only work one, got %d", len(m.tasks))
	}
}

func TestProjectPickerUncategorized(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "P")
	m = press(t, m, "j")
	m = press(t, m, "enter")
	if len(m.tasks) != 3 {
		t.Errorf("(uncategorized) should leave 3, got %d", len(m.tasks))
	}
	if m.filter.Project == nil || *m.filter.Project != "" {
		t.Errorf("filter.Project = %v, want a pointer to an empty string", m.filter.Project)
	}
}

func TestProjectPickerAllClearsFilter(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "P")
	m = press(t, m, "j")
	m = press(t, m, "enter")
	m = press(t, m, "P")
	m = press(t, m, "enter") // row 0 spans every project
	if m.filter.Project != nil {
		t.Errorf("picking All projects should clear the project filter, got %v", m.filter.Project)
	}
	if len(m.tasks) != 4 {
		t.Errorf("got %d, want 4", len(m.tasks))
	}
}

func TestTagPicker(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "T")
	if m.mode != modePicker {
		t.Fatal("T should open the menu")
	}
	if !strings.Contains(m.View(), "@urgent") {
		t.Errorf("the tag menu should list @urgent:\n%s", m.View())
	}
	// Row 1 is untagged, then tags sort by name: row 2 is @misc, row 3 is @urgent.
	m = press(t, m, "j")
	m = press(t, m, "j")
	m = press(t, m, "j")
	m = press(t, m, "enter")
	if len(m.tasks) != 1 || m.tasks[0].Title != "first" {
		t.Errorf("picking @urgent should leave only first, got %d", len(m.tasks))
	}
}

func TestPickerEscCancels(t *testing.T) {
	m, _ := newModel(t)
	before := m.scope()
	m = press(t, m, "P")
	m = press(t, m, "esc")
	if m.mode != modeList {
		t.Error("esc should close the menu")
	}
	if m.scope() != before {
		t.Errorf("esc must leave the scope alone, %q became %q", before, m.scope())
	}
}

func TestProjectPickerShowsOpenCounts(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "P")
	v := m.View()
	if !strings.Contains(v, "3 open") || !strings.Contains(v, "1 open") {
		t.Errorf("the picker should show how much is open in each project:\n%s", v)
	}
}

func TestEscReturnsToTheUncategorizedDefault(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "P")
	m = press(t, m, "enter") // all projects
	if len(m.tasks) != 4 {
		t.Fatalf("got %d, want 4", len(m.tasks))
	}
	m = press(t, m, "esc")
	if m.filter.Project == nil || *m.filter.Project != "" {
		t.Errorf("esc should restore the uncategorized default, got %v", m.filter.Project)
	}
	if len(m.tasks) != 3 {
		t.Errorf("got %d, want the 3 uncategorized tasks", len(m.tasks))
	}
}

// The uncategorized scope is where the interface starts, so it has to be
// reachable from the menu even when nothing is in it yet.
func TestProjectPickerOffersUncategorizedWhenEmpty(t *testing.T) {
	s, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if _, err := s.Add(task.Task{Title: "work one", Project: "/p/work", CreatedAt: refTime(), UpdatedAt: refTime()}); err != nil {
		t.Fatal(err)
	}
	m := New(s, refTime, t.TempDir())
	m, msg := run(t, m, m.Init())
	m, _ = send(t, m, msg)

	m = press(t, m, "P")
	if !strings.Contains(m.View(), "(uncategorized)") {
		t.Fatalf("the menu should always offer uncategorized:\n%s", m.View())
	}
	m = press(t, m, "j")
	m = press(t, m, "enter")
	if m.filter.Project == nil || *m.filter.Project != "" {
		t.Errorf("filter.Project = %v, want a pointer to an empty string", m.filter.Project)
	}
}

func TestTagPickerOffersUntagged(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "T")
	if !strings.Contains(m.View(), "(untagged)") {
		t.Fatalf("the tag menu should offer untagged:\n%s", m.View())
	}
	// Row 0 is all tags, row 1 is untagged.
	m = press(t, m, "j")
	m = press(t, m, "enter")
	if !m.filter.Untagged {
		t.Error("picking untagged should set the filter")
	}
	// Of the three uncategorized tasks, only first carries a tag.
	if len(m.tasks) != 2 {
		t.Errorf("untagged should leave second and third, got %d", len(m.tasks))
	}
	if !strings.Contains(m.View(), "untagged") {
		t.Errorf("the header should name the filter:\n%s", m.View())
	}
}

func TestTagPickerReplacesUntagged(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "T")
	m = press(t, m, "j")
	m = press(t, m, "enter") // untagged
	m = press(t, m, "T")
	m = press(t, m, "j")
	m = press(t, m, "j")
	m = press(t, m, "j")
	m = press(t, m, "enter") // @urgent
	if m.filter.Untagged {
		t.Error("picking a tag should clear the untagged filter")
	}
	if len(m.tasks) != 1 || m.tasks[0].Title != "first" {
		t.Errorf("picking @urgent should leave only first, got %d", len(m.tasks))
	}
	m = press(t, m, "T")
	m = press(t, m, "enter") // All tags
	if m.filter.Untagged || m.filter.Tags != nil {
		t.Error("All tags should clear both tag filters")
	}
}

// The header is where the current filter stops being invisible state, and a
// tag filter used to leave no trace there at all.
func TestHeaderNamesTheTagFilter(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "T")
	m = press(t, m, "j")
	m = press(t, m, "j")
	m = press(t, m, "j")
	m = press(t, m, "enter")
	if !strings.Contains(m.header(), "@urgent") {
		t.Errorf("the header should name the tag filter, got %q", m.header())
	}
}
