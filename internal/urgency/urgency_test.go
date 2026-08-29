package urgency

import (
	"math"
	"testing"
	"time"

	"todo.mirumo.net/internal/theme"
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

func TestColorRunsThePaletteRamp(t *testing.T) {
	if got := Color(0); got != theme.Green {
		t.Errorf("level 0 = %v, want the Macchiato green", got)
	}
	if got := Color(1); got != theme.Red {
		t.Errorf("level 1 = %v, want the Macchiato red", got)
	}
	// The stops are green, yellow, peach, red, so a third of the way along
	// lands on yellow.
	if got := Color(1.0 / 3); got != theme.Yellow {
		t.Errorf("level 1/3 = %v, want the Macchiato yellow", got)
	}
	if got := Color(2.0 / 3); got != theme.Peach {
		t.Errorf("level 2/3 = %v, want the Macchiato peach", got)
	}
}

// Redness should never fall as a deadline approaches.
func TestColorGetsSteadilyRedder(t *testing.T) {
	prev := math.MinInt
	for i := 0; i <= 20; i++ {
		c := Color(float64(i) / 20)
		// Green sits below zero on this measure, so the starting point cannot
		// be a hand-picked sentinel.
		redness := int(c.R) - int(c.G)
		if redness < prev {
			t.Fatalf("level %.2f went back towards green: %v", float64(i)/20, c)
		}
		prev = redness
	}
}

func TestANSIIsTruecolor(t *testing.T) {
	if got := ANSI(1); got != "\x1b[38;2;237;135;150m" {
		t.Errorf("= %q, want a truecolor escape for the Macchiato red", got)
	}
}
