package urgency

import (
	"testing"
	"time"
)

func now() time.Time { return time.Date(2026, 8, 29, 15, 0, 0, 0, time.Local) }

func at(y int, mo time.Month, d, h, mi int) time.Time {
	return time.Date(y, mo, d, h, mi, 0, 0, time.Local)
}

func TestLevelRange(t *testing.T) {
	cases := []struct {
		name    string
		due     time.Time
		hasTime bool
		wantOK  bool
		want    float64
	}{
		{"a week out is not coloured", at(2026, 9, 5, 15, 0), true, false, 0},
		{"exactly three days out is the green end", at(2026, 9, 1, 15, 0), true, true, 0},
		{"halfway between is halfway along", at(2026, 8, 31, 9, 0), true, true, 0.5}, // 42h left
		{"twelve hours out is fully red", at(2026, 8, 30, 3, 0), true, true, 1},
		{"less than twelve hours stays fully red", at(2026, 8, 29, 23, 0), true, true, 1},
		{"overdue stays fully red", at(2026, 8, 20, 9, 0), true, true, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := Level(c.due, c.hasTime, now())
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v", ok, c.wantOK)
			}
			if ok && (got < c.want-0.01 || got > c.want+0.01) {
				t.Errorf("level = %v, want %v", got, c.want)
			}
		})
	}
}

// A date with no time is due by the end of that day, not at its midnight.
// Treating it as midnight would make everything due today look overdue.
func TestDateOnlyDueMeansEndOfDay(t *testing.T) {
	got, ok := Level(at(2026, 8, 29, 0, 0), false, now())
	if !ok {
		t.Fatal("something due today should be coloured")
	}
	if got != 1 {
		t.Errorf("level = %v, want 1: under nine hours remain", got)
	}

	// End of 2026-09-01 is 3d 9h away, past the three-day window.
	if _, ok := Level(at(2026, 9, 1, 0, 0), false, now()); ok {
		t.Error("a date-only due more than three days out should not be coloured")
	}
}

func TestColorRampsGreenToRed(t *testing.T) {
	r, g, b := Color(0)
	if r != 0 || g == 0 || b != 0 {
		t.Errorf("level 0 = (%d,%d,%d), want green", r, g, b)
	}
	r, g, b = Color(1)
	if r != 255 || g != 0 || b != 0 {
		t.Errorf("level 1 = (%d,%d,%d), want pure red", r, g, b)
	}
	r, g, _ = Color(0.5)
	if r == 0 || g == 0 {
		t.Errorf("halfway = (%d,%d,_), want a blend of both", r, g)
	}
}

func TestANSIIsTruecolor(t *testing.T) {
	if got := ANSI(1); got != "\x1b[38;2;255;0;0m" {
		t.Errorf("= %q, want a truecolor escape", got)
	}
}
