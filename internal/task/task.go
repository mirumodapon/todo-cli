// Package task defines the domain types. It performs no IO.
package task

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"todo.mirumo.net/internal/datearg"
)

// Priority ranks a task. Values ascend from low to high so SQL can ORDER BY it directly.
type Priority int

const (
	PriNone Priority = iota
	PriLow
	PriMed
	PriHigh
)

// ParsePriority parses user input. An empty string means unset. Both the words
// and the marks a listing shows are accepted, so what you read back is
// something you can type.
func ParsePriority(s string) (Priority, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return PriNone, nil
	case "low", "!":
		return PriLow, nil
	case "med", "!!":
		return PriMed, nil
	case "high", "!!!":
		return PriHigh, nil
	}
	return PriNone, fmt.Errorf("unknown priority %q (use low, med, high, or !, !!, !!!)", s)
}

// Marks is how a priority is shown in a listing: one mark per level, so
// urgency reads as a shape before it reads as a word.
func (p Priority) Marks() string {
	switch p {
	case PriLow:
		return "!"
	case PriMed:
		return "!!"
	case PriHigh:
		return "!!!"
	}
	return ""
}

// String returns the identifier used on the command line.
func (p Priority) String() string {
	switch p {
	case PriLow:
		return "low"
	case PriMed:
		return "med"
	case PriHigh:
		return "high"
	}
	return ""
}

// Task is one piece of work. An empty Project means globally uncategorized.
type Task struct {
	ID    int64
	Title string
	// Desc is the long form of the task, free text over as many lines as it
	// takes. Listings show only the title; Desc is what the detail view is for.
	Desc    string
	Project string
	Due     *time.Time
	// DueHasTime distinguishes "due on that day" from "due at that moment".
	// Midnight is a legitimate time of day, so the zero clock cannot carry it.
	DueHasTime bool
	Priority   Priority
	DoneAt     *time.Time
	// DeletedAt marks a task removed from every list without being removed from
	// the database. rm sets it; rm -f is what actually destroys a row.
	DeletedAt *time.Time
	Tags      []string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Done reports whether the task is complete.
func (t Task) Done() bool { return t.DoneAt != nil }

// Deleted reports whether the task has been soft deleted.
func (t Task) Deleted() bool { return t.DeletedAt != nil }

// ValidateTitle trims surrounding whitespace and rejects an empty result.
func ValidateTitle(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("title cannot be empty")
	}
	return s, nil
}

// NormalizeTags trims, drops empties, removes duplicates, and keeps first-seen order.
func NormalizeTags(tags []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// SortBy selects the listing order.
type SortBy int

const (
	// SortID is the zero value, and so the default: the order tasks were added
	// is the one a reader can predict from the ids in front of them.
	SortID SortBy = iota
	SortDue
	SortPriority
)

// There is no ordering by creation time: ids ascend with it, so "created" and
// "id" would be two names for one list.

// SortCount is how many orderings there are, for anything cycling through them.
const SortCount = int(SortPriority) + 1

// ParseSortBy parses the value of -s.
func ParseSortBy(s string) (SortBy, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "id":
		return SortID, nil
	case "due":
		return SortDue, nil
	case "pri":
		return SortPriority, nil
	}
	return SortID, fmt.Errorf("unknown sort %q (use id, due, pri)", s)
}

// DueRange narrows a query by due date.
type DueRange int

const (
	DueAny DueRange = iota
	DueToday
	DueWeek
	DueOverdue
	DueOn
)

// ParseDueFilter reads what a user writes for a due-date filter: the three
// range words, or a date in any form datearg takes. Both the CLI and the MCP
// server go through here, so the two cannot come to disagree about what
// "week" means.
//
// Filtering is by day, so a time of day narrows nothing and is dropped.
func ParseDueFilter(s string, now time.Time) (DueRange, time.Time, error) {
	switch v := strings.ToLower(strings.TrimSpace(s)); v {
	case "":
		return DueAny, time.Time{}, nil
	case "today":
		return DueToday, time.Time{}, nil
	case "week":
		return DueWeek, time.Time{}, nil
	case "overdue":
		return DueOverdue, time.Time{}, nil
	default:
		d, _, err := datearg.Parse(v, now)
		if err != nil {
			return DueAny, time.Time{}, err
		}
		return DueOn, d, nil
	}
}

// Filter describes one query. A nil Project means no project filtering;
// a pointer to an empty string means uncategorized tasks only.
type Filter struct {
	Project *string
	Tags    []string
	// Untagged narrows to tasks with no tags at all, which no entry in Tags
	// can express.
	Untagged    bool
	DueRange    DueRange
	DueOn       time.Time
	Priority    *Priority
	Search      string
	IncludeDone bool
	OnlyDone    bool
	// A soft-deleted task is out of every list unless one of these says
	// otherwise, which is what makes deleting recoverable rather than final.
	IncludeDeleted bool
	OnlyDeleted    bool
	Sort           SortBy
	// Reverse flips whatever Sort chose, every term of it.
	Reverse bool
}
