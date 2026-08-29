// Package datearg parses due dates and renders them for people.
package datearg

import (
	"fmt"
	"math"
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

var zhWeekday = [7]string{"週日", "週一", "週二", "週三", "週四", "週五", "週六"}

// Parse reads a user-supplied due date and returns local midnight on that day.
// Accepts today / tomorrow / yesterday, weekday abbreviations, +3d, +2w, and YYYY-MM-DD.
func Parse(s string, now time.Time) (time.Time, error) {
	s = strings.ToLower(strings.TrimSpace(s))
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
	return time.Time{}, fmt.Errorf("看不懂的日期：%q（可用 today、tomorrow、fri、+3d、2026-09-01）", s)
}

// Format renders a due date for display: overdue, today, tomorrow, a weekday, or a date.
func Format(due, now time.Time) string {
	d, base := Day(due), Day(now)
	diff := int(math.Round(d.Sub(base).Hours() / 24))
	switch {
	case diff < 0:
		return fmt.Sprintf("逾期 %d 天", -diff)
	case diff == 0:
		return "今天"
	case diff == 1:
		return "明天"
	case diff < 7:
		return zhWeekday[int(d.Weekday())]
	case d.Year() == base.Year():
		return d.Format("01-02")
	default:
		return d.Format("2006-01-02")
	}
}
