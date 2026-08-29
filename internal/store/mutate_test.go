package store

import (
	"errors"
	"testing"

	"todo.mirumo.net/internal/task"
)

func TestSetDoneAndUndone(t *testing.T) {
	s := newStore(t)
	got, _ := s.Add(sample())
	if err := s.SetDone(got.ID, true, ref()); err != nil {
		t.Fatalf("SetDone: %v", err)
	}
	back, _ := s.Get(got.ID)
	if !back.Done() {
		t.Fatal("should be done")
	}
	if back.DoneAt.Format("2006-01-02") != "2026-08-29" {
		t.Errorf("done_at = %v, want the completion time recorded", back.DoneAt)
	}
	if err := s.SetDone(got.ID, false, ref()); err != nil {
		t.Fatal(err)
	}
	back, _ = s.Get(got.ID)
	if back.Done() {
		t.Error("DoneAt should be nil after reopening")
	}
}

func TestSetDoneMissingID(t *testing.T) {
	s := newStore(t)
	if err := s.SetDone(42, true, ref()); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateOverwritesFieldsAndTags(t *testing.T) {
	s := newStore(t)
	got, _ := s.Add(sample())
	got.Title = "buy soy milk"
	got.Project = ""
	got.Due = nil
	got.Priority = task.PriLow
	got.Tags = []string{"breakfast"}
	if err := s.Update(got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	back, _ := s.Get(got.ID)
	if back.Title != "buy soy milk" || back.Project != "" || back.Due != nil || back.Priority != task.PriLow {
		t.Errorf("fields were not updated: %+v", back)
	}
	if len(back.Tags) != 1 || back.Tags[0] != "breakfast" {
		t.Errorf("tags = %v, the whole set should be replaced with [breakfast]", back.Tags)
	}
}

func TestUpdateMissingID(t *testing.T) {
	s := newStore(t)
	if err := s.Update(task.Task{ID: 7, Title: "x", CreatedAt: ref(), UpdatedAt: ref()}); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDeleteRemovesTagLinks(t *testing.T) {
	s := newStore(t)
	got, _ := s.Add(sample())
	if err := s.Delete(got.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(got.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	tags, err := s.Tags()
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 0 {
		t.Errorf("Tags() = %v, want none: only referenced tags are listed", tags)
	}
}

func TestRestoreReusesOriginalID(t *testing.T) {
	s := newStore(t)
	got, _ := s.Add(sample())
	original, _ := s.Get(got.ID)
	if err := s.Delete(got.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Restore(original); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	back, err := s.Get(original.ID)
	if err != nil {
		t.Fatalf("the original id should fetch it again after restore: %v", err)
	}
	if back.Title != original.Title || len(back.Tags) != len(original.Tags) {
		t.Errorf("restored content does not match: %+v", back)
	}
}

func TestTagsListsOnlyReferenced(t *testing.T) {
	s := newStore(t)
	seed(t, s)
	tags, err := s.Tags()
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 || tags[0] != "misc" || tags[1] != "urgent" {
		t.Errorf("= %v, want [misc urgent] sorted by name", tags)
	}
}

func TestProjectsCountsOpenTasks(t *testing.T) {
	s := newStore(t)
	ids := seed(t, s)
	if err := s.SetDone(ids["work one"], true, ref()); err != nil {
		t.Fatal(err)
	}
	ps, err := s.Projects()
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 2 {
		t.Fatalf("= %+v, want two projects, uncategorized included", ps)
	}
	if ps[0].Path != "" || ps[0].Open != 3 {
		t.Errorf("ps[0] = %+v, want uncategorized with 3 open", ps[0])
	}
	if ps[1].Path != "/p/work" || ps[1].Open != 1 {
		t.Errorf("ps[1] = %+v, want /p/work with 1 open, the other being done", ps[1])
	}
}
