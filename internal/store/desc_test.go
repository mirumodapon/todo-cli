package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	"todo.mirumo.net/internal/task"
)

func TestDescriptionRoundTrips(t *testing.T) {
	s := newStore(t)
	in := sample()
	in.Desc = "line one\nline two"
	got, err := s.Add(in)
	if err != nil {
		t.Fatal(err)
	}
	back, err := s.Get(got.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.Desc != in.Desc {
		t.Errorf("desc = %q, want %q", back.Desc, in.Desc)
	}

	back.Desc = "rewritten"
	if err := s.Update(back); err != nil {
		t.Fatal(err)
	}
	if again, _ := s.Get(got.ID); again.Desc != "rewritten" {
		t.Errorf("desc after Update = %q", again.Desc)
	}

	ts, err := s.List(task.Filter{}, ref())
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) != 1 || ts[0].Desc != "rewritten" {
		t.Errorf("List should carry the description, got %+v", ts)
	}
}

// Undo reinserts the whole task, so it has to carry the description too.
func TestRestoreKeepsTheDescription(t *testing.T) {
	s := newStore(t)
	in := sample()
	in.Desc = "worth keeping"
	got, _ := s.Add(in)
	if err := s.Delete(got.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Restore(got); err != nil {
		t.Fatal(err)
	}
	back, err := s.Get(got.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.Desc != "worth keeping" {
		t.Errorf("desc = %q, want it restored", back.Desc)
	}
}

// A database written before the column existed must keep working: the schema is
// created with IF NOT EXISTS, which does nothing to a table that already exists.
func TestOpenAddsTheDescriptionColumnToAnOlderDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
CREATE TABLE tasks (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  title      TEXT    NOT NULL,
  project    TEXT    NOT NULL DEFAULT '',
  due        TEXT    NULL,
  priority   INTEGER NOT NULL DEFAULT 0,
  done_at    TEXT    NULL,
  created_at TEXT    NOT NULL,
  updated_at TEXT    NOT NULL
);
INSERT INTO tasks (title, created_at, updated_at)
VALUES ('from the old schema', '2026-08-29T15:00:00+08:00', '2026-08-29T15:00:00+08:00');`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	s, err := OpenSQLite(path)
	if err != nil {
		t.Fatalf("OpenSQLite on an older database: %v", err)
	}
	defer s.Close()

	ts, err := s.List(task.Filter{}, ref())
	if err != nil {
		t.Fatalf("List on an older database: %v", err)
	}
	if len(ts) != 1 || ts[0].Title != "from the old schema" {
		t.Fatalf("the existing row should survive, got %+v", ts)
	}
	if ts[0].Desc != "" {
		t.Errorf("desc = %q, want empty for a row written before the column", ts[0].Desc)
	}
	ts[0].Desc = "added later"
	if err := s.Update(ts[0]); err != nil {
		t.Fatalf("Update on an older database: %v", err)
	}
	if back, _ := s.Get(ts[0].ID); back.Desc != "added later" {
		t.Errorf("desc = %q, the migrated column should be writable", back.Desc)
	}
}
