package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"todo.mirumo.net/internal/task"
)

// viewLines splits a rendered screen for layout assertions.
func viewLines(m Model) []string { return strings.Split(m.View(), "\n") }

// The hint belongs on the last row of the terminal. Letting it sit right under
// the content makes it jump up and down as the list grows and shrinks.
func TestHintSitsOnTheLastLine(t *testing.T) {
	base, _ := newModel(t) // three tasks, far short of the 24 rows
	cases := []struct {
		name string
		keys []string
		want string
	}{
		{"list", nil, "q quit"},
		{"search", []string{"/"}, "search titles"},
		{"help", []string{"?"}, "go back"},
		{"form", []string{"a"}, "esc cancel"},
		{"picker", []string{"P"}, "esc cancel"},
		{"confirm", []string{" "}, "done? (y/n)"},
		{"detail", []string{"enter"}, "go back"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := base
			for _, k := range c.keys {
				m = press(t, m, k)
			}
			got := viewLines(m)
			if len(got) != 24 {
				t.Fatalf("the screen should fill 24 rows, got %d:\n%s", len(got), m.View())
			}
			if !strings.Contains(got[23], c.want) {
				t.Errorf("the last row should hold the hint %q, got %q", c.want, got[23])
			}
		})
	}
}

// bulk fills the store with enough uncategorized tasks to overflow the screen.
func bulk(t *testing.T, m Model, n int) Model {
	t.Helper()
	for i := 0; i < n; i++ {
		ti := task.Task{Title: "bulk " + itoa(int64(i)), CreatedAt: refTime(), UpdatedAt: refTime()}
		if _, err := m.store.Add(ti); err != nil {
			t.Fatal(err)
		}
	}
	m, msg := run(t, m, m.loadCmd())
	m, _ = send(t, m, msg)
	return m
}

func TestListScrollsToKeepTheCursorVisible(t *testing.T) {
	m, _ := newModel(t)
	m, _ = send(t, m, tea.WindowSizeMsg{Width: 80, Height: 12})
	m = bulk(t, m, 30)

	m = press(t, m, "G")
	v := m.View()
	if n := len(strings.Split(v, "\n")); n != 12 {
		t.Fatalf("the screen should stay 12 rows, got %d:\n%s", n, v)
	}
	if !strings.Contains(v, "bulk 29") {
		t.Errorf("the cursor's row must be on screen:\n%s", v)
	}
	if strings.Contains(v, "first") {
		t.Errorf("the top of the list should have scrolled away:\n%s", v)
	}
	if !strings.Contains(v, "todo — ") {
		t.Errorf("the header must survive scrolling:\n%s", v)
	}

	m = press(t, m, "g")
	v = m.View()
	if !strings.Contains(v, "first") || strings.Contains(v, "bulk 29") {
		t.Errorf("g should scroll back to the top:\n%s", v)
	}
}

// The list scrolls a row at a time rather than jumping a page, so the rows
// around the cursor stay where the eye left them.
func TestListScrollsOneRowAtATime(t *testing.T) {
	m, _ := newModel(t)
	m, _ = send(t, m, tea.WindowSizeMsg{Width: 80, Height: 12})
	m = bulk(t, m, 30)

	for i := 0; i < m.listHeight(); i++ {
		m = press(t, m, "j")
	}
	if m.offset != 1 {
		t.Fatalf("stepping one past the bottom should scroll one row, offset = %d", m.offset)
	}
	m = press(t, m, "k")
	if m.offset != 1 {
		t.Errorf("moving back inside the window must not scroll, offset = %d", m.offset)
	}
}

// A hint wider than the terminal wraps onto a second line, which would push it
// off the bottom of the frame.
func TestListHintFitsAnEightyColumnTerminal(t *testing.T) {
	m, _ := newModel(t)
	if w := lipgloss.Width(m.footer()); w > 80 {
		t.Errorf("the hint is %d columns wide, it must fit in 80: %s", w, m.footer())
	}
}
