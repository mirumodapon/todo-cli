package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"todo.mirumo.net/internal/task"
)

// 所有與資料庫往來的動作都包成 tea.Cmd，結果以 msg 回到 Update。
// Update 本身不碰 IO，維持純函式，測試只需要餵 msg。
type (
	tasksMsg []task.Task
	errMsg   struct{ err error }
	savedMsg struct{ note string }
)

func (m Model) loadCmd() tea.Cmd {
	s, f, now := m.store, m.filter, m.now()
	return func() tea.Msg {
		ts, err := s.List(f, now)
		if err != nil {
			return errMsg{err}
		}
		return tasksMsg(ts)
	}
}

func (m Model) toggleCmd(t task.Task) tea.Cmd {
	s, now := m.store, m.now()
	return func() tea.Msg {
		if err := s.SetDone(t.ID, !t.Done(), now); err != nil {
			return errMsg{err}
		}
		note := "已完成「" + t.Title + "」"
		if t.Done() {
			note = "已取消完成「" + t.Title + "」"
		}
		return savedMsg{note: note}
	}
}
