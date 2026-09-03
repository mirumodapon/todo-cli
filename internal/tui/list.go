package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"todo.mirumo.net/internal/datearg"
	"todo.mirumo.net/internal/project"
	"todo.mirumo.net/internal/task"
	"todo.mirumo.net/internal/theme"
	"todo.mirumo.net/internal/urgency"
)

var (
	styleCursor = lipgloss.NewStyle().Bold(true)
	styleDim    = lipgloss.NewStyle().Faint(true)
	styleErr    = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Red.Hex()))
	styleHint   = lipgloss.NewStyle().Faint(true)
)

func pad(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

// taskLine builds one row's text, without the cursor marker.
func (m Model) taskLine(t task.Task) string {
	status := "[ ]"
	if t.Done() {
		status = "[x]"
	}
	parts := []string{status}
	if p := t.Priority.Marks(); p != "" {
		parts = append(parts, p)
	}
	if t.Due != nil {
		if m.dates {
			parts = append(parts, datearg.Format(*t.Due, t.DueHasTime, m.now()))
		} else {
			parts = append(parts, datearg.Remaining(*t.Due, t.DueHasTime, m.now()))
		}
	}
	parts = append(parts, t.Title)
	if p := project.Label(t.Project); p != "" {
		parts = append(parts, p)
	}
	if len(t.Tags) > 0 {
		parts = append(parts, "@"+strings.Join(t.Tags, " @"))
	}
	return strings.Join(parts, " ")
}

// cursorMarker is how the selected row is shown. Selection deliberately does
// not restyle the row: colour there means how soon the task is due, and
// overriding it would hide that for whichever row you happened to be on.
const cursorMarker = "▶"

// markerWidth keeps selected and unselected rows aligned. The arrow is an
// East Asian ambiguous-width character, so it occupies one cell in some
// terminals and two in others; padding to a fixed width covers both.
const markerWidth = 2

func (m Model) marker(i int) string {
	if i == m.cursor {
		return pad(cursorMarker, markerWidth)
	}
	return pad("", markerWidth)
}

// rowStyle paints a row by how soon it is due, on the same ramp the CLI uses.
// It takes only the task: what a row looks like must not depend on where the
// cursor happens to be.
func (m Model) rowStyle(t task.Task) lipgloss.Style {
	if t.Done() {
		return styleDim
	}
	if t.Due == nil {
		return lipgloss.NewStyle()
	}
	level, ok := urgency.Level(*t.Due, t.DueHasTime, m.now())
	if !ok {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(urgency.Hex(level)))
}

// screen frames a page: the content at the top, the hint on the very last row,
// and blank rows between them. The hint is a fixed landmark, so it must not
// float up under a short list or drift down out of sight under a long one.
func (m Model) screen(body, footer string) string {
	// A hint wider than the terminal would wrap onto a second line and push
	// itself off the bottom, so it is clipped rather than wrapped.
	footer = lipgloss.NewStyle().MaxWidth(m.width).Render(footer)
	rows := strings.Split(strings.TrimRight(body, "\n"), "\n")
	if n := m.height - 1 - len(rows); n > 0 {
		rows = append(rows, make([]string, n)...)
	}
	// Bodies that manage their own scrolling never reach this, but one that
	// overflows is clipped so the frame still ends with the hint.
	if len(rows) > m.height-1 {
		rows = rows[:max(0, m.height-1)]
	}
	return strings.Join(rows, "\n") + "\n" + footer
}

func (m Model) viewList() string {
	var b strings.Builder
	b.WriteString(m.header() + "\n\n")
	if len(m.tasks) == 0 {
		b.WriteString(styleDim.Render("No matching tasks") + "\n")
	}
	end := min(len(m.tasks), m.offset+m.listHeight())
	for i := m.offset; i < end; i++ {
		t := m.tasks[i]
		b.WriteString(m.marker(i) + m.rowStyle(t).Render(m.taskLine(t)) + "\n")
	}
	return m.screen(b.String(), m.footer())
}

// scope names what the list is currently showing, so the project filter is
// never invisible state.
func (m Model) scope() string {
	switch {
	case m.filter.Project == nil:
		return "all projects"
	case *m.filter.Project == "":
		return "uncategorized"
	default:
		return project.Label(*m.filter.Project)
	}
}

func (m Model) header() string {
	unit := "tasks"
	if len(m.tasks) == 1 {
		unit = "task"
	}
	h := fmt.Sprintf("todo — %d %s · %s", len(m.tasks), unit, m.scope())
	if m.filter.Search != "" {
		h += "  search: " + m.filter.Search
	}
	if m.filter.IncludeDone {
		h += "  including done"
	}
	return h
}

func (m Model) footer() string {
	if m.mode == modeConfirm {
		return m.confirm.prompt
	}
	if m.mode == modeSearch {
		return m.search.View()
	}
	if m.err != nil {
		return styleErr.Render("error: " + m.err.Error())
	}
	if m.status != "" {
		return m.status
	}
	// Kept inside 80 columns: a hint that wraps breaks the frame. The rest of
	// the bindings live one ? away.
	return styleHint.Render("a add · e edit · space done · d delete · / search · P/T filter · ? help · q quit")
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// helpRows is the one list of keys. The README quotes it, and a test checks
// the two agree, so the documentation cannot drift from the bindings.
var helpRows = [][2]string{
	{"j / k / ↑ / ↓", "Move"},
	{"ctrl+n / ctrl+p", "Move, including while typing"},
	{"g / G", "Jump to top / bottom"},
	{"space", "Toggle done (asks first)"},
	{"a / e", "Add / edit"},
	{"d", "Delete (asks first)"},
	{"u", "Undo the last delete"},
	{"/", "Search titles"},
	{"P / T", "Filter by project / tag"},
	{"A", "Show or hide done tasks"},
	{"s", "Cycle sort order"},
	{"D", "Switch between time remaining and dates"},
	{"esc", "Back to the uncategorized default"},
	{"?", "This help"},
	{"q", "Quit"},
}

func (m Model) viewHelp() string {
	var b strings.Builder
	// As in the form, the key column is derived rather than hard-coded, so a
	// longer binding cannot run into its description.
	var w int
	for _, r := range helpRows {
		w = max(w, lipgloss.Width(r[0]))
	}
	b.WriteString("Keys\n\n")
	for _, r := range helpRows {
		b.WriteString("  " + pad(r[0], w+2) + r[1] + "\n")
	}
	return m.screen(b.String(), styleHint.Render("Press any key to go back"))
}
