package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// withDesc gives the first task a description and reloads.
func withDesc(t *testing.T, m Model, desc string) Model {
	t.Helper()
	first := m.tasks[0]
	first.Desc = desc
	if err := m.store.Update(first); err != nil {
		t.Fatal(err)
	}
	m, msg := run(t, m, m.loadCmd())
	m, _ = send(t, m, msg)
	return m
}

func TestEnterOpensTheDetailView(t *testing.T) {
	m, _ := newModel(t)
	m = withDesc(t, m, "semi-skimmed\ntwo litres")

	m = press(t, m, "enter")
	if m.mode != modeDetail {
		t.Fatalf("enter should open the detail view, mode = %v", m.mode)
	}
	v := m.View()
	for _, want := range []string{"first", "semi-skimmed", "two litres", "@urgent", "high", "open"} {
		if !strings.Contains(v, want) {
			t.Errorf("the detail view is missing %q:\n%s", want, v)
		}
	}
	m = press(t, m, "esc")
	if m.mode != modeList {
		t.Error("esc should close the detail view")
	}
}

// The description is the whole point of the view, so say when there is none
// rather than showing a page that looks broken.
func TestDetailViewWithoutADescription(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "j") // second, which has no description
	m = press(t, m, "enter")
	if m.mode != modeDetail {
		t.Fatal("enter should open the detail view")
	}
	if !strings.Contains(m.View(), "No description") {
		t.Errorf("it should say the description is empty:\n%s", m.View())
	}
}

func TestEnterOnAnEmptyListDoesNothing(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "/")
	m = press(t, m, "zzzz") // matches nothing
	m = press(t, m, "enter")
	m = press(t, m, "enter")
	if m.mode == modeDetail {
		t.Error("there is nothing under the cursor to show")
	}
}

// A long description is wrapped to the frame, and the padding that wrapping
// leaves behind must not survive into the output.
func TestDetailViewWrapsWithoutTrailingSpaces(t *testing.T) {
	m, _ := newModel(t)
	m = withDesc(t, m, strings.Repeat("a long line that has to wrap somewhere ", 6))
	m = press(t, m, "enter")
	for _, line := range strings.Split(m.View(), "\n") {
		if strings.HasSuffix(line, " ") {
			t.Errorf("trailing whitespace: %q", line)
		}
		if lipgloss.Width(line) > 80 {
			t.Errorf("line wider than the 80-column frame: %q", line)
		}
	}
}

// fakeEditor stands in for the user's editor: it records what it was handed and
// returns text without spawning anything.
func fakeEditor(m Model, out string, err error, saw *string) Model {
	m.edit = func(text string, apply func(string, error) tea.Msg) tea.Cmd {
		*saw = text
		return func() tea.Msg { return apply(out, err) }
	}
	return m
}

func TestDetailViewEditsTheDescriptionInAnEditor(t *testing.T) {
	m, _ := newModel(t)
	m = withDesc(t, m, "before")
	var saw string
	m = fakeEditor(m, "from the editor", nil, &saw)

	m = press(t, m, "enter")
	m = press(t, m, "E")

	if saw != "before" {
		t.Errorf("the editor should open on the current description, got %q", saw)
	}
	back, err := m.store.Get(m.tasks[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.Desc != "from the editor" {
		t.Errorf("desc = %q, what the editor returned should be saved", back.Desc)
	}
	if m.mode != modeDetail {
		t.Errorf("the view should stay open on the task, mode = %v", m.mode)
	}
	if !strings.Contains(m.View(), "from the editor") {
		t.Errorf("the new text should be on screen:\n%s", m.View())
	}
}

func TestAFailingEditorIsReportedNotSaved(t *testing.T) {
	m, _ := newModel(t)
	m = withDesc(t, m, "before")
	var saw string
	m = fakeEditor(m, "", errors.New("exit status 3"), &saw)

	m = press(t, m, "enter")
	m = press(t, m, "E")

	if back, _ := m.store.Get(m.tasks[0].ID); back.Desc != "before" {
		t.Errorf("desc = %q, an aborted edit must change nothing", back.Desc)
	}
	if m.err == nil {
		t.Error("the failure should be recorded")
	}
}

// E works from the list too: the form is for the five short fields, and e there
// still opens it.
func TestEditsTheDescriptionFromTheList(t *testing.T) {
	m, _ := newModel(t)
	m = withDesc(t, m, "before")
	var saw string
	m = fakeEditor(m, "from the editor", nil, &saw)

	m = press(t, m, "E")
	if m.mode != modeList {
		t.Errorf("E should not leave the list, mode = %v", m.mode)
	}
	if saw != "before" {
		t.Errorf("the editor should open on the current description, got %q", saw)
	}
	if back, _ := m.store.Get(m.tasks[0].ID); back.Desc != "from the editor" {
		t.Errorf("desc = %q", back.Desc)
	}
}

// The form is unchanged: it edits the five single-line fields and leaves the
// description exactly as it found it.
func TestTheFormLeavesTheDescriptionAlone(t *testing.T) {
	m, _ := newModel(t)
	m = withDesc(t, m, "written elsewhere")
	m = press(t, m, "e")
	m = press(t, m, "!")
	m = press(t, m, "enter")
	if back, _ := m.store.Get(m.tasks[0].ID); back.Desc != "written elsewhere" {
		t.Errorf("desc = %q, saving the form must not touch it", back.Desc)
	}
}

// In the detail view every key but E still closes.
func TestLowercaseEClosesTheDetailView(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "enter")
	m = press(t, m, "e")
	if m.mode != modeList {
		t.Errorf("e should close the detail view, mode = %v", m.mode)
	}
}
