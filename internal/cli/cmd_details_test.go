package cli

import (
	"strings"
	"testing"
)

func TestAddStoresTheDescription(t *testing.T) {
	app, _, _ := newApp(t)
	if code := app.Run([]string{"add", "buy milk", "--desc", "semi-skimmed\nthe corner shop shuts at 8"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got, err := app.Store.Get(1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Desc, "semi-skimmed") || !strings.Contains(got.Desc, "shuts at 8") {
		t.Errorf("desc = %q, both lines should be stored", got.Desc)
	}
}

func TestEditChangesAndClearsTheDescription(t *testing.T) {
	app, _, _ := newApp(t)
	app.Run([]string{"add", "buy milk", "--desc", "first"})

	app.Run([]string{"edit", "1", "--pri", "high"})
	if got, _ := app.Store.Get(1); got.Desc != "first" {
		t.Errorf("desc = %q, an edit without --desc must leave it alone", got.Desc)
	}

	app.Run([]string{"edit", "1", "--desc", "second"})
	if got, _ := app.Store.Get(1); got.Desc != "second" {
		t.Errorf("desc = %q, want second", got.Desc)
	}

	app.Run([]string{"edit", "1", "--desc", ""})
	if got, _ := app.Store.Get(1); got.Desc != "" {
		t.Errorf("desc = %q, --desc \"\" should clear it", got.Desc)
	}
}

func TestDetailsPrintsEveryField(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "buy milk", "-d", "2026-09-01", "--pri", "high",
		"-t", "shopping", "-t", "chores", "-p", "/p/home", "--desc", "semi-skimmed\ntwo litres"})
	out.Reset()

	if code := app.Run([]string{"details", "1"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	s := out.String()
	for _, want := range []string{
		"#1", "buy milk", "open", "2026-09-01", "high", "home", "/p/home",
		"@shopping", "@chores", "semi-skimmed", "two litres",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("details is missing %q:\n%s", want, s)
		}
	}
}

// A task with nothing but a title should not print a page of empty labels.
func TestDetailsOmitsEmptyFields(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "bare"})
	out.Reset()
	app.Run([]string{"details", "1"})
	s := out.String()
	for _, unwanted := range []string{"due", "priority", "project", "tags"} {
		if strings.Contains(s, unwanted) {
			t.Errorf("a bare task should print no %q line:\n%s", unwanted, s)
		}
	}
	if !strings.Contains(s, "bare") || !strings.Contains(s, "open") {
		t.Errorf("the title and status are always shown:\n%s", s)
	}
}

func TestDetailsShowsDoneAndSeveralTasks(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "one"})
	app.Run([]string{"add", "two"})
	app.Run([]string{"done", "1"})
	out.Reset()

	if code := app.Run([]string{"details", "1", "2"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	s := out.String()
	if !strings.Contains(s, "done") {
		t.Errorf("a finished task should say so:\n%s", s)
	}
	if !strings.Contains(s, "one") || !strings.Contains(s, "two") {
		t.Errorf("both tasks should be printed:\n%s", s)
	}
}

func TestDetailsRejectsBadInput(t *testing.T) {
	app, _, errBuf := newApp(t)
	app.Run([]string{"add", "one"})

	if code := app.Run([]string{"details"}); code != 1 {
		t.Errorf("exit code = %d, want 1 with no id", code)
	}
	errBuf.Reset()
	if code := app.Run([]string{"details", "99"}); code != 1 {
		t.Errorf("exit code = %d, want 1 for an unknown id", code)
	}
	if !strings.Contains(errBuf.String(), "#99") {
		t.Errorf("the error should name the id: %q", errBuf.String())
	}
}
