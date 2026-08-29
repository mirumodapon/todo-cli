package tui

import (
	"errors"
	"strings"
	"testing"

	"todo.mirumo.net/internal/store"
	"todo.mirumo.net/internal/task"
)

func TestDeleteThenUndo(t *testing.T) {
	m, s := newModel(t)
	victim := m.tasks[0]

	m = press(t, m, "d")
	m = press(t, m, "y")
	if _, err := s.Get(victim.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("d 應該刪掉項目，err = %v", err)
	}
	if len(m.tasks) != 2 {
		t.Errorf("刪除後剩 %d 筆，預期 2 筆", len(m.tasks))
	}
	if !strings.Contains(m.View(), "u 復原") {
		t.Errorf("底部應提示可以復原：\n%s", m.View())
	}

	m = press(t, m, "u")
	back, err := s.Get(victim.ID)
	if err != nil {
		t.Fatalf("u 應該以原 id 復原，err = %v", err)
	}
	if back.Title != victim.Title || len(back.Tags) != len(victim.Tags) {
		t.Errorf("復原內容不符：%+v", back)
	}
	if len(m.tasks) != 3 {
		t.Errorf("復原後 %d 筆，預期 3 筆", len(m.tasks))
	}
}

func TestUndoOnlyKeepsOneLevel(t *testing.T) {
	m, s := newModel(t)
	first := m.tasks[0]
	m = press(t, m, "d")
	m = press(t, m, "y")
	second := m.tasks[0]
	m = press(t, m, "d")
	m = press(t, m, "y")
	m = press(t, m, "u")

	if _, err := s.Get(second.ID); err != nil {
		t.Errorf("最後刪的那筆應該被復原：%v", err)
	}
	if _, err := s.Get(first.ID); !errors.Is(err, store.ErrNotFound) {
		t.Error("只保留一層 undo，更早刪的不該回來")
	}
	m = press(t, m, "u")
	if !strings.Contains(m.View(), "沒有可復原") {
		t.Errorf("沒有可復原時應該說一聲：\n%s", m.View())
	}
}

func TestSearchFiltersIncrementally(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "/")
	if m.mode != modeSearch {
		t.Fatal("/ 應該進入搜尋模式")
	}
	m = press(t, m, "第二")
	if len(m.tasks) != 1 || m.tasks[0].Title != "第二件" {
		t.Errorf("打字應該即時過濾，實得 %d 筆", len(m.tasks))
	}
	m = press(t, m, "enter")
	if m.mode != modeList {
		t.Error("enter 應該回到清單並保留過濾")
	}
	if len(m.tasks) != 1 {
		t.Errorf("enter 後過濾應保留，實得 %d 筆", len(m.tasks))
	}
}

func TestSearchEscCancels(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "/")
	m = press(t, m, "第二")
	m = press(t, m, "esc")
	if m.mode != modeList {
		t.Error("esc 應該回到清單")
	}
	if len(m.tasks) != 3 {
		t.Errorf("esc 應該取消過濾，實得 %d 筆", len(m.tasks))
	}
}

func TestToggleIncludeDone(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, " ")
	m = press(t, m, "y")
	if len(m.tasks) != 2 {
		t.Fatalf("預期剩 2 筆，實得 %d", len(m.tasks))
	}
	m = press(t, m, "A")
	if len(m.tasks) != 3 {
		t.Errorf("A 應該連已完成一起顯示，實得 %d 筆", len(m.tasks))
	}
	m = press(t, m, "A")
	if len(m.tasks) != 2 {
		t.Errorf("再按 A 應該切回只看未完成，實得 %d 筆", len(m.tasks))
	}
}

func TestSortCycles(t *testing.T) {
	m, _ := newModel(t)
	if m.filter.Sort != task.SortDue {
		t.Fatal("預設應為 due")
	}
	m = press(t, m, "s")
	if m.filter.Sort != task.SortPriority {
		t.Errorf("s 之後 = %v，預期 pri", m.filter.Sort)
	}
	m = press(t, m, "s")
	if m.filter.Sort != task.SortCreated {
		t.Errorf("再按 s = %v，預期 created", m.filter.Sort)
	}
	m = press(t, m, "s")
	if m.filter.Sort != task.SortDue {
		t.Errorf("循環回來 = %v，預期 due", m.filter.Sort)
	}
}

func TestEscClearsAllFilters(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "/")
	m = press(t, m, "第二")
	m = press(t, m, "enter")
	m = press(t, m, "A")
	m = press(t, m, "esc")
	if len(m.tasks) != 3 {
		t.Errorf("esc 應該清掉所有過濾，實得 %d 筆", len(m.tasks))
	}
	if m.filter.Search != "" || m.filter.IncludeDone {
		t.Errorf("filter 應該歸零：%+v", m.filter)
	}
}
