package datearg

import (
	"testing"
	"time"
)

// 2026-08-29 is a Saturday.
func ref() time.Time {
	return time.Date(2026, 8, 29, 15, 4, 5, 0, time.Local)
}

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"today", "2026-08-29"},
		{"tomorrow", "2026-08-30"},
		{"yesterday", "2026-08-28"},
		{"sat", "2026-08-29"},
		{"mon", "2026-08-31"},
		{"+3d", "2026-09-01"},
		{"+2w", "2026-09-12"},
		{"2026-12-25", "2026-12-25"},
		{"  TOMORROW  ", "2026-08-30"},
	}
	for _, c := range cases {
		got, hasTime, err := Parse(c.in, ref())
		if err != nil {
			t.Errorf("Parse(%q) unexpected error: %v", c.in, err)
			continue
		}
		if got.Format("2006-01-02") != c.want {
			t.Errorf("Parse(%q) = %s, want %s", c.in, got.Format("2006-01-02"), c.want)
		}
		if hasTime {
			t.Errorf("Parse(%q) reported a time of day where none was given", c.in)
		}
	}
}

func TestParseTimeOfDay(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"today 15:00", "2026-08-29 15:00"},
		{"2026-09-01 14:30", "2026-09-01 14:30"},
		{"tomorrow 9:05", "2026-08-30 09:05"},
		{"fri 08:00", "2026-09-04 08:00"},
		{"+3d 18:00", "2026-09-01 18:00"},
		{"18:00", "2026-08-29 18:00"},
		{"  TODAY   07:30 ", "2026-08-29 07:30"},
	}
	for _, c := range cases {
		got, hasTime, err := Parse(c.in, ref())
		if err != nil {
			t.Errorf("Parse(%q) unexpected error: %v", c.in, err)
			continue
		}
		if !hasTime {
			t.Errorf("Parse(%q) should report a time of day", c.in)
		}
		if got.Format("2006-01-02 15:04") != c.want {
			t.Errorf("Parse(%q) = %s, want %s", c.in, got.Format("2006-01-02 15:04"), c.want)
		}
	}
}

func TestParseRejectsBadTimes(t *testing.T) {
	for _, in := range []string{"today 25:00", "today 12:60", "today 1500", "today :30"} {
		if _, _, err := Parse(in, ref()); err == nil {
			t.Errorf("Parse(%q) should fail", in)
		}
	}
}

func TestParseReturnsMidnight(t *testing.T) {
	got, _, err := Parse("today", ref())
	if err != nil {
		t.Fatal(err)
	}
	if got.Hour() != 0 || got.Minute() != 0 || got.Second() != 0 {
		t.Errorf("= %v, want midnight of that day", got)
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	for _, in := range []string{"someday", "2026-13-45", "+3x", "", "+d"} {
		if _, _, err := Parse(in, ref()); err == nil {
			t.Errorf("Parse(%q) should fail", in)
		}
	}
}

// Weekday abbreviations are still accepted as input; they are simply never
// produced as output.
var weekdayNames = [7]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

func TestFormatNeverUsesWeekdayNames(t *testing.T) {
	for i := 0; i < 14; i++ {
		due := ref().AddDate(0, 0, i)
		got := Format(due, false, ref())
		for _, name := range weekdayNames {
			if got == name {
				t.Errorf("Format(%s) = %q, weekday names are not used for display", due.Format("2006-01-02"), got)
			}
		}
	}
}

func TestFormatShowsTimeOnlyForToday(t *testing.T) {
	at := func(day string, hh, mm int) time.Time {
		d, err := time.ParseInLocation("2006-01-02", day, time.Local)
		if err != nil {
			t.Fatal(err)
		}
		return d.Add(time.Duration(hh)*time.Hour + time.Duration(mm)*time.Minute)
	}
	if got := Format(at("2026-08-29", 15, 4), true, ref()); got != "15:04" {
		t.Errorf("= %q, a task due today should show its time", got)
	}
	if got := Format(at("2026-08-30", 15, 4), true, ref()); got != "tomorrow" {
		t.Errorf("= %q, only today shows a time", got)
	}
	if got := Format(at("2026-08-29", 0, 0), false, ref()); got != "today" {
		t.Errorf("= %q, a task due today with no time still reads today", got)
	}
}

func TestFormat(t *testing.T) {
	cases := []struct {
		due  string
		want string
	}{
		{"2026-08-27", "2d overdue"},
		{"2026-08-28", "1d overdue"},
		{"2026-08-29", "today"},
		{"2026-08-30", "tomorrow"},
		{"2026-08-31", "08-31"},
		{"2026-09-04", "09-04"},
		{"2026-09-05", "09-05"},
		{"2027-01-02", "2027-01-02"},
	}
	for _, c := range cases {
		due, err := time.ParseInLocation("2006-01-02", c.due, time.Local)
		if err != nil {
			t.Fatal(err)
		}
		if got := Format(due, false, ref()); got != c.want {
			t.Errorf("Format(%s) = %q, want %q", c.due, got, c.want)
		}
	}
}
