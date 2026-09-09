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
	// styleDescBody indents through padding rather than a literal prefix, so a
	// wrapped line keeps the indent on every row it takes.
	styleDescBody = lipgloss.NewStyle().PaddingLeft(2)
)

func pad(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

// idWidth is the width of the id column: the widest id in the whole list, not
// just the visible part, so the column does not shift as you scroll.
func (m Model) idWidth() int {
	var w int
	for _, t := range m.tasks {
		w = max(w, len(itoa(t.ID)))
	}
	return w
}

// taskLine builds one row's text, without the cursor marker. The id leads it:
// it is what every other way into a task is addressed by, from task done 3 to
// the MCP tools.
func (m Model) taskLine(t task.Task) string {
	status := "[ ]"
	if t.Done() {
		status = "[x]"
	}
	parts := []string{pad(itoa(t.ID), m.idWidth()), status}
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

// listRows renders the visible slice of the list, cursor markers included.
func (m Model) listRows() []string {
	if len(m.tasks) == 0 {
		return []string{styleDim.Render("No matching tasks")}
	}
	end := min(len(m.tasks), m.offset+m.listHeight())
	rows := make([]string, 0, end-m.offset)
	for i := m.offset; i < end; i++ {
		t := m.tasks[i]
		rows = append(rows, m.marker(i)+m.rowStyle(t).Render(m.taskLine(t)))
	}
	return rows
}

func (m Model) viewList() string {
	rows := m.listRows()
	body := m.header() + "\n\n"
	switch m.paneLayout() {
	case layoutSide:
		body += m.beside(rows)
	case layoutStacked:
		body += m.under(rows)
	default:
		for _, r := range rows {
			body += clip(r, m.width) + "\n"
		}
	}
	return m.screen(body, m.footer())
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
	arrow := "↑"
	if m.filter.Reverse {
		arrow = "↓"
	}
	h := fmt.Sprintf("%d %s · %s · %s %s", len(m.tasks), unit, m.scope(), sortLabel(m.filter.Sort), arrow)
	if m.filter.Untagged {
		h += "  untagged"
	} else if len(m.filter.Tags) > 0 {
		h += "  @" + strings.Join(m.filter.Tags, " @")
	}
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
	{"enter", "Show the full task"},
	{"space", "Toggle done (asks first)"},
	{"a / e", "Add / edit"},
	{"E", "Edit the whole task in $EDITOR"},
	{"d", "Delete (asks first)"},
	{"u", "Undo the last delete"},
	{"/", "Search titles"},
	{"P / T", "Filter by project / tag"},
	{"A", "Show or hide done tasks"},
	{"s", "Choose the order"},
	{"R", "Reverse the order"},
	{"D", "Switch between time remaining and dates"},
	{"v", "Show or hide the detail pane"},
	{"r", "Reread the database now"},
	{"ctrl+z", "Suspend to the shell"},
	{"esc", "Back to the filter it opened on"},
	{"?", "This help"},
	{"q / ctrl+c / ctrl+d", "Quit (asks first)"},
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
	return m.screen(b.String(), m.hint(styleHint.Render("Press any key to go back")))
}
