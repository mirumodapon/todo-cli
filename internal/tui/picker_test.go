package tui

import (
	"strings"
	"testing"
)

func TestProjectPickerFiltersByProject(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "P")
	if m.mode != modePicker {
		t.Fatal("P should open the menu")
	}
	v := m.View()
	for _, want := range []string{"All", "(uncategorized)", "work"} {
		if !strings.Contains(v, want) {
			t.Errorf("the menu is missing %q:\n%s", want, v)
		}
	}
	// Row 0 is the clear-filter entry, row 1 is uncategorised, row 2 is work.
	m = press(t, m, "j")
	m = press(t, m, "j")
	m = press(t, m, "enter")
	if m.mode != modeList {
		t.Fatal("enter should return to the list")
	}
	if len(m.tasks) != 1 || m.tasks[0].Title != "second" {
		t.Errorf("picking work should leave only second, got %d", len(m.tasks))
	}
}

func TestProjectPickerUncategorized(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "P")
	m = press(t, m, "j")
	m = press(t, m, "enter")
	if len(m.tasks) != 2 {
		t.Errorf("(uncategorized) should leave 2, got %d", len(m.tasks))
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
	m = press(t, m, "enter") // row 0 clears the filter
	if m.filter.Project != nil {
		t.Errorf("picking All should clear the project filter, got %v", m.filter.Project)
	}
	if len(m.tasks) != 3 {
		t.Errorf("got %d, want 3", len(m.tasks))
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
	m = press(t, m, "j")
	m = press(t, m, "enter")
	if len(m.tasks) != 1 || m.tasks[0].Title != "first" {
		t.Errorf("picking @urgent should leave only first, got %d", len(m.tasks))
	}
}

func TestPickerEscCancels(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "P")
	m = press(t, m, "esc")
	if m.mode != modeList {
		t.Error("esc should close the menu")
	}
	if m.filter.Project != nil {
		t.Error("esc must not apply any filter")
	}
}
