package cli

import (
	"strings"
	"testing"
)

// Seed a few tasks before exercising ls.
func seedCLI(t *testing.T, app *App) {
	t.Helper()
	cases := [][]string{
		{"add", "逾期的事", "-d", "2026-08-20"},
		{"add", "今天的事", "-d", "today", "--pri", "high", "-t", "急"},
		{"add", "工作的事", "-p", "work", "-t", "雜"},
		{"add", "沒期限的事"},
	}
	for _, args := range cases {
		if code := app.Run(args); code != 0 {
			t.Fatalf("%v 失敗", args)
		}
	}
}

func TestLsDefaultsToOpenOnly(t *testing.T) {
	app, out, _ := newApp(t)
	seedCLI(t, app)
	if code := app.Run([]string{"done", "1"}); code != 0 {
		t.Skip("done 尚未實作，Task 11 後再跑")
	}
	out.Reset()
	app.Run([]string{"ls"})
	if strings.Contains(out.String(), "逾期的事") {
		t.Errorf("預設不該列已完成：%q", out.String())
	}
	out.Reset()
	app.Run([]string{"ls", "-a"})
	if !strings.Contains(out.String(), "逾期的事") {
		t.Errorf("-a 應含已完成：%q", out.String())
	}
}

func TestLsFilterByProjectAndNoProject(t *testing.T) {
	app, out, _ := newApp(t)
	seedCLI(t, app)

	out.Reset()
	app.Run([]string{"ls", "-p", "work"})
	if !strings.Contains(out.String(), "工作的事") || strings.Contains(out.String(), "今天的事") {
		t.Errorf("-p work = %q", out.String())
	}

	out.Reset()
	app.Run([]string{"ls", "--no-project"})
	if strings.Contains(out.String(), "工作的事") {
		t.Errorf("--no-project 不該含有專案的項目：%q", out.String())
	}
	if !strings.Contains(out.String(), "今天的事") {
		t.Errorf("--no-project 應含未分類項目：%q", out.String())
	}
}

func TestLsRejectsConflictingProjectFlags(t *testing.T) {
	app, _, errBuf := newApp(t)
	if code := app.Run([]string{"ls", "-p", "work", "--no-project"}); code != 1 {
		t.Errorf("離開碼 = %d，預期 1", code)
	}
	if !strings.Contains(errBuf.String(), "不能同時使用") {
		t.Errorf("stderr = %q", errBuf.String())
	}
}

func TestLsDueKeywordsAndTags(t *testing.T) {
	app, out, _ := newApp(t)
	seedCLI(t, app)

	out.Reset()
	app.Run([]string{"ls", "-d", "today"})
	if !strings.Contains(out.String(), "今天的事") || strings.Contains(out.String(), "逾期的事") {
		t.Errorf("-d today = %q", out.String())
	}

	out.Reset()
	app.Run([]string{"ls", "-d", "overdue"})
	if !strings.Contains(out.String(), "逾期的事") {
		t.Errorf("-d overdue = %q", out.String())
	}

	out.Reset()
	app.Run([]string{"ls", "-t", "急"})
	if !strings.Contains(out.String(), "今天的事") || strings.Contains(out.String(), "工作的事") {
		t.Errorf("-t 急 = %q", out.String())
	}
}

func TestLsRejectsPositionalArgs(t *testing.T) {
	app, _, errBuf := newApp(t)
	if code := app.Run([]string{"ls", "垃圾"}); code != 1 {
		t.Errorf("離開碼 = %d，預期 1", code)
	}
	if !strings.Contains(errBuf.String(), "不接受位置參數") {
		t.Errorf("stderr = %q", errBuf.String())
	}
}

func TestLsBadSortAndPriority(t *testing.T) {
	app, _, _ := newApp(t)
	if code := app.Run([]string{"ls", "-s", "title"}); code == 0 {
		t.Error("未知的排序應該失敗")
	}
	if code := app.Run([]string{"ls", "--pri", "urgent"}); code == 0 {
		t.Error("未知的優先度應該失敗")
	}
}
