package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeEditor writes a shell script that stands in for the user's editor. body
// receives the file to edit as "$1".
func fakeEditor(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fake-editor")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCommandPrefersVisualThenEditor(t *testing.T) {
	t.Setenv("VISUAL", "vis")
	t.Setenv("EDITOR", "ed")
	c := Command("/tmp/note")
	if got := strings.Join(c.Args, " "); !strings.Contains(got, "vis") || strings.Contains(got, "ed ") {
		t.Errorf("args = %v, VISUAL should win", c.Args)
	}
	if c.Args[len(c.Args)-1] != "/tmp/note" {
		t.Errorf("args = %v, the file should be passed as an argument", c.Args)
	}

	t.Setenv("VISUAL", "")
	c = Command("/tmp/note")
	if !strings.Contains(strings.Join(c.Args, " "), "ed") {
		t.Errorf("args = %v, EDITOR should be used when VISUAL is empty", c.Args)
	}
}

// With neither variable set there is still an editor: a machine without vi is
// rarer than a user who never set EDITOR.
func TestCommandFallsBackToVi(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	c := Command("/tmp/note")
	if !strings.Contains(strings.Join(c.Args, " "), "vi") {
		t.Errorf("args = %v, want a vi fallback", c.Args)
	}
}

// The editor is run through a shell, so EDITOR may carry arguments.
func TestCommandKeepsEditorArguments(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "code -w")
	c := Command("/tmp/note")
	if !strings.Contains(strings.Join(c.Args, " "), "code -w") {
		t.Errorf("args = %v, the arguments in EDITOR should survive", c.Args)
	}
}

func TestEditRoundTrips(t *testing.T) {
	t.Setenv("EDITOR", fakeEditor(t, `printf '%s and more\n\n' "$(cat "$1")" > "$1"`))
	got, err := Edit("description", "what was there")
	if err != nil {
		t.Fatal(err)
	}
	if got != "what was there and more" {
		t.Errorf("= %q; the editor should see the current text and its result comes back trimmed", got)
	}
}

// An editor that fails is an abort, not an instruction to save nothing.
func TestEditReportsAFailingEditor(t *testing.T) {
	t.Setenv("EDITOR", fakeEditor(t, "exit 3"))
	if _, err := Edit("description", "keep me"); err == nil {
		t.Error("a failing editor should be an error")
	}
}

func TestEditRemovesItsTempFile(t *testing.T) {
	// The stand-in editor records the path it was handed, somewhere the edit
	// itself will not clean up.
	rec := filepath.Join(t.TempDir(), "path")
	t.Setenv("EDITOR", fakeEditor(t, `printf 'x' > "$1"; printf '%s' "$1" > `+rec))
	if _, err := Edit("description", ""); err != nil {
		t.Fatal(err)
	}
	seen, err := os.ReadFile(rec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(string(seen)); !os.IsNotExist(err) {
		t.Errorf("%s should have been removed after the edit", seen)
	}
}
