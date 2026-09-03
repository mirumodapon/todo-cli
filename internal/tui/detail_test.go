package tui

import (
	"errors"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"todo.mirumo.net/internal/editor"
	"todo.mirumo.net/internal/task"
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

// fakeEditor stands in for the user's editor: it records the file it was handed
// and returns one, without spawning anything.
func fakeEditor(m Model, out string, err error, saw *string) Model {
	m.edit = func(text string, apply func(string, string, error) tea.Msg) tea.Cmd {
		*saw = text
		return func() tea.Msg { return apply(out, "", err) }
	}
	return m
}

// edited builds the file an editor would return for these fields.
func edited(title, pri, desc string) string {
	return "title: " + title + "\npriority: " + pri + "\n\n" + desc
}

func TestEditsTheWholeTaskInAnEditor(t *testing.T) {
	m, _ := newModel(t)
	m = withDesc(t, m, "before")
	var saw string
	m = fakeEditor(m, edited("renamed", "low", "after"), nil, &saw)

	m = press(t, m, "enter")
	m = press(t, m, "E")

	// The file the editor opened carries every field, not just the description.
	for _, want := range []string{"title: first", "priority: high", "tags: urgent", "before"} {
		if !strings.Contains(saw, want) {
			t.Errorf("the file is missing %q:\n%s", want, saw)
		}
	}
	back, err := m.store.Get(m.tasks[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.Title != "renamed" || back.Priority != task.PriLow || back.Desc != "after" {
		t.Errorf("every edited field should be saved: %+v", back)
	}
	if len(back.Tags) != 1 || back.Tags[0] != "urgent" {
		t.Errorf("tags = %v, a field the file kept should survive", back.Tags)
	}
	if m.mode != modeDetail {
		t.Errorf("the view should stay open on the task, mode = %v", m.mode)
	}
	if !strings.Contains(m.View(), "after") {
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

// A file that will not parse must not throw away what was typed into it.
func TestAnUnparsableFileIsKeptAndReported(t *testing.T) {
	m, _ := newModel(t)
	path, err := editor.WriteTemp("task", "")
	if err != nil {
		t.Fatal(err)
	}
	m.edit = func(text string, apply func(string, string, error) tea.Msg) tea.Cmd {
		return func() tea.Msg { return apply("colour: red\n\nhours of typing", path, nil) }
	}

	m = press(t, m, "E")
	if m.err == nil || !strings.Contains(m.err.Error(), "colour") {
		t.Fatalf("the parse failure should be reported, got %v", m.err)
	}
	if !strings.Contains(m.err.Error(), path) {
		t.Errorf("the error should say where the text is: %v", m.err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the file should still be there: %v", err)
	}
	editor.Discard(path)
}

// E works from the list too: the form is for the five short fields, and e there
// still opens it.
func TestEditsTheTaskFromTheList(t *testing.T) {
	m, _ := newModel(t)
	m = withDesc(t, m, "before")
	var saw string
	m = fakeEditor(m, edited("renamed", "low", "after"), nil, &saw)

	m = press(t, m, "E")
	if m.mode != modeList {
		t.Errorf("E should not leave the list, mode = %v", m.mode)
	}
	if !strings.Contains(saw, "title: first") {
		t.Errorf("the editor should open on the current task, got %q", saw)
	}
	if back, _ := m.store.Get(m.tasks[0].ID); back.Title != "renamed" {
		t.Errorf("title = %q", back.Title)
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
