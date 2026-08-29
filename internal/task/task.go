// Package task defines the todo domain types. It performs no IO.
package task

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Priority ranks a task. Values ascend from low to high so SQL can ORDER BY it directly.
type Priority int

const (
	PriNone Priority = iota
	PriLow
	PriMed
	PriHigh
)

// ParsePriority parses user input. An empty string means unset.
func ParsePriority(s string) (Priority, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return PriNone, nil
	case "low":
		return PriLow, nil
	case "med":
		return PriMed, nil
	case "high":
		return PriHigh, nil
	}
	return PriNone, fmt.Errorf("unknown priority %q (use low, med, high)", s)
}

// String returns the identifier used on the command line and in listings.
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

// Task is one todo item. An empty Project means globally uncategorized.
type Task struct {
	ID        int64
	Title     string
	Project   string
	Due       *time.Time
	Priority  Priority
	DoneAt    *time.Time
	Tags      []string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Done reports whether the task is complete.
func (t Task) Done() bool { return t.DoneAt != nil }

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
	SortDue SortBy = iota
	SortPriority
	SortCreated
)

// ParseSortBy parses the value of -s.
func ParseSortBy(s string) (SortBy, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "due":
		return SortDue, nil
	case "pri":
		return SortPriority, nil
	case "created":
		return SortCreated, nil
	}
	return SortDue, fmt.Errorf("unknown sort %q (use due, pri, created)", s)
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

// Filter describes one query. A nil Project means no project filtering;
// a pointer to an empty string means uncategorized tasks only.
type Filter struct {
	Project     *string
	Tags        []string
	DueRange    DueRange
	DueOn       time.Time
	Priority    *Priority
	Search      string
	IncludeDone bool
	OnlyDone    bool
	Sort        SortBy
}
