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
	return styleHint.Render("a 新增 · e 編輯 · space 完成 · d 刪除 · / 搜尋 · P/T 過濾 · ? 說明 · q 離開")
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func viewHelp() string {
	rows := [][2]string{
		{"j / k / ↑ / ↓", "移動"},
		{"g / G", "跳到頂 / 底"},
		{"space", "切換完成"},
		{"a / e", "新增 / 編輯"},
		{"d", "刪除"},
		{"u", "復原最近一次刪除"},
		{"/", "搜尋標題"},
		{"P / T", "依專案 / 標籤過濾"},
		{"A", "切換是否顯示已完成"},
		{"s", "循環排序"},
		{"esc", "清除所有過濾"},
		{"?", "這份說明"},
		{"q", "離開"},
	}
	var b strings.Builder
	b.WriteString("按鍵說明\n\n")
	for _, r := range rows {
		b.WriteString("  " + pad(r[0], 16) + r[1] + "\n")
	}
	b.WriteString("\n" + styleHint.Render("按任意鍵返回"))
	return b.String()
}
