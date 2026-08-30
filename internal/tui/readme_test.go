package tui

import (
	"os"
	"strings"
	"testing"
)

// The README documents the key bindings in a table. Two hand-maintained lists
// of the same thing drift, so this reads the table back and compares it with
// the one the interface actually shows.
func TestREADMEKeyTableMatchesTheHelp(t *testing.T) {
	raw, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatalf("cannot read the README: %v", err)
	}
	lines := strings.Split(string(raw), "\n")

	start := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "| Key | Action |") {
			start = i + 2 // skip the separator row
			break
		}
	}
	if start < 0 {
		t.Fatal("the README has no key table; it should document the bindings")
	}

	var got [][2]string
	for _, l := range lines[start:] {
		if !strings.HasPrefix(l, "|") {
			break
		}
		cells := strings.Split(strings.Trim(l, "|"), "|")
		if len(cells) != 2 {
			t.Fatalf("malformed table row: %q", l)
		}
		key := strings.TrimSpace(strings.ReplaceAll(cells[0], "`", ""))
		got = append(got, [2]string{key, strings.TrimSpace(cells[1])})
	}

	if len(got) != len(helpRows) {
		t.Fatalf("the README lists %d keys, the help lists %d", len(got), len(helpRows))
	}
	for i := range helpRows {
		if got[i] != helpRows[i] {
			t.Errorf("row %d: README has %v, the help has %v", i, got[i], helpRows[i])
		}
	}
}
