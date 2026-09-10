package cli

import (
	"strings"
	"testing"

	"todo.mirumo.net/internal/task"
)

// rm takes a task out of the lists without destroying it: a delete you cannot
// take back is a poor default for a keystroke away from every other one.
func TestRmIsSoftByDefault(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "buy milk"})
	out.Reset()

	if code := app.Run([]string{"rm", "1"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out.String(), "deleted #1") {
		t.Errorf("it should say what happened: %q", out.String())
	}
	got, err := app.Store.Get(1)
	if err != nil {
		t.Fatalf("the task should still be there: %v", err)
	}
	if !got.Deleted() {
		t.Error("and marked deleted")
	}

	out.Reset()
	app.Run([]string{"ls"})
	if strings.Contains(out.String(), "buy milk") {
		t.Errorf("a deleted task should be out of the listing: %q", out.String())
	}
}

func TestRmForceDestroysTheRow(t *testing.T) {
	app, _, _ := newApp(t)
	app.Run([]string{"add", "buy milk"})

	if code := app.Run([]string{"rm", "-f", "1"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if _, err := app.Store.Get(1); err == nil {
		t.Error("-f should leave nothing behind")
	}
}

// A second rm on something already in the bin is not a second delete.
func TestRmTwiceSaysSo(t *testing.T) {
	app, out, errBuf := newApp(t)
	app.Run([]string{"add", "buy milk"})
	app.Run([]string{"rm", "1"})
	out.Reset()

	if code := app.Run([]string{"rm", "1"}); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "already") {
		t.Errorf("it should say it is already deleted: %q", errBuf.String())
	}
	// But -f still purges it.
	if code := app.Run([]string{"rm", "-f", "1"}); code != 0 {
		t.Errorf("-f on a deleted task should purge it, exit code = %d", code)
	}
}

func TestLsDeletedShowsTheBin(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "buy milk"})
	app.Run([]string{"add", "still here"})
	app.Run([]string{"rm", "1"})
	out.Reset()

	app.Run([]string{"ls", "--deleted"})
	s := out.String()
	if !strings.Contains(s, "buy milk") {
		t.Errorf("--deleted should list what was removed: %q", s)
	}
	if strings.Contains(s, "still here") {
		t.Errorf("and only that: %q", s)
	}
}

func TestRestoreBringsItBack(t *testing.T) {
	app, out, errBuf := newApp(t)
	app.Run([]string{"add", "buy milk", "-t", "shopping"})
	app.Run([]string{"rm", "1"})
	out.Reset()

	if code := app.Run([]string{"restore", "1"}); code != 0 {
		t.Fatalf("exit code = %d: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "restored #1") {
		t.Errorf("it should say so: %q", out.String())
	}
	got, err := app.Store.Get(1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Deleted() {
		t.Error("it should be back")
	}
	if len(got.Tags) != 1 {
		t.Errorf("with everything it had: %+v", got)
	}
}

func TestRestoreRejectsWhatItCannot(t *testing.T) {
	app, _, errBuf := newApp(t)
	app.Run([]string{"add", "buy milk"})

	if code := app.Run([]string{"restore"}); code != 1 {
		t.Errorf("exit code = %d, want 1 with no id", code)
	}
	errBuf.Reset()
	if code := app.Run([]string{"restore", "99"}); code != 1 {
		t.Errorf("exit code = %d, want 1 for an unknown id", code)
	}
	if !strings.Contains(errBuf.String(), "#99") {
		t.Errorf("the error should name the id: %q", errBuf.String())
	}
}

// details still finds a deleted task, and says that is what it is: you have to
// be able to look before deciding whether to bring it back.
func TestDetailsShowsThatATaskIsDeleted(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "buy milk"})
	app.Run([]string{"rm", "1"})
	out.Reset()

	if code := app.Run([]string{"details", "1"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out.String(), "deleted") {
		t.Errorf("details should say the task is deleted:\n%s", out.String())
	}
}

var _ = task.Task{}
