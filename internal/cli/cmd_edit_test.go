package cli

import (
	"strings"
	"testing"

	"todo.mirumo.net/internal/task"
)

func TestEditOnlyTouchesGivenFlags(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "買牛奶", "-d", "tomorrow", "--pri", "high", "-t", "購物", "-p", "work"})
	out.Reset()

	if code := app.Run([]string{"edit", "1", "--pri", "low"}); code != 0 {
		t.Fatalf("離開碼 = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.Priority != task.PriLow {
		t.Errorf("priority = %v，預期 low", got.Priority)
	}
	if got.Due == nil {
		t.Error("沒給 --due 就不該動到截止日")
	}
	if got.Project != "work" {
		t.Errorf("project = %q，沒給 -p 就不該動", got.Project)
	}
	if len(got.Tags) != 1 {
		t.Errorf("tags = %v，沒給 -t 就不該動", got.Tags)
	}
}

func TestEditEmptyDueClearsIt(t *testing.T) {
	app, _, _ := newApp(t)
	app.Run([]string{"add", "買牛奶", "-d", "tomorrow"})
	if code := app.Run([]string{"edit", "1", "--due", ""}); code != 0 {
		t.Fatalf("離開碼 = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.Due != nil {
		t.Errorf("due = %v，--due \"\" 應該清掉期限", got.Due)
	}
}

func TestEditEmptyProjectMakesItGlobal(t *testing.T) {
	app, _, _ := newApp(t)
	app.Run([]string{"add", "買牛奶", "-p", "work"})
	if code := app.Run([]string{"edit", "1", "--project="}); code != 0 {
		t.Fatalf("離開碼 = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.Project != "" {
		t.Errorf("project = %q，--project= 應該改回全域未分類", got.Project)
	}
}

func TestEditReplacesTagsWholesale(t *testing.T) {
	app, _, _ := newApp(t)
	app.Run([]string{"add", "買牛奶", "-t", "購物", "-t", "家務"})
	app.Run([]string{"edit", "1", "-t", "早餐"})
	got, _ := app.Store.Get(1)
	if len(got.Tags) != 1 || got.Tags[0] != "早餐" {
		t.Errorf("tags = %v，-t 應整組取代而非累加", got.Tags)
	}
}

func TestEditTitleViaSecondPositional(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "買牛奶"})
	out.Reset()
	if code := app.Run([]string{"edit", "1", "買豆漿"}); code != 0 {
		t.Fatalf("離開碼 = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.Title != "買豆漿" {
		t.Errorf("title = %q", got.Title)
	}
	if !strings.Contains(out.String(), "已更新 #1：買豆漿") {
		t.Errorf("stdout = %q", out.String())
	}
}

func TestEditErrors(t *testing.T) {
	app, _, _ := newApp(t)
	app.Run([]string{"add", "買牛奶"})
	for _, args := range [][]string{
		{"edit"},
		{"edit", "x"},
		{"edit", "42", "--pri", "low"},
		{"edit", "1", "a", "b"},
		{"edit", "1", "--due", "someday"},
	} {
		if code := app.Run(args); code == 0 {
			t.Errorf("%v 應該失敗", args)
		}
	}
}
