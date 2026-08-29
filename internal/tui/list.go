package tui

import (
	"fmt"
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

// taskLine 組出一行的內容（不含游標標記）。
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
	return styleHint.Render("j/k 移動 · space 完成 · d 刪除 · / 搜尋 · s 排序 · A 含已完成 · esc 清除 · q 離開")
}
