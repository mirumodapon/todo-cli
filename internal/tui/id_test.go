package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"todo.mirumo.net/internal/task"
)

// The id is what every other way into a task is addressed by — task done 3,
// task details 3, the MCP tools — so the interface has to show it.
func TestTheListShowsIDs(t *testing.T) {
	m, _ := newModel(t)
	v := m.View()
	for i, ti := range m.tasks {
		row := lineWith(v, ti.Title)
		if row == "" {
			t.Fatalf("no row for %q:\n%s", ti.Title, v)
		}
		if !strings.Contains(row, itoa(ti.ID)) {
			t.Errorf("row %d should carry id %d: %q", i, ti.ID, row)
		}
	}
}

// Ids are a column, so they line up: a ragged left edge is harder to read than
// the numbers are worth.
func TestIDsAreAligned(t *testing.T) {
	m, s := newModel(t)
	// Push the ids into two digits.
	for i := range 8 {
		if _, err := s.Add(task.Task{Title: "bulk " + itoa(int64(i)), CreatedAt: refTime(), UpdatedAt: refTime()}); err != nil {
			t.Fatal(err)
		}
	}
	m = press(t, m, "r")

	var titleAt []int
	for _, ti := range m.tasks {
		row := lineWith(m.View(), ti.Title)
		// Columns, not bytes: the cursor arrow is one cell and three bytes.
		titleAt = append(titleAt, lipgloss.Width(row[:strings.Index(row, "[")]))
	}
	for i, at := range titleAt {
		if at != titleAt[0] {
			t.Fatalf("row %d starts its status at %d, the first at %d:\n%s", i, at, titleAt[0], m.View())
		}
	}
}
