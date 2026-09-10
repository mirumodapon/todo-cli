// Package store persists tasks.
package store

import (
	"errors"
	"time"

	"todo.mirumo.net/internal/task"
)

// ErrNotFound reports that no task has the given id.
var ErrNotFound = errors.New("task not found")

// ProjectCount pairs a project with its open task count.
type ProjectCount struct {
	Path string
	Open int
}

// Store is the persistence interface. The CLI and the TUI know only this interface,
// so tests can swap in an in-memory implementation and never touch real data.
type Store interface {
	// Add inserts a task and returns it with its ID filled in.
	Add(t task.Task) (task.Task, error)
	// Get fetches one task by id, returning ErrNotFound when it does not exist.
	Get(id int64) (task.Task, error)
	// List queries by f; now resolves relative conditions such as today, week, and overdue.
	List(f task.Filter, now time.Time) ([]task.Task, error)
	// Update overwrites every field of t.ID, tags included.
	Update(t task.Task) error
	// Delete removes one task along with its tag links.
	Delete(id int64) error
	// SetDeleted soft deletes a task or brings it back. Delete is what destroys
	// a row; this only takes it out of the lists.
	SetDeleted(id int64, deleted bool, now time.Time) error
	// SetDone sets or clears the completed state.
	SetDone(id int64, done bool, now time.Time) error
	// Tags lists the tags referenced by at least one task.
	Tags() ([]string, error)
	// DataVersion reports a counter that moves when another connection writes.
	// It is how a long-running reader notices that someone else changed
	// something, without reading everything to find out.
	DataVersion() (int64, error)
	// Projects lists every project with its open task count.
	Projects() ([]ProjectCount, error)
	Close() error
}
