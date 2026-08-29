package store

import (
	"errors"
	"testing"

	"todo.mirumo.net/internal/task"
)

func TestSetDoneAndUndone(t *testing.T) {
	s := newStore(t)
	got, _ := s.Add(sample())
	if err := s.SetDone(got.ID, true, ref()); err != nil {
		t.Fatalf("SetDone：%v", err)
	}
	back, _ := s.Get(got.ID)
	if !back.Done() {
		t.Fatal("應為已完成")
	}
	if back.DoneAt.Format("2006-01-02") != "2026-08-29" {
		t.Errorf("done_at = %v，預期記錄完成時間", back.DoneAt)
	}
	if err := s.SetDone(got.ID, false, ref()); err != nil {
		t.Fatal(err)
	}
	back, _ = s.Get(got.ID)
	if back.Done() {
		t.Error("取消完成後 DoneAt 應為 nil")
	}
}

func TestSetDoneMissingID(t *testing.T) {
	s := newStore(t)
	if err := s.SetDone(42, true, ref()); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v，預期 ErrNotFound", err)
	}
}

func TestUpdateOverwritesFieldsAndTags(t *testing.T) {
	s := newStore(t)
	got, _ := s.Add(sample())
	got.Title = "買豆漿"
	got.Project = ""
	got.Due = nil
	got.Priority = task.PriLow
	got.Tags = []string{"早餐"}
	if err := s.Update(got); err != nil {
		t.Fatalf("Update：%v", err)
	}
	back, _ := s.Get(got.ID)
	if back.Title != "買豆漿" || back.Project != "" || back.Due != nil || back.Priority != task.PriLow {
		t.Errorf("欄位沒更新：%+v", back)
	}
	if len(back.Tags) != 1 || back.Tags[0] != "早餐" {
		t.Errorf("tags = %v，預期整組被取代成 [早餐]", back.Tags)
	}
}

func TestUpdateMissingID(t *testing.T) {
	s := newStore(t)
	if err := s.Update(task.Task{ID: 7, Title: "x", CreatedAt: ref(), UpdatedAt: ref()}); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v，預期 ErrNotFound", err)
	}
}

func TestDeleteRemovesTagLinks(t *testing.T) {
	s := newStore(t)
	got, _ := s.Add(sample())
	if err := s.Delete(got.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(got.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v，預期 ErrNotFound", err)
	}
	tags, err := s.Tags()
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 0 {
		t.Errorf("Tags() = %v，預期空：只列被引用的標籤", tags)
	}
}

func TestRestoreReusesOriginalID(t *testing.T) {
	s := newStore(t)
	got, _ := s.Add(sample())
	original, _ := s.Get(got.ID)
	if err := s.Delete(got.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Restore(original); err != nil {
		t.Fatalf("Restore：%v", err)
	}
	back, err := s.Get(original.ID)
	if err != nil {
		t.Fatalf("復原後應能用原 id 取回：%v", err)
	}
	if back.Title != original.Title || len(back.Tags) != len(original.Tags) {
		t.Errorf("復原內容不符：%+v", back)
	}
}

func TestTagsListsOnlyReferenced(t *testing.T) {
	s := newStore(t)
	seed(t, s)
	tags, err := s.Tags()
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 || tags[0] != "急" || tags[1] != "雜" {
		t.Errorf("= %v，預期 [急 雜] 依名稱排序", tags)
	}
}

func TestProjectsCountsOpenTasks(t *testing.T) {
	s := newStore(t)
	ids := seed(t, s)
	if err := s.SetDone(ids["工作上的事"], true, ref()); err != nil {
		t.Fatal(err)
	}
	ps, err := s.Projects()
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 2 {
		t.Fatalf("= %+v，預期兩個專案（含空字串的未分類）", ps)
	}
	if ps[0].Path != "" || ps[0].Open != 3 {
		t.Errorf("ps[0] = %+v，預期未分類 3 筆未完成", ps[0])
	}
	if ps[1].Path != "/p/work" || ps[1].Open != 1 {
		t.Errorf("ps[1] = %+v，預期 /p/work 1 筆未完成（另一筆已完成）", ps[1])
	}
}
