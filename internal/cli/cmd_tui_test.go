package cli

import (
	"strings"
	"testing"

	"todo.mirumo.net/internal/task"
)

// startedWith runs "tui" with args and returns the filter the interface would open on.
func startedWith(t *testing.T, args ...string) (task.Filter, bool, int) {
	t.Helper()
	app, _, _ := newApp(t)
	var got task.Filter
	var dates bool
	app.RunTUI = func(f task.Filter, d bool) error { got, dates = f, d; return nil }
	code := app.Run(append([]string{"tui"}, args...))
	return got, dates, code
}

func TestTUIOpensOnTheFilterFromItsFlags(t *testing.T) {
	got, dates, code := startedWith(t, "-t", "urgent", "-t", "home", "--pri", "high",
		"-d", "week", "-a", "-s", "pri", "--dates")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if strings.Join(got.Tags, ",") != "urgent,home" {
		t.Errorf("tags = %v", got.Tags)
	}
	if got.Priority == nil || *got.Priority != task.PriHigh {
		t.Errorf("priority = %v", got.Priority)
	}
	if got.DueRange != task.DueWeek {
		t.Errorf("due = %v", got.DueRange)
	}
	if !got.IncludeDone || got.Sort != task.SortPriority {
		t.Errorf("filter = %+v", got)
	}
	if !dates {
		t.Error("--dates should start the interface showing calendar dates")
	}
}

// With no flags at all the interface opens where "task ls" points: uncategorized.
func TestTUIDefaultsToUncategorized(t *testing.T) {
	got, dates, code := startedWith(t)
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if got.Project == nil || *got.Project != "" {
		t.Errorf("project = %v, want a pointer to an empty string", got.Project)
	}
	if dates {
		t.Error("dates should be off by default")
	}
}

func TestTUIProjectSelectors(t *testing.T) {
	got, _, _ := startedWith(t, "-p", "work")
	if got.Project == nil || *got.Project != "work" {
		t.Errorf("-p work: project = %v", got.Project)
	}
	got, _, _ = startedWith(t, "--all-projects")
	if got.Project != nil {
		t.Errorf("--all-projects: project = %v, want nil", got.Project)
	}
	if _, _, code := startedWith(t, "-p", "work", "--all-projects"); code != 1 {
		t.Errorf("exit code = %d, the selectors should still be mutually exclusive", code)
	}
}

func TestTUIRejectsWhatItCannotUse(t *testing.T) {
	app, _, errBuf := newApp(t)
	app.RunTUI = func(task.Filter, bool) error { return nil }

	if code := app.Run([]string{"tui", "extra"}); code != 1 {
		t.Errorf("exit code = %d, want 1 for a positional argument", code)
	}
	errBuf.Reset()
	// Colour is not a choice inside the interface, so the flag would be a lie.
	if code := app.Run([]string{"tui", "-c"}); code != 1 {
		t.Errorf("exit code = %d, want 1 for -c", code)
	}
	if !strings.Contains(errBuf.String(), "-c") {
		t.Errorf("the error should name the flag: %q", errBuf.String())
	}
}

func TestTUIReportsABadFilterValue(t *testing.T) {
	app, _, errBuf := newApp(t)
	app.RunTUI = func(task.Filter, bool) error { t.Error("it should not open at all"); return nil }
	if code := app.Run([]string{"tui", "-d", "someday"}); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "someday") {
		t.Errorf("the error should name the value: %q", errBuf.String())
	}
}
