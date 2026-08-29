package tui

import (
	"errors"
	"strings"
	"testing"

	"todo.mirumo.net/internal/store"
)

func TestSpaceAsksBeforeMarkingDone(t *testing.T) {
	m, s := newModel(t)
	id := m.tasks[0].ID

	m = press(t, m, " ")
	if m.mode != modeConfirm {
		t.Fatalf("space 應該先問過，mode = %v", m.mode)
	}
	if got, _ := s.Get(id); got.Done() {
		t.Error("還沒確認就不該改到資料")
	}
	v := m.View()
	if !strings.Contains(v, "第一件") || !strings.Contains(v, "y/n") {
		t.Errorf("確認提示應該指名項目並說明按鍵：%q", v)
	}

	m = press(t, m, "y")
	if m.mode != modeList {
		t.Errorf("確認後應回到清單，mode = %v", m.mode)
	}
	if got, _ := s.Get(id); !got.Done() {
		t.Error("按 y 之後應該真的標成完成")
	}
}

func TestConfirmCancelLeavesDataUnchanged(t *testing.T) {
	for _, cancelKey := range []string{"n", "esc", "q"} {
		m, s := newModel(t)
		id := m.tasks[0].ID
		m = press(t, m, " ")
		m = press(t, m, cancelKey)
		if m.mode != modeList {
			t.Errorf("%s 之後應回到清單，mode = %v", cancelKey, m.mode)
		}
		if got, _ := s.Get(id); got.Done() {
			t.Errorf("按 %s 取消後不該有任何變更", cancelKey)
		}
	}
}

func TestUnmarkingDoneNeedsNoConfirmation(t *testing.T) {
	// 完成需要確認，所以誤按不會發生；取消完成本身就是回復動作，再問一次只是噪音。
	m, s := newModel(t)
	id := m.tasks[0].ID
	m = press(t, m, " ")
	m = press(t, m, "y")
	m = press(t, m, "A") // 顯示已完成，游標才找得到它
	m = press(t, m, "g") // 有截止日者排最前，第一件就在頂端
	if cur, ok := m.current(); !ok || !cur.Done() {
		t.Fatalf("游標應該落在已完成那筆，實得 %+v", cur)
	}

	m = press(t, m, " ")
	if m.mode != modeList {
		t.Fatalf("取消完成不該再問一次，mode = %v", m.mode)
	}
	if got, _ := s.Get(id); got.Done() {
		t.Error("應該已經取消完成")
	}
}

func TestDeleteAsksBeforeDeleting(t *testing.T) {
	m, s := newModel(t)
	victim := m.tasks[0]

	m = press(t, m, "d")
	if m.mode != modeConfirm {
		t.Fatalf("d 應該先問過，mode = %v", m.mode)
	}
	if _, err := s.Get(victim.ID); err != nil {
		t.Error("還沒確認就不該刪掉")
	}
	if !strings.Contains(m.View(), victim.Title) {
		t.Errorf("確認提示應該指名要刪哪一筆：%q", m.View())
	}

	m = press(t, m, "y")
	if _, err := s.Get(victim.ID); !errors.Is(err, store.ErrNotFound) {
		t.Error("按 y 之後應該真的刪掉")
	}
	if !strings.Contains(m.View(), "u 復原") {
		t.Errorf("刪除後仍應保留 undo：%q", m.View())
	}
}

func TestDeleteCancelKeepsTheTask(t *testing.T) {
	m, s := newModel(t)
	victim := m.tasks[0]
	m = press(t, m, "d")
	m = press(t, m, "esc")
	if _, err := s.Get(victim.ID); err != nil {
		t.Errorf("取消後該項目應該還在：%v", err)
	}
	if len(m.tasks) != 3 {
		t.Errorf("清單應該沒變，實得 %d 筆", len(m.tasks))
	}
}

func TestConfirmOnEmptyListDoesNothing(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "/")
	m = press(t, m, "沒有這種東西")
	m = press(t, m, "enter")
	if len(m.tasks) != 0 {
		t.Fatalf("預期空清單，實得 %d 筆", len(m.tasks))
	}
	for _, k := range []string{" ", "d"} {
		m2 := press(t, m, k)
		if m2.mode != modeList {
			t.Errorf("清單空的時候按 %q 不該開啟確認", k)
		}
	}
}
