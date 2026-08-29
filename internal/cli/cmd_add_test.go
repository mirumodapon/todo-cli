package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"todo.mirumo.net/internal/task"
)

func TestAddMinimal(t *testing.T) {
	app, out, _ := newApp(t)
	if code := app.Run([]string{"add", "  buy milk  "}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out.String(), "added #1: buy milk") {
		t.Errorf("stdout = %q", out.String())
	}
	got, err := app.Store.Get(1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "buy milk" {
		t.Errorf("title = %q, want surrounding whitespace trimmed", got.Title)
	}
	if got.Project != "" {
		t.Errorf("project = %q, without -p it should stay uncategorized", got.Project)
	}
}

func TestAddAllFlags(t *testing.T) {
	app, _, _ := newApp(t)
	code := app.Run([]string{"add", "buy milk", "-t", "shopping", "--tag=chores", "-d", "tomorrow", "--pri", "high"})
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.Due == nil || got.Due.Format("2006-01-02") != "2026-08-30" {
		t.Errorf("due = %v, want 2026-08-30", got.Due)
	}
	if got.Priority != task.PriHigh {
		t.Errorf("priority = %v", got.Priority)
	}
	if len(got.Tags) != 2 {
		t.Errorf("tags = %v", got.Tags)
	}
}

func TestAddProjectFromCwd(t *testing.T) {
	app, _, _ := newApp(t)
	root := app.Cwd
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if code := app.Run([]string{"add", "fix bug", "-p"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got, _ := app.Store.Get(1)
	want, _ := filepath.EvalSymlinks(root)
	gotResolved, _ := filepath.EvalSymlinks(got.Project)
	if gotResolved != want {
		t.Errorf("project = %q, want the current repo root %q", gotResolved, want)
	}
}

func TestAddProjectExplicitName(t *testing.T) {
	app, _, _ := newApp(t)
	if code := app.Run([]string{"add", "fix bug", "-p", "work"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.Project != "work" {
		t.Errorf("project = %q, want work", got.Project)
	}
}

func TestAddMissingTitleExplainsTheFootgun(t *testing.T) {
	app, _, errBuf := newApp(t)
	if code := app.Run([]string{"add", "-p", "buy milk"}); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	msg := errBuf.String()
	if !strings.Contains(msg, "missing title") || !strings.Contains(msg, "--project=buy milk") {
		t.Errorf("the error should say -p swallowed it and show the fix, got %q", msg)
	}
}

func TestAddRejectsBadValues(t *testing.T) {
	cases := [][]string{
		{"add", "x", "--pri", "urgent"},
		{"add", "x", "-d", "someday"},
		{"add", "   "},
		{"add", "a", "b"},
	}
	for _, args := range cases {
		app, _, errBuf := newApp(t)
		if code := app.Run(args); code == 0 {
			t.Errorf("%v should fail", args)
		}
		if errBuf.Len() == 0 {
			t.Errorf("%v should print an error message", args)
		}
	}
}

func TestAddWithTimeOfDay(t *testing.T) {
	app, out, _ := newApp(t)
	if code := app.Run([]string{"add", "standup", "-d", "today 09:30"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got, _ := app.Store.Get(1)
	if !got.DueHasTime || got.Due.Format("2006-01-02 15:04") != "2026-08-29 09:30" {
		t.Fatalf("due = %v hasTime = %v", got.Due, got.DueHasTime)
	}

	// Due today, so the date listing shows the clock rather than the word
	// "today". The default listing shows time remaining instead.
	out.Reset()
	app.Run([]string{"ls", "--dates"})
	if !strings.Contains(out.String(), "09:30") {
		t.Errorf("a task due today should list its time: %q", out.String())
	}
}

func TestAddBareTimeMeansToday(t *testing.T) {
	app, _, _ := newApp(t)
	if code := app.Run([]string{"add", "lunch", "-d", "12:00"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.Due.Format("2006-01-02 15:04") != "2026-08-29 12:00" {
		t.Errorf("due = %v, want today at 12:00", got.Due)
	}
}

func TestEditCanDropTheTimeOfDay(t *testing.T) {
	app, _, _ := newApp(t)
	app.Run([]string{"add", "standup", "-d", "today 09:30"})
	if code := app.Run([]string{"edit", "1", "--due", "today"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.DueHasTime {
		t.Error("re-setting the due date without a time should drop the time")
	}
}
