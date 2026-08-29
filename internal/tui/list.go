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
	if p := t.Priority.String(); p != "" {
		parts = append(parts, "!"+p)
	}
	if t.Due != nil {
		parts = append(parts, datearg.Format(*t.Due, t.DueHasTime, m.now()))
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

func (m Model) viewList() string {
	var b strings.Builder
	b.WriteString(m.header() + "\n\n")
	if len(m.tasks) == 0 {
		b.WriteString(styleDim.Render("No matching tasks") + "\n")
	}
	for i, t := range m.tasks {
		b.WriteString(m.marker(i) + m.rowStyle(t).Render(m.taskLine(t)) + "\n")
	}
	b.WriteString("\n" + m.footer())
	return b.String()
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
	return styleHint.Render("a add · e edit · space toggle · d delete · / search · P/T filter · ? help · q quit")
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func viewHelp() string {
	rows := [][2]string{
		{"j / k / ↑ / ↓", "Move"},
		{"g / G", "Jump to top / bottom"},
		{"space", "Toggle done (asks first)"},
		{"a / e", "Add / edit"},
		{"d", "Delete (asks first)"},
		{"u", "Undo the last delete"},
		{"/", "Search titles"},
		{"P / T", "Filter by project / tag"},
		{"A", "Show or hide done tasks"},
		{"s", "Cycle sort order"},
		{"esc", "Clear all filters"},
		{"?", "This help"},
		{"q", "Quit"},
	}
	var b strings.Builder
	b.WriteString("Keys\n\n")
	for _, r := range rows {
		b.WriteString("  " + pad(r[0], 16) + r[1] + "\n")
	}
	b.WriteString("\n" + styleHint.Render("Press any key to go back"))
	return b.String()
}
