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
)

const (
	ansiReset  = "\x1b[0m"
	ansiDim    = "\x1b[2m"
	ansiRed    = "\x1b[31m"
	ansiYellow = "\x1b[33m"
)

// pad 依終端顯示寬度補空白。中文字佔兩格，len() 與 text/tabwriter 都會算錯。
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
	if p := t.Priority.Label(); p != "" {
		r.pri = "!" + p
	}
	if t.Due != nil {
		r.due = datearg.Format(*t.Due, now)
	}
	if len(t.Tags) > 0 {
		r.tags = "@" + strings.Join(t.Tags, " @")
	}
	return r
}

func colorize(line string, t task.Task, now time.Time) string {
	switch {
	case t.Done():
		return ansiDim + line + ansiReset
	case t.Due != nil && datearg.Day(*t.Due).Before(datearg.Day(now)):
		return ansiRed + line + ansiReset
	case t.Priority == task.PriHigh:
		return ansiYellow + line + ansiReset
	}
	return line
}

// WriteList 輸出對齊的待辦清單。color 為 false 時完全不輸出 ANSI 碼。
func WriteList(w io.Writer, ts []task.Task, now time.Time, color bool) {
	if len(ts) == 0 {
		fmt.Fprintln(w, "沒有符合的待辦")
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
