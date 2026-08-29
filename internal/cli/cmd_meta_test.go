package cli

import (
	"strings"
	"testing"
)

func TestProjectsListsCountsAndUncategorized(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "global one"})
	app.Run([]string{"add", "work A", "-p", "/p/work"})
	app.Run([]string{"add", "work B", "-p", "/p/work"})
	app.Run([]string{"done", "3"})
	out.Reset()

	if code := app.Run([]string{"projects"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	s := out.String()
	if !strings.Contains(s, "(uncategorized)") {
		t.Errorf("an empty project should be shown as (uncategorized): %q", s)
	}
	if !strings.Contains(s, "work") || !strings.Contains(s, "1 open") {
		t.Errorf("it should show the basename and the open count: %q", s)
	}
	if !strings.Contains(s, "/p/work") {
		t.Errorf("a real project should carry its full path: %q", s)
	}
}

func TestProjectsEmpty(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"projects"})
	if !strings.Contains(out.String(), "No tasks yet") {
		t.Errorf("= %q", out.String())
	}
}

func TestTags(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "x", "-t", "shopping", "-t", "chores"})
	out.Reset()
	app.Run([]string{"tags"})
	s := out.String()
	if !strings.Contains(s, "@chores") || !strings.Contains(s, "@shopping") {
		t.Errorf("= %q", s)
	}
}

func TestTagsEmpty(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"tags"})
	if !strings.Contains(out.String(), "No tags yet") {
		t.Errorf("= %q", out.String())
	}
}

func TestMetaCommandsRejectArgs(t *testing.T) {
	for _, cmd := range []string{"projects", "tags"} {
		app, _, errBuf := newApp(t)
		if code := app.Run([]string{cmd, "junk"}); code != 1 {
			t.Errorf("%s exit code = %d, want 1", cmd, code)
		}
		if !strings.Contains(errBuf.String(), "takes no arguments") {
			t.Errorf("%s stderr = %q", cmd, errBuf.String())
		}
	}
}
