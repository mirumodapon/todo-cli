package cli

import (
	"strings"
	"testing"
)

func TestDoneAndUndone(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "buy milk"})
	out.Reset()

	if code := app.Run([]string{"done", "1"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out.String(), "done #1: buy milk") {
		t.Errorf("stdout = %q", out.String())
	}
	got, _ := app.Store.Get(1)
	if !got.Done() {
		t.Error("should be done")
	}

	out.Reset()
	app.Run([]string{"undone", "1"})
	if !strings.Contains(out.String(), "reopened #1") {
		t.Errorf("stdout = %q", out.String())
	}
	got, _ = app.Store.Get(1)
	if got.Done() {
		t.Error("should be open")
	}
}

func TestDoneAcceptsMultipleIDs(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "a"})
	app.Run([]string{"add", "b"})
	out.Reset()
	if code := app.Run([]string{"done", "1", "2"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if strings.Count(out.String(), "done") != 2 {
		t.Errorf("stdout = %q, want two lines", out.String())
	}
}

func TestMarkMissingIDNamesTheID(t *testing.T) {
	app, _, errBuf := newApp(t)
	if code := app.Run([]string{"done", "42"}); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "#42") {
		t.Errorf("the error should name the id: %q", errBuf.String())
	}
}

func TestRm(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "buy milk"})
	out.Reset()
	if code := app.Run([]string{"rm", "1"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out.String(), "deleted #1: buy milk") {
		t.Errorf("stdout = %q", out.String())
	}
	if _, err := app.Store.Get(1); err == nil {
		t.Error("it should be gone")
	}
}

func TestMarkRequiresID(t *testing.T) {
	for _, cmd := range []string{"done", "undone", "rm"} {
		app, _, errBuf := newApp(t)
		if code := app.Run([]string{cmd}); code != 1 {
			t.Errorf("%s without an id: exit code = %d, want 1", cmd, code)
		}
		if errBuf.Len() == 0 {
			t.Errorf("%s should print an error", cmd)
		}
	}
}
