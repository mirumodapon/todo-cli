package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"todo.mirumo.net/internal/store"
	"todo.mirumo.net/internal/task"
)

// 所有與資料庫往來的動作都包成 tea.Cmd，結果以 msg 回到 Update。
// Update 本身不碰 IO，維持純函式，測試只需要餵 msg。
type (
	tasksMsg   []task.Task
	errMsg     struct{ err error }
	savedMsg   struct{ note string }
	deletedMsg struct{ t task.Task }
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

// deleteCmd 先完整取回再刪除——undo 需要包含標籤的整份資料。
func (m Model) deleteCmd(t task.Task) tea.Cmd {
	s := m.store
	return func() tea.Msg {
		full, err := s.Get(t.ID)
		if err != nil {
			return errMsg{err}
		}
		if err := s.Delete(t.ID); err != nil {
			return errMsg{err}
		}
		return deletedMsg{t: full}
	}
}

func (m Model) restoreCmd(t task.Task) tea.Cmd {
	s := m.store
	return func() tea.Msg {
		if err := s.Restore(t); err != nil {
			return errMsg{err}
		}
		return savedMsg{note: "已復原「" + t.Title + "」"}
	}
}

type (
	projectsMsg []store.ProjectCount
	tagsMsg     []string
)

func (m Model) projectsCmd() tea.Cmd {
	s := m.store
	return func() tea.Msg {
		ps, err := s.Projects()
		if err != nil {
			return errMsg{err}
		}
		return projectsMsg(ps)
	}
}

func (m Model) tagsCmd() tea.Cmd {
	s := m.store
	return func() tea.Msg {
		ts, err := s.Tags()
		if err != nil {
			return errMsg{err}
		}
		return tagsMsg(ts)
	}
}

func (m Model) saveCmd(t task.Task, editing bool) tea.Cmd {
	s := m.store
	return func() tea.Msg {
		if editing {
			if err := s.Update(t); err != nil {
				return errMsg{err}
			}
			return savedMsg{note: "已更新「" + t.Title + "」"}
		}
		if _, err := s.Add(t); err != nil {
			return errMsg{err}
		}
		return savedMsg{note: "已新增「" + t.Title + "」"}
	}
}
