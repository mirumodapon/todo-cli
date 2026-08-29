package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"todo.mirumo.net/internal/task"
)

// typeInto 依序把字串打進目前聚焦的欄位。
func typeInto(t *testing.T, m Model, s string) Model {
	t.Helper()
	return press(t, m, s)
}

func TestFormAddCreatesTask(t *testing.T) {
	m, s := newModel(t)
	m = press(t, m, "a")
	if m.mode != modeForm {
		t.Fatal("a 應該開啟表單")
	}
	m = typeInto(t, m, "第四件")
	m = press(t, m, "tab")
	m = press(t, m, "tab")
	m = typeInto(t, m, "急,雜")
	m = press(t, m, "tab")
	m = typeInto(t, m, "tomorrow")
	m = press(t, m, "tab")
	m = typeInto(t, m, "high")
	m = press(t, m, "enter")

	if m.mode != modeList {
		t.Fatalf("儲存後應回到清單，mode = %v", m.mode)
	}
	ts, err := s.List(task.Filter{Search: "第四件"}, refTime())
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) != 1 {
		t.Fatalf("應該新增了一筆，實得 %d 筆", len(ts))
	}
	got := ts[0]
	if got.Due == nil || got.Due.Format("2006-01-02") != "2026-08-30" {
		t.Errorf("due = %v", got.Due)
	}
	if got.Priority != task.PriHigh {
		t.Errorf("priority = %v", got.Priority)
	}
	if len(got.Tags) != 2 {
		t.Errorf("tags = %v，逗號分隔應拆成兩個", got.Tags)
	}
}

func TestFormEditPrefillsAndUpdates(t *testing.T) {
	m, s := newModel(t)
	id := m.tasks[0].ID
	m = press(t, m, "e")
	if m.mode != modeForm {
		t.Fatal("e 應該開啟表單")
	}
	if !strings.Contains(m.View(), "第一件") {
		t.Errorf("編輯時應預填現有值：\n%s", m.View())
	}
	for range len([]rune("第一件")) {
		m = press(t, m, "backspace")
	}
	m = typeInto(t, m, "改過的")
	m = press(t, m, "enter")

	got, err := s.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "改過的" {
		t.Errorf("title = %q", got.Title)
	}
	if got.Due == nil {
		t.Error("沒動到的欄位應該保持原值")
	}
}

func TestFormRejectsEmptyTitle(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "a")
	m = press(t, m, "enter")
	if m.mode != modeForm {
		t.Error("標題空白時不該離開表單")
	}
	if !strings.Contains(m.View(), "標題不能是空的") {
		t.Errorf("應該說明為什麼存不了：\n%s", m.View())
	}
}

func TestFormRejectsBadDue(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "a")
	m = typeInto(t, m, "測試")
	m = press(t, m, "tab")
	m = press(t, m, "tab")
	m = press(t, m, "tab")
	m = typeInto(t, m, "someday")
	m = press(t, m, "enter")
	if m.mode != modeForm {
		t.Error("日期不合法時不該離開表單")
	}
	if !strings.Contains(m.View(), "看不懂的日期") {
		t.Errorf("應該指出是日期的問題：\n%s", m.View())
	}
}

func TestFormEscCancels(t *testing.T) {
	m, s := newModel(t)
	before, _ := s.List(task.Filter{}, refTime())
	m = press(t, m, "a")
	m = typeInto(t, m, "不要存")
	m = press(t, m, "esc")
	if m.mode != modeList {
		t.Error("esc 應該回到清單")
	}
	after, _ := s.List(task.Filter{}, refTime())
	if len(after) != len(before) {
		t.Error("esc 不該存進任何東西")
	}
}

func TestFormFillsProjectFromCwd(t *testing.T) {
	m, _ := newModel(t)
	if err := os.MkdirAll(filepath.Join(m.cwd, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	m = press(t, m, "a")
	m = press(t, m, "ctrl+p")
	if !strings.Contains(m.View(), filepath.Base(m.cwd)) {
		t.Errorf("ctrl+p 應該填入當前目錄的專案：\n%s", m.View())
	}
}

func TestHelpOverlay(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "?")
	if m.mode != modeHelp {
		t.Fatal("? 應該開啟說明")
	}
	v := m.View()
	for _, want := range []string{"space", "d", "u", "/", "P", "T"} {
		if !strings.Contains(v, want) {
			t.Errorf("說明缺少 %q：\n%s", want, v)
		}
	}
	m = press(t, m, "esc")
	if m.mode != modeList {
		t.Error("esc 應該關掉說明")
	}
}
