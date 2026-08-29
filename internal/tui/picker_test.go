package tui

import (
	"strings"
	"testing"
)

func TestProjectPickerFiltersByProject(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "P")
	if m.mode != modePicker {
		t.Fatal("P 應該開啟選單")
	}
	v := m.View()
	for _, want := range []string{"全部", "（未分類）", "work"} {
		if !strings.Contains(v, want) {
			t.Errorf("選單缺少 %q：\n%s", want, v)
		}
	}
	// Row 0 is the clear-filter entry, row 1 is uncategorised, row 2 is work.
	m = press(t, m, "j")
	m = press(t, m, "j")
	m = press(t, m, "enter")
	if m.mode != modeList {
		t.Fatal("enter 應該回到清單")
	}
	if len(m.tasks) != 1 || m.tasks[0].Title != "第二件" {
		t.Errorf("選 work 後應只剩第二件，實得 %d 筆", len(m.tasks))
	}
}

func TestProjectPickerUncategorized(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "P")
	m = press(t, m, "j")
	m = press(t, m, "enter")
	if len(m.tasks) != 2 {
		t.Errorf("（未分類）應剩 2 筆，實得 %d 筆", len(m.tasks))
	}
	if m.filter.Project == nil || *m.filter.Project != "" {
		t.Errorf("filter.Project = %v，預期指向空字串", m.filter.Project)
	}
}

func TestProjectPickerAllClearsFilter(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "P")
	m = press(t, m, "j")
	m = press(t, m, "enter")
	m = press(t, m, "P")
	m = press(t, m, "enter") // row 0 clears the filter
	if m.filter.Project != nil {
		t.Errorf("選「全部」應該清掉專案過濾，實得 %v", m.filter.Project)
	}
	if len(m.tasks) != 3 {
		t.Errorf("實得 %d 筆，預期 3 筆", len(m.tasks))
	}
}

func TestTagPicker(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "T")
	if m.mode != modePicker {
		t.Fatal("T 應該開啟選單")
	}
	if !strings.Contains(m.View(), "@急") {
		t.Errorf("標籤選單應列出 @急：\n%s", m.View())
	}
	m = press(t, m, "j")
	m = press(t, m, "enter")
	if len(m.tasks) != 1 || m.tasks[0].Title != "第一件" {
		t.Errorf("選 @急 後應只剩第一件，實得 %d 筆", len(m.tasks))
	}
}

func TestPickerEscCancels(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "P")
	m = press(t, m, "esc")
	if m.mode != modeList {
		t.Error("esc 應該關掉選單")
	}
	if m.filter.Project != nil {
		t.Error("esc 不該套用任何過濾")
	}
}
