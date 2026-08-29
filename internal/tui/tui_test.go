package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"todo.mirumo.net/internal/store"
	"todo.mirumo.net/internal/task"
)

func refTime() time.Time { return time.Date(2026, 8, 29, 15, 0, 0, 0, time.Local) }

func day(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	return &t
}

// newModel 建一個接上 in-memory 資料庫、已載入資料的 Model。
func newModel(t *testing.T) (Model, store.Store) {
	t.Helper()
	s, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	for _, ti := range []task.Task{
		{Title: "第一件", Due: day(2026, 8, 29), Priority: task.PriHigh, Tags: []string{"急"}},
		{Title: "第二件", Project: "/p/work"},
		{Title: "第三件"},
	} {
		ti.CreatedAt, ti.UpdatedAt = refTime(), refTime()
		if _, err := s.Add(ti); err != nil {
			t.Fatal(err)
		}
	}
	m := New(s, refTime, t.TempDir())
	m, _ = send(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, msg := run(t, m, m.Init())
	m, _ = send(t, m, msg)
	return m, s
}

// key 把按鍵字串轉成 tea.KeyMsg。
func key(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "ctrl+p":
		return tea.KeyMsg{Type: tea.KeyCtrlP}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// send 餵一個 msg，回傳新 model 與 cmd 執行後的結果 msg（沒有 cmd 時為 nil）。
func send(t *testing.T, m Model, msg tea.Msg) (Model, tea.Msg) {
	t.Helper()
	next, cmd := m.Update(msg)
	return run(t, next.(Model), cmd)
}

func run(t *testing.T, m Model, cmd tea.Cmd) (Model, tea.Msg) {
	t.Helper()
	if cmd == nil {
		return m, nil
	}
	return m, cmd()
}

// press 按一個鍵，並把它引發的 cmd 結果也餵回去（模擬 Bubble Tea 的迴圈一輪）。
func press(t *testing.T, m Model, k string) Model {
	t.Helper()
	m, msg := send(t, m, key(k))
	for i := 0; msg != nil && i < 4; i++ {
		m, msg = send(t, m, msg)
	}
	return m
}

func TestInitLoadsTasks(t *testing.T) {
	m, _ := newModel(t)
	if len(m.tasks) != 3 {
		t.Fatalf("載入 %d 筆，預期 3 筆", len(m.tasks))
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d，預期 0", m.cursor)
	}
}

func TestNavigation(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "j")
	if m.cursor != 1 {
		t.Errorf("j 之後 cursor = %d，預期 1", m.cursor)
	}
	m = press(t, m, "down")
	m = press(t, m, "down")
	if m.cursor != 2 {
		t.Errorf("到底之後 cursor = %d，預期停在 2 不越界", m.cursor)
	}
	m = press(t, m, "k")
	if m.cursor != 1 {
		t.Errorf("k 之後 cursor = %d，預期 1", m.cursor)
	}
	m = press(t, m, "g")
	if m.cursor != 0 {
		t.Errorf("g 之後 cursor = %d，預期 0", m.cursor)
	}
	m = press(t, m, "G")
	if m.cursor != 2 {
		t.Errorf("G 之後 cursor = %d，預期 2", m.cursor)
	}
	m = press(t, m, "k")
	m = press(t, m, "k")
	m = press(t, m, "k")
	if m.cursor != 0 {
		t.Errorf("到頂之後 cursor = %d，預期停在 0 不越界", m.cursor)
	}
}

func TestSpaceTogglesDone(t *testing.T) {
	m, s := newModel(t)
	id := m.tasks[0].ID
	m = press(t, m, " ")

	got, err := s.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Done() {
		t.Error("space 應該把項目標成已完成")
	}
	if len(m.tasks) != 2 {
		t.Errorf("重查後剩 %d 筆，預期 2 筆", len(m.tasks))
	}
}

func TestQuit(t *testing.T) {
	m, _ := newModel(t)
	_, cmd := m.Update(key("q"))
	if cmd == nil {
		t.Fatal("q 應該回傳一個 cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q 應該離開程式")
	}
}

func TestErrMsgShowsWithoutCrashing(t *testing.T) {
	m, _ := newModel(t)
	m, _ = send(t, m, errMsg{err: errFake})
	if m.err == nil {
		t.Fatal("錯誤應該被記下來")
	}
	if !strings.Contains(m.View(), "壞掉了") {
		t.Errorf("錯誤應該顯示在畫面上：%q", m.View())
	}
}

func TestViewShowsTasksAndCursor(t *testing.T) {
	m, _ := newModel(t)
	v := m.View()
	for _, want := range []string{"第一件", "第二件", "第三件", "今天", "!高", "@急", "work"} {
		if !strings.Contains(v, want) {
			t.Errorf("畫面缺少 %q：\n%s", want, v)
		}
	}
	if !strings.Contains(v, "▸") {
		t.Errorf("畫面應該有游標標記：\n%s", v)
	}
}

var errFake = fakeErr{}

type fakeErr struct{}

func (fakeErr) Error() string { return "資料庫壞掉了" }
