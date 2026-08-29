package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"todo.mirumo.net/internal/datearg"
	"todo.mirumo.net/internal/project"
	"todo.mirumo.net/internal/task"
	"todo.mirumo.net/internal/urgency"
)

const (
	ansiReset = "\x1b[0m"
	ansiDim   = "\x1b[2m"
)

// pad aligns by terminal display width. CJK characters take two cells, which len() and text/tabwriter both get wrong.
func pad(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

type row struct{ id, status, pri, due, title, proj, tags string }

func toRow(t task.Task, now time.Time) row {
	r := row{
		id:     fmt.Sprintf("%d", t.ID),
		status: "[ ]",
		title:  t.Title,
		proj:   project.Label(t.Project),
	}
	if t.Done() {
		r.status = "[x]"
	}
	if p := t.Priority.String(); p != "" {
		r.pri = "!" + p
	}
	if t.Due != nil {
		r.due = datearg.Format(*t.Due, t.DueHasTime, now)
	}
	if len(t.Tags) > 0 {
		r.tags = "@" + strings.Join(t.Tags, " @")
	}
	return r
}

// colorize paints a row by how soon it is due: green three days out, ramping
// to red at twelve hours and staying there. Anything further out is left
// plain, so colour marks what is actually close rather than decorating
// everything. Done tasks are dimmed and take no urgency colour.
func colorize(line string, t task.Task, now time.Time) string {
	if t.Done() {
		return ansiDim + line + ansiReset
	}
	if t.Due == nil {
		return line
	}
	level, ok := urgency.Level(*t.Due, t.DueHasTime, now)
	if !ok {
		return line
	}
	return urgency.ANSI(level) + line + ansiReset
}

// WriteList prints an aligned task list. With color false it emits no ANSI codes at all.
func WriteList(w io.Writer, ts []task.Task, now time.Time, color bool) {
	if len(ts) == 0 {
		fmt.Fprintln(w, "No matching tasks")
		return
	}
	rows := make([]row, len(ts))
	var wID, wPri, wDue, wTitle, wProj int
	for i, t := range ts {
		rows[i] = toRow(t, now)
		wID = max(wID, lipgloss.Width(rows[i].id))
		wPri = max(wPri, lipgloss.Width(rows[i].pri))
		wDue = max(wDue, lipgloss.Width(rows[i].due))
		wTitle = max(wTitle, lipgloss.Width(rows[i].title))
		wProj = max(wProj, lipgloss.Width(rows[i].proj))
	}
	for i, r := range rows {
		line := strings.TrimRight(strings.Join([]string{
			pad(r.id, wID), r.status, pad(r.pri, wPri), pad(r.due, wDue),
			pad(r.title, wTitle), pad(r.proj, wProj), r.tags,
		}, " "), " ")
		if color {
			line = colorize(line, ts[i], now)
		}
		fmt.Fprintln(w, line)
	}
}
