package store

import (
	"errors"
	"testing"
	"time"

	"todo.mirumo.net/internal/task"
)

func ref() time.Time { return time.Date(2026, 8, 29, 15, 0, 0, 0, time.Local) }

func day(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	return &t
}

// newStore opens an in-memory store; tests never touch ~/.todo.
func newStore(t *testing.T) Store {
	t.Helper()
	s, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func sample() task.Task {
	return task.Task{
		Title:     "buy milk",
		Project:   "/Users/me/Projects/home",
		Due:       day(2026, 9, 1),
		Priority:  task.PriHigh,
		Tags:      []string{"shopping", "chores"},
		CreatedAt: ref(),
		UpdatedAt: ref(),
	}
}

func TestAddAssignsIDAndRoundTrips(t *testing.T) {
	s := newStore(t)
	got, err := s.Add(sample())
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if got.ID == 0 {
		t.Fatal("Add should fill in the ID")
	}
	back, err := s.Get(got.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if back.Title != "buy milk" || back.Project != "/Users/me/Projects/home" {
		t.Errorf("title or project stored wrong: %+v", back)
	}
	if back.Priority != task.PriHigh {
		t.Errorf("priority = %v, want high", back.Priority)
	}
	if back.Due == nil || back.Due.Format("2006-01-02") != "2026-09-01" {
		t.Errorf("due = %v, want 2026-09-01", back.Due)
	}
	if back.Done() {
		t.Error("a new task must not start out done")
	}
	if len(back.Tags) != 2 {
		t.Errorf("tags = %v, want two", back.Tags)
	}
}

func TestAddAcceptsEmptyProjectAndNilDue(t *testing.T) {
	s := newStore(t)
	in := task.Task{Title: "pay rent", CreatedAt: ref(), UpdatedAt: ref()}
	got, err := s.Add(in)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	back, err := s.Get(got.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.Project != "" {
		t.Errorf("project = %q, uncategorized should be the empty string", back.Project)
	}
	if back.Due != nil {
		t.Errorf("due = %v, want nil", back.Due)
	}
	if len(back.Tags) != 0 {
		t.Errorf("tags = %v, want none", back.Tags)
	}
}

func TestGetMissingReturnsErrNotFound(t *testing.T) {
	s := newStore(t)
	_, err := s.Get(999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestIDsAreNotReused(t *testing.T) {
	s := newStore(t)
	a, _ := s.Add(sample())
	if err := s.Delete(a.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	b, _ := s.Add(sample())
	if b.ID == a.ID {
		t.Errorf("id %d was reused; AUTOINCREMENT must never reuse one", b.ID)
	}
}

func TestDueTimeOfDayRoundTrips(t *testing.T) {
	s := newStore(t)
	at := time.Date(2026, 9, 1, 15, 4, 0, 0, time.Local)
	in := sample()
	in.Due, in.DueHasTime = &at, true

	got, err := s.Add(in)
	if err != nil {
		t.Fatal(err)
	}
	back, err := s.Get(got.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !back.DueHasTime {
		t.Fatal("the stored task should remember it carries a time of day")
	}
	if back.Due.Format("2006-01-02 15:04") != "2026-09-01 15:04" {
		t.Errorf("due = %v, want 2026-09-01 15:04", back.Due)
	}
}

func TestDateOnlyDueStaysDateOnly(t *testing.T) {
	s := newStore(t)
	got, err := s.Add(sample())
	if err != nil {
		t.Fatal(err)
	}
	back, _ := s.Get(got.ID)
	if back.DueHasTime {
		t.Error("a date-only due must not come back claiming a time of day")
	}
	if back.Due.Hour() != 0 || back.Due.Minute() != 0 {
		t.Errorf("due = %v, want midnight", back.Due)
	}
}
