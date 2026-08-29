// Package urgency turns "how long until this is due" into a colour, so a list
// can be read by shape before it is read word by word.
package urgency

import (
	"time"

	"todo.mirumo.net/internal/theme"
)

const (
	// ColourFrom is how far out colouring starts. Anything further away is
	// left plain: everything cannot be urgent.
	ColourFrom = 72 * time.Hour
	// FullRed is the point where the ramp has run out and stays red.
	FullRed = 12 * time.Hour
)

// deadline is the moment a due date actually falls due. A date with no time of
// day is due by the end of that day; treating it as midnight would make
// everything due today read as already overdue.
func deadline(due time.Time, hasTime bool) time.Time {
	if hasTime {
		return due
	}
	return time.Date(due.Year(), due.Month(), due.Day(), 23, 59, 59, 0, due.Location())
}

// Level reports how urgent a due date is: 0 at ColourFrom, rising to 1 at
// FullRed and staying there once past it. ok is false when the deadline is far
// enough away that it should not be coloured at all.
func Level(due time.Time, hasTime bool, now time.Time) (float64, bool) {
	left := deadline(due, hasTime).Sub(now)
	if left > ColourFrom {
		return 0, false
	}
	if left <= FullRed {
		return 1, true
	}
	// Between the two thresholds, linearly.
	return float64(ColourFrom-left) / float64(ColourFrom-FullRed), true
}

// stops are the colours the ramp passes through, all of them Catppuccin
// Macchiato entries. Blending straight from green to red would run through
// shades the palette does not contain; going by way of yellow and peach keeps
// every part of the ramp recognisably from the same set.
var stops = []theme.RGB{theme.Green, theme.Yellow, theme.Peach, theme.Red}

// Color returns the colour for a level, green at 0 and red at 1.
func Color(level float64) theme.RGB { return theme.Ramp(stops, level) }

// ANSI is the truecolor escape sequence for a level.
func ANSI(level float64) string { return Color(level).ANSI() }

// Hex is the same colour as "#rrggbb", for libraries that want one.
func Hex(level float64) string { return Color(level).Hex() }
