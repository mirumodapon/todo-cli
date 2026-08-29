// Package urgency turns "how long until this is due" into a colour, so a list
// can be read by shape before it is read word by word.
package urgency

import (
	"fmt"
	"time"
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

// Colour returns the RGB for a level, ramping green to red. Interpolating
// straight through RGB passes via yellow, which is the ramp people expect.
func Color(level float64) (r, g, b uint8) {
	if level < 0 {
		level = 0
	}
	if level > 1 {
		level = 1
	}
	const greenAt0 = 200.0
	return uint8(level * 255), uint8(greenAt0 * (1 - level)), 0
}

// ANSI is the truecolor escape sequence for a level.
func ANSI(level float64) string {
	r, g, b := Color(level)
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
}

// Hex is the same colour as "#rrggbb", for libraries that want one.
func Hex(level float64) string {
	r, g, b := Color(level)
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}
