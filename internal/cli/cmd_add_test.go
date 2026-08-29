package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"todo.mirumo.net/internal/task"
)

func TestAddMinimal(t *testing.T) {
	app, out, _ := newApp(t)
	if code := app.Run([]string{"add", "  買牛奶  "}); code != 0 {
		t.Fatalf("離開碼 = %d", code)
	}
	if !strings.Contains(out.String(), "已新增 #1：買牛奶") {
		t.Errorf("stdout = %q", out.String())
	}
	got, err := app.Store.Get(1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "買牛奶" {
		t.Errorf("標題 = %q，預期去掉頭尾空白", got.Title)
	}
	if got.Project != "" {
		t.Errorf("project = %q，沒給 -p 就該是全域未分類", got.Project)
	}
}

func TestAddAllFlags(t *testing.T) {
	app, _, _ := newApp(t)
	code := app.Run([]string{"add", "買牛奶", "-t", "購物", "--tag=家務", "-d", "tomorrow", "--pri", "high"})
	if code != 0 {
		t.Fatalf("離開碼 = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.Due == nil || got.Due.Format("2006-01-02") != "2026-08-30" {
		t.Errorf("due = %v，預期 2026-08-30", got.Due)
	}
	if got.Priority != task.PriHigh {
		t.Errorf("priority = %v", got.Priority)
	}
	if len(got.Tags) != 2 {
		t.Errorf("tags = %v", got.Tags)
	}
}

func TestAddProjectFromCwd(t *testing.T) {
	app, _, _ := newApp(t)
	root := app.Cwd
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if code := app.Run([]string{"add", "修 bug", "-p"}); code != 0 {
		t.Fatalf("離開碼 = %d", code)
	}
	got, _ := app.Store.Get(1)
	want, _ := filepath.EvalSymlinks(root)
	gotResolved, _ := filepath.EvalSymlinks(got.Project)
	if gotResolved != want {
		t.Errorf("project = %q，預期當前 repo 根 %q", gotResolved, want)
	}
}

func TestAddProjectExplicitName(t *testing.T) {
	app, _, _ := newApp(t)
	if code := app.Run([]string{"add", "修 bug", "-p", "work"}); code != 0 {
		t.Fatalf("離開碼 = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.Project != "work" {
		t.Errorf("project = %q，預期 work", got.Project)
	}
}

func TestAddMissingTitleExplainsTheFootgun(t *testing.T) {
	app, _, errBuf := newApp(t)
	if code := app.Run([]string{"add", "-p", "買牛奶"}); code != 1 {
		t.Fatalf("離開碼 = %d，預期 1", code)
	}
	msg := errBuf.String()
	if !strings.Contains(msg, "缺少標題") || !strings.Contains(msg, "--project=買牛奶") {
		t.Errorf("錯誤訊息應指出被 -p 吃掉並給出修正寫法，實得：%q", msg)
	}
}

func TestAddRejectsBadValues(t *testing.T) {
	cases := [][]string{
		{"add", "x", "--pri", "urgent"},
		{"add", "x", "-d", "someday"},
		{"add", "   "},
		{"add", "a", "b"},
	}
	for _, args := range cases {
		app, _, errBuf := newApp(t)
		if code := app.Run(args); code == 0 {
			t.Errorf("%v 應該失敗", args)
		}
		if errBuf.Len() == 0 {
			t.Errorf("%v 應該印出錯誤訊息", args)
		}
	}
}
