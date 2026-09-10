package store

import (
	"testing"

	"todo.mirumo.net/internal/task"
)

// A soft delete takes the task out of every list without taking it out of the
// database, so a mistake stays recoverable after the fact.
func TestSoftDeleteHidesWithoutDestroying(t *testing.T) {
	s := newStore(t)
	seed(t, s)
	ids := map[string]int64{}
	all, _ := s.List(task.Filter{}, ref())
	for _, ti := range all {
		ids[ti.Title] = ti.ID
	}

	if err := s.SetDeleted(ids["today one"], true, ref()); err != nil {
		t.Fatal(err)
	}
	got, err := s.List(task.Filter{}, ref())
	if err != nil {
		t.Fatal(err)
	}
	for _, ti := range got {
		if ti.Title == "today one" {
			t.Fatalf("a deleted task should be out of the list: %v", titles(got))
		}
	}

	// It is still there to be found, and says what happened to it.
	back, err := s.Get(ids["today one"])
	if err != nil {
		t.Fatalf("Get on a deleted task: %v", err)
	}
	if !back.Deleted() {
		t.Error("it should know it is deleted")
	}
	if back.DeletedAt == nil || !back.DeletedAt.Equal(ref()) {
		t.Errorf("deleted_at = %v, want when it happened", back.DeletedAt)
	}
}

func TestOnlyDeletedListsTheBin(t *testing.T) {
	s := newStore(t)
	seed(t, s)
	all, _ := s.List(task.Filter{}, ref())
	if err := s.SetDeleted(all[0].ID, true, ref()); err != nil {
		t.Fatal(err)
	}
	got, err := s.List(task.Filter{OnlyDeleted: true}, ref())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != all[0].ID {
		t.Errorf("= %v, want just the deleted one", titles(got))
	}

	both, err := s.List(task.Filter{IncludeDeleted: true}, ref())
	if err != nil {
		t.Fatal(err)
	}
	if len(both) != len(all) {
		t.Errorf("IncludeDeleted should list everything, got %d of %d", len(both), len(all))
	}
}

func TestUndeleteBringsItBack(t *testing.T) {
	s := newStore(t)
	seed(t, s)
	all, _ := s.List(task.Filter{}, ref())
	id := all[0].ID
	if err := s.SetDeleted(id, true, ref()); err != nil {
		t.Fatal(err)
	}
	if err := s.SetDeleted(id, false, ref()); err != nil {
		t.Fatal(err)
	}
	back, err := s.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if back.Deleted() {
		t.Error("it should be back")
	}
	got, _ := s.List(task.Filter{}, ref())
	if len(got) != len(all) {
		t.Errorf("the list should be whole again, got %d of %d", len(got), len(all))
	}
}

func TestSetDeletedOnAnUnknownID(t *testing.T) {
	s := newStore(t)
	if err := s.SetDeleted(99, true, ref()); err == nil {
		t.Error("an id that does not exist should be an error")
	}
}

// The counts and menus are about tasks you can still act on, so a deleted one
// stops contributing to them.
func TestProjectsAndTagsIgnoreDeleted(t *testing.T) {
	s := newStore(t)
	seed(t, s)
	added, err := s.Add(task.Task{
		Title: "only mine", Project: "/p/work", Tags: []string{"exclusive"},
		CreatedAt: ref(), UpdatedAt: ref(),
	})
	if err != nil {
		t.Fatal(err)
	}
	openIn := func(path string) int {
		t.Helper()
		ps, err := s.Projects()
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range ps {
			if p.Path == path {
				return p.Open
			}
		}
		return -1
	}
	before := openIn("/p/work")
	if err := s.SetDeleted(added.ID, true, ref()); err != nil {
		t.Fatal(err)
	}
	if after := openIn("/p/work"); after != before-1 {
		t.Errorf("open count %d -> %d, want one fewer", before, after)
	}
	tags, err := s.Tags()
	if err != nil {
		t.Fatal(err)
	}
	for _, tag := range tags {
		if tag == "exclusive" {
			t.Errorf("a tag only a deleted task carries should not be listed: %v", tags)
		}
	}
}
