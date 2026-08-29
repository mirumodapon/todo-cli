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
		t.Fatalf("OpenSQLite：%v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func sample() task.Task {
	return task.Task{
		Title:     "買牛奶",
		Project:   "/Users/me/Projects/home",
		Due:       day(2026, 9, 1),
		Priority:  task.PriHigh,
		Tags:      []string{"購物", "家務"},
		CreatedAt: ref(),
		UpdatedAt: ref(),
	}
}

func TestAddAssignsIDAndRoundTrips(t *testing.T) {
	s := newStore(t)
	got, err := s.Add(sample())
	if err != nil {
		t.Fatalf("Add：%v", err)
	}
	if got.ID == 0 {
		t.Fatal("Add 應該回填 ID")
	}
	back, err := s.Get(got.ID)
	if err != nil {
		t.Fatalf("Get：%v", err)
	}
	if back.Title != "買牛奶" || back.Project != "/Users/me/Projects/home" {
		t.Errorf("標題/專案沒存對：%+v", back)
	}
	if back.Priority != task.PriHigh {
		t.Errorf("priority = %v，預期 high", back.Priority)
	}
	if back.Due == nil || back.Due.Format("2006-01-02") != "2026-09-01" {
		t.Errorf("due = %v，預期 2026-09-01", back.Due)
	}
	if back.Done() {
		t.Error("新增的任務不該是已完成")
	}
	if len(back.Tags) != 2 {
		t.Errorf("tags = %v，預期兩個", back.Tags)
	}
}

func TestAddAcceptsEmptyProjectAndNilDue(t *testing.T) {
	s := newStore(t)
	in := task.Task{Title: "繳房租", CreatedAt: ref(), UpdatedAt: ref()}
	got, err := s.Add(in)
	if err != nil {
		t.Fatalf("Add：%v", err)
	}
	back, err := s.Get(got.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.Project != "" {
		t.Errorf("project = %q，全域未分類應為空字串", back.Project)
	}
	if back.Due != nil {
		t.Errorf("due = %v，預期 nil", back.Due)
	}
	if len(back.Tags) != 0 {
		t.Errorf("tags = %v，預期空", back.Tags)
	}
}

func TestGetMissingReturnsErrNotFound(t *testing.T) {
	s := newStore(t)
	_, err := s.Get(999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v，預期 ErrNotFound", err)
	}
}

func TestIDsAreNotReused(t *testing.T) {
	s := newStore(t)
	a, _ := s.Add(sample())
	if err := s.Delete(a.ID); err != nil {
		t.Fatalf("Delete：%v", err)
	}
	b, _ := s.Add(sample())
	if b.ID == a.ID {
		t.Errorf("id 被重用了（%d），AUTOINCREMENT 應該保證不重用", b.ID)
	}
}
