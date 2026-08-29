package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"todo.mirumo.net/internal/datearg"
	"todo.mirumo.net/internal/project"
	"todo.mirumo.net/internal/task"
)

var (
	styleCursor = lipgloss.NewStyle().Bold(true)
	styleDim    = lipgloss.NewStyle().Faint(true)
	styleErr    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
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
	if p := t.Priority.Label(); p != "" {
		parts = append(parts, "!"+p)
	}
	if t.Due != nil {
		parts = append(parts, datearg.Format(*t.Due, m.now()))
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

func (m Model) viewList() string {
	var b strings.Builder
	b.WriteString(m.header() + "\n\n")
	if len(m.tasks) == 0 {
		b.WriteString(styleDim.Render("沒有符合的待辦") + "\n")
	}
	for i, t := range m.tasks {
		marker := "  "
		if i == m.cursor {
			marker = "▸ "
		}
		line := marker + m.taskLine(t)
		switch {
		case i == m.cursor:
			line = styleCursor.Render(line)
		case t.Done():
			line = styleDim.Render(line)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + m.footer())
	return b.String()
}

func (m Model) header() string {
	h := fmt.Sprintf("todo — %d 筆", len(m.tasks))
	if m.filter.Search != "" {
		h += "  搜尋：" + m.filter.Search
	}
	if m.filter.IncludeDone {
		h += "  含已完成"
	}
	return h
}

func (m Model) footer() string {
	if m.mode == modeSearch {
		return m.search.View()
	}
	if m.err != nil {
		return styleErr.Render("錯誤：" + m.err.Error())
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
		{"space", "Toggle done"},
		{"a / e", "Add / edit"},
		{"d", "Delete"},
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
