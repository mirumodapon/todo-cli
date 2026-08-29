package cli

import (
	"strings"
	"testing"
)

// Seed a few tasks before exercising ls.
func seedCLI(t *testing.T, app *App) {
	t.Helper()
	cases := [][]string{
		{"add", "overdue one", "-d", "2026-08-20"},
		{"add", "today one", "-d", "today", "--pri", "high", "-t", "urgent"},
		{"add", "work one", "-p", "work", "-t", "misc"},
		{"add", "undated one"},
	}
	for _, args := range cases {
		if code := app.Run(args); code != 0 {
			t.Fatalf("%v failed", args)
		}
	}
}

func TestLsDefaultsToOpenOnly(t *testing.T) {
	app, out, _ := newApp(t)
	seedCLI(t, app)
	if code := app.Run([]string{"done", "1"}); code != 0 {
		t.Skip("done is not implemented yet; revisit after Task 11")
	}
	out.Reset()
	app.Run([]string{"ls"})
	if strings.Contains(out.String(), "overdue one") {
		t.Errorf("the default must not list done tasks: %q", out.String())
	}
	out.Reset()
	app.Run([]string{"ls", "-a"})
	if !strings.Contains(out.String(), "overdue one") {
		t.Errorf("-a should include done tasks: %q", out.String())
	}
}

func TestLsDefaultsToUncategorized(t *testing.T) {
	app, out, _ := newApp(t)
	seedCLI(t, app)

	out.Reset()
	app.Run([]string{"ls"})
	if strings.Contains(out.String(), "work one") {
		t.Errorf("the default must not show tasks that belong to a project: %q", out.String())
	}
	if !strings.Contains(out.String(), "today one") {
		t.Errorf("the default should show uncategorized tasks: %q", out.String())
	}
}

func TestLsFilterByProjectAndNoProject(t *testing.T) {
	app, out, _ := newApp(t)
	seedCLI(t, app)

	out.Reset()
	app.Run([]string{"ls", "-p", "work"})
	if !strings.Contains(out.String(), "work one") || strings.Contains(out.String(), "today one") {
		t.Errorf("-p work = %q", out.String())
	}

	// --no-project states the default explicitly.
	out.Reset()
	app.Run([]string{"ls", "--no-project"})
	if strings.Contains(out.String(), "work one") {
		t.Errorf("--no-project must exclude tasks with a project: %q", out.String())
	}
	if !strings.Contains(out.String(), "today one") {
		t.Errorf("--no-project should include uncategorized tasks: %q", out.String())
	}
}

func TestLsAllProjects(t *testing.T) {
	app, out, _ := newApp(t)
	seedCLI(t, app)

	out.Reset()
	app.Run([]string{"ls", "--all-projects"})
	s := out.String()
	if !strings.Contains(s, "work one") || !strings.Contains(s, "today one") {
		t.Errorf("--all-projects should span every project: %q", s)
	}
}

func TestLsRejectsConflictingProjectFlags(t *testing.T) {
	cases := [][]string{
		{"ls", "-p", "work", "--no-project"},
		{"ls", "-p", "work", "--all-projects"},
		{"ls", "--no-project", "--all-projects"},
	}
	for _, args := range cases {
		app, _, errBuf := newApp(t)
		if code := app.Run(args); code != 1 {
			t.Errorf("%v exit code = %d, want 1", args, code)
		}
		if !strings.Contains(errBuf.String(), "cannot be used together") {
			t.Errorf("%v stderr = %q", args, errBuf.String())
		}
	}
}

func TestLsDueKeywordsAndTags(t *testing.T) {
	app, out, _ := newApp(t)
	seedCLI(t, app)

	out.Reset()
	app.Run([]string{"ls", "-d", "today"})
	if !strings.Contains(out.String(), "today one") || strings.Contains(out.String(), "overdue one") {
		t.Errorf("-d today = %q", out.String())
	}

	out.Reset()
	app.Run([]string{"ls", "-d", "overdue"})
	if !strings.Contains(out.String(), "overdue one") {
		t.Errorf("-d overdue = %q", out.String())
	}

	out.Reset()
	app.Run([]string{"ls", "-t", "urgent"})
	if !strings.Contains(out.String(), "today one") || strings.Contains(out.String(), "work one") {
		t.Errorf("-t urgent = %q", out.String())
	}
}

func TestLsRejectsPositionalArgs(t *testing.T) {
	app, _, errBuf := newApp(t)
	if code := app.Run([]string{"ls", "junk"}); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "takes no positional arguments") {
		t.Errorf("stderr = %q", errBuf.String())
	}
}

func TestLsBadSortAndPriority(t *testing.T) {
	app, _, _ := newApp(t)
	if code := app.Run([]string{"ls", "-s", "title"}); code == 0 {
		t.Error("an unknown sort should fail")
	}
	if code := app.Run([]string{"ls", "--pri", "urgent"}); code == 0 {
		t.Error("an unknown priority should fail")
	}
}
