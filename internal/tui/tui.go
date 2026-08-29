// Package tui 提供 todo 的互動介面。
package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"todo.mirumo.net/internal/store"
	"todo.mirumo.net/internal/task"
)

type mode int

const (
	modeList mode = iota
	modeSearch
)

// Model 是根 model。所有子狀態掛在這裡，Update 依 mode 分派。
type Model struct {
	store store.Store
	now   func() time.Time
	cwd   string

	mode   mode
	tasks  []task.Task
	cursor int
	filter task.Filter

	search textinput.Model
	// undo 只保留一層：最近一次刪除的完整項目，離開 TUI 即失效。
	undo *task.Task

	status        string
	err           error
	width, height int
}

// New 建立一個 Model。
func New(s store.Store, now func() time.Time, cwd string) Model {
	ti := textinput.New()
	ti.Prompt = "/"
	ti.Placeholder = "搜尋標題"
	return Model{
		store: s, now: now, cwd: cwd,
		mode: modeList, search: ti,
		width: 80, height: 24,
	}
}

// Run 啟動互動介面。
func Run(s store.Store, now func() time.Time, cwd string) error {
	_, err := tea.NewProgram(New(s, now, cwd), tea.WithAltScreen()).Run()
	return err
}

func (m Model) Init() tea.Cmd { return m.loadCmd() }

// current 回傳游標所指的項目。
func (m Model) current() (task.Task, bool) {
	if m.cursor < 0 || m.cursor >= len(m.tasks) {
		return task.Task{}, false
	}
	return m.tasks[m.cursor], true
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tasksMsg:
		m.tasks = []task.Task(msg)
		m.err = nil
		if m.cursor >= len(m.tasks) {
			m.cursor = max(0, len(m.tasks)-1)
		}
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil

	case savedMsg:
		m.status, m.err = msg.note, nil
		return m, m.loadCmd()

	case deletedMsg:
		t := msg.t
		m.undo = &t
		m.status = "已刪除「" + t.Title + "」· u 復原"
		m.err = nil
		return m, m.loadCmd()

	case tea.KeyMsg:
		switch m.mode {
		case modeList:
			return m.updateList(msg)
		case modeSearch:
			return m.updateSearch(msg)
		}
	}
	return m, nil
}

func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		if m.cursor < len(m.tasks)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "g":
		m.cursor = 0
	case "G":
		m.cursor = max(0, len(m.tasks)-1)
	case " ":
		if t, ok := m.current(); ok {
			return m, m.toggleCmd(t)
		}
	case "d":
		if t, ok := m.current(); ok {
			return m, m.deleteCmd(t)
		}
	case "u":
		if m.undo == nil {
			m.status = "沒有可復原的刪除"
			return m, nil
		}
		t := *m.undo
		m.undo = nil
		return m, m.restoreCmd(t)
	case "/":
		m.mode = modeSearch
		m.search.SetValue(m.filter.Search)
		m.search.CursorEnd()
		m.search.Focus()
		return m, nil
	case "A":
		m.filter.IncludeDone = !m.filter.IncludeDone
		return m, m.loadCmd()
	case "s":
		m.filter.Sort = (m.filter.Sort + 1) % 3
		m.status = "排序：" + sortLabel(m.filter.Sort)
		return m, m.loadCmd()
	case "esc":
		m.filter = task.Filter{}
		m.search.SetValue("")
		m.status = ""
		return m, m.loadCmd()
	}
	return m, nil
}

// updateSearch 處理增量搜尋。每個按鍵都重查一次——清單規模小，成本可忽略，
// 換來的是畫面與資料庫永遠一致。
func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.mode = modeList
		m.search.Blur()
		return m, nil
	case "esc":
		m.mode = modeList
		m.search.Blur()
		m.search.SetValue("")
		m.filter.Search = ""
		return m, m.loadCmd()
	}
	// 刻意丟掉 textinput 回傳的 cmd：那是游標閃爍的計時器。
	// 轉傳它會讓 Update 的測試變成在等計時器，而閃爍只是裝飾。
	m.search, _ = m.search.Update(msg)
	m.filter.Search = m.search.Value()
	m.cursor = 0
	return m, m.loadCmd()
}

func sortLabel(s task.SortBy) string {
	switch s {
	case task.SortPriority:
		return "優先度"
	case task.SortCreated:
		return "建立時間"
	}
	return "截止日"
}

func (m Model) View() string { return m.viewList() }
