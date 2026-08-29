// Package datearg parses due dates and renders them for people.
package datearg

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Day truncates a time to midnight in its own location.
func Day(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

var weekdays = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday,
	"wed": time.Wednesday, "thu": time.Thursday,
	"fri": time.Friday, "sat": time.Saturday,
}

var timeOfDay = regexp.MustCompile(`^([0-9]{1,2}):([0-9]{2})$`)

// Parse reads a user-supplied due date and reports whether a time of day came
// with it. Accepts today / tomorrow / yesterday, weekday abbreviations, +3d,
// +2w, and YYYY-MM-DD, each optionally followed by HH:MM. A bare HH:MM means
// today at that time. Without a time the result is local midnight.
func Parse(s string, now time.Time) (time.Time, bool, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	datePart, timePart := s, ""
	if i := strings.LastIndexAny(s, " \t"); i >= 0 {
		if tail := strings.TrimSpace(s[i+1:]); timeOfDay.MatchString(tail) {
			datePart, timePart = strings.TrimSpace(s[:i]), tail
		}
	} else if timeOfDay.MatchString(s) {
		datePart, timePart = "today", s
	}

	d, err := parseDay(datePart, now)
	if err != nil {
		return time.Time{}, false, err
	}
	if timePart == "" {
		return d, false, nil
	}
	m := timeOfDay.FindStringSubmatch(timePart)
	hh, _ := strconv.Atoi(m[1])
	mm, _ := strconv.Atoi(m[2])
	if hh > 23 || mm > 59 {
		return time.Time{}, false, fmt.Errorf("%q is not a valid time of day", timePart)
	}
	return time.Date(d.Year(), d.Month(), d.Day(), hh, mm, 0, 0, d.Location()), true, nil
}

// parseDay handles the date half, with no time of day involved.
func parseDay(s string, now time.Time) (time.Time, error) {
	base := Day(now)
	switch s {
	case "today":
		return base, nil
	case "tomorrow":
		return base.AddDate(0, 0, 1), nil
	case "yesterday":
		return base.AddDate(0, 0, -1), nil
	}
	if wd, ok := weekdays[s]; ok {
		// First matching day within the next seven, today included.
		for i := 0; i < 7; i++ {
			if d := base.AddDate(0, 0, i); d.Weekday() == wd {
				return d, nil
			}
		}
	}
	if strings.HasPrefix(s, "+") && len(s) > 2 {
		unit := s[len(s)-1]
		if n, err := strconv.Atoi(s[1 : len(s)-1]); err == nil {
			switch unit {
			case 'd':
				return base.AddDate(0, 0, n), nil
			case 'w':
				return base.AddDate(0, 0, 7*n), nil
			}
		}
	}
	if t, err := time.ParseInLocation("2006-01-02", s, now.Location()); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("cannot read date %q (try today, tomorrow, fri, +3d, 2026-09-01, optionally with a time such as \"today 15:00\")", s)
}

// Format renders a due date for display: "3d overdue", "today", a time of day
// when the task is due today, or a date. Only today gets a word; every other
// day is a date, which says which day without the reader having to count.
func Format(due time.Time, hasTime bool, now time.Time) string {
	d, base := Day(due), Day(now)
	diff := int(math.Round(d.Sub(base).Hours() / 24))
	// A time of day only earns its place on the day it matters.
	if hasTime && diff == 0 {
		return due.Format("15:04")
	}
	switch {
	case diff < 0:
		return fmt.Sprintf("%dd overdue", -diff)
	case diff == 0:
		return "today"
	case d.Year() == base.Year():
		return d.Format("01-02")
	default:
		return d.Format("2006-01-02")
	}
}
