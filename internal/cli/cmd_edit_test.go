package cli

import (
	"strings"
	"testing"

	"todo.mirumo.net/internal/task"
)

func TestEditOnlyTouchesGivenFlags(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "buy milk", "-d", "tomorrow", "--pri", "high", "-t", "shopping", "-p", "work"})
	out.Reset()

	if code := app.Run([]string{"edit", "1", "--pri", "low"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.Priority != task.PriLow {
		t.Errorf("priority = %v, want low", got.Priority)
	}
	if got.Due == nil {
		t.Error("leaving out --due must not touch the due date")
	}
	if got.Project != "work" {
		t.Errorf("project = %q, leaving out -p must not touch it", got.Project)
	}
	if len(got.Tags) != 1 {
		t.Errorf("tags = %v, leaving out -t must not touch them", got.Tags)
	}
}

func TestEditEmptyDueClearsIt(t *testing.T) {
	app, _, _ := newApp(t)
	app.Run([]string{"add", "buy milk", "-d", "tomorrow"})
	if code := app.Run([]string{"edit", "1", "--due", ""}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.Due != nil {
		t.Errorf("due = %v, --due \"\" should clear the due date", got.Due)
	}
}

func TestEditEmptyProjectMakesItGlobal(t *testing.T) {
	app, _, _ := newApp(t)
	app.Run([]string{"add", "buy milk", "-p", "work"})
	if code := app.Run([]string{"edit", "1", "--project="}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.Project != "" {
		t.Errorf("project = %q, --project= should move it back to uncategorized", got.Project)
	}
}

func TestEditReplacesTagsWholesale(t *testing.T) {
	app, _, _ := newApp(t)
	app.Run([]string{"add", "buy milk", "-t", "shopping", "-t", "chores"})
	app.Run([]string{"edit", "1", "-t", "breakfast"})
	got, _ := app.Store.Get(1)
	if len(got.Tags) != 1 || got.Tags[0] != "breakfast" {
		t.Errorf("tags = %v, -t should replace the set rather than append", got.Tags)
	}
}

func TestEditTitleViaSecondPositional(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "buy milk"})
	out.Reset()
	if code := app.Run([]string{"edit", "1", "buy soy milk"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.Title != "buy soy milk" {
		t.Errorf("title = %q", got.Title)
	}
	if !strings.Contains(out.String(), "updated #1: buy soy milk") {
		t.Errorf("stdout = %q", out.String())
	}
}

func TestEditErrors(t *testing.T) {
	app, _, _ := newApp(t)
	app.Run([]string{"add", "buy milk"})
	for _, args := range [][]string{
		{"edit"},
		{"edit", "x"},
		{"edit", "42", "--pri", "low"},
		{"edit", "1", "a", "b"},
		{"edit", "1", "--due", "someday"},
	} {
		if code := app.Run(args); code == 0 {
			t.Errorf("%v should fail", args)
		}
	}
}
