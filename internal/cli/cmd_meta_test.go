package cli

import (
	"strings"
	"testing"
)

func TestProjectsListsCountsAndUncategorized(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "全域的事"})
	app.Run([]string{"add", "工作 A", "-p", "/p/work"})
	app.Run([]string{"add", "工作 B", "-p", "/p/work"})
	app.Run([]string{"done", "3"})
	out.Reset()

	if code := app.Run([]string{"projects"}); code != 0 {
		t.Fatalf("離開碼 = %d", code)
	}
	s := out.String()
	if !strings.Contains(s, "（未分類）") {
		t.Errorf("應該把空專案顯示為（未分類）：%q", s)
	}
	if !strings.Contains(s, "work") || !strings.Contains(s, "1 未完成") {
		t.Errorf("應顯示 basename 與未完成數：%q", s)
	}
	if !strings.Contains(s, "/p/work") {
		t.Errorf("有專案者應附上完整路徑：%q", s)
	}
}

func TestProjectsEmpty(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"projects"})
	if !strings.Contains(out.String(), "還沒有任何待辦") {
		t.Errorf("= %q", out.String())
	}
}

func TestTags(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "x", "-t", "購物", "-t", "家務"})
	out.Reset()
	app.Run([]string{"tags"})
	s := out.String()
	if !strings.Contains(s, "@家務") || !strings.Contains(s, "@購物") {
		t.Errorf("= %q", s)
	}
}

func TestTagsEmpty(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"tags"})
	if !strings.Contains(out.String(), "還沒有任何標籤") {
		t.Errorf("= %q", out.String())
	}
}

func TestMetaCommandsRejectArgs(t *testing.T) {
	for _, cmd := range []string{"projects", "tags"} {
		app, _, errBuf := newApp(t)
		if code := app.Run([]string{cmd, "垃圾"}); code != 1 {
			t.Errorf("%s 離開碼 = %d，預期 1", cmd, code)
		}
		if !strings.Contains(errBuf.String(), "不接受參數") {
			t.Errorf("%s stderr = %q", cmd, errBuf.String())
		}
	}
}
