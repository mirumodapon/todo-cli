package cli

import (
	"strings"
	"testing"
)

func TestDoneAndUndone(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "買牛奶"})
	out.Reset()

	if code := app.Run([]string{"done", "1"}); code != 0 {
		t.Fatalf("離開碼 = %d", code)
	}
	if !strings.Contains(out.String(), "已完成 #1：買牛奶") {
		t.Errorf("stdout = %q", out.String())
	}
	got, _ := app.Store.Get(1)
	if !got.Done() {
		t.Error("應為已完成")
	}

	out.Reset()
	app.Run([]string{"undone", "1"})
	if !strings.Contains(out.String(), "已取消完成 #1") {
		t.Errorf("stdout = %q", out.String())
	}
	got, _ = app.Store.Get(1)
	if got.Done() {
		t.Error("應為未完成")
	}
}

func TestDoneAcceptsMultipleIDs(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "a"})
	app.Run([]string{"add", "b"})
	out.Reset()
	if code := app.Run([]string{"done", "1", "2"}); code != 0 {
		t.Fatalf("離開碼 = %d", code)
	}
	if strings.Count(out.String(), "已完成") != 2 {
		t.Errorf("stdout = %q，預期兩行", out.String())
	}
}

func TestMarkMissingIDNamesTheID(t *testing.T) {
	app, _, errBuf := newApp(t)
	if code := app.Run([]string{"done", "42"}); code != 1 {
		t.Fatalf("離開碼 = %d，預期 1", code)
	}
	if !strings.Contains(errBuf.String(), "#42") {
		t.Errorf("錯誤訊息應指出是哪個 id：%q", errBuf.String())
	}
}

func TestRm(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "買牛奶"})
	out.Reset()
	if code := app.Run([]string{"rm", "1"}); code != 0 {
		t.Fatalf("離開碼 = %d", code)
	}
	if !strings.Contains(out.String(), "已刪除 #1：買牛奶") {
		t.Errorf("stdout = %q", out.String())
	}
	if _, err := app.Store.Get(1); err == nil {
		t.Error("應該被刪掉了")
	}
}

func TestMarkRequiresID(t *testing.T) {
	for _, cmd := range []string{"done", "undone", "rm"} {
		app, _, errBuf := newApp(t)
		if code := app.Run([]string{cmd}); code != 1 {
			t.Errorf("%s 沒給 id 時離開碼 = %d，預期 1", cmd, code)
		}
		if errBuf.Len() == 0 {
			t.Errorf("%s 應該印出錯誤", cmd)
		}
	}
}
