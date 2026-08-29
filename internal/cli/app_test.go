package cli

import (
	"errors"
	"strings"
	"testing"
)

func TestRunNoArgsPrintsUsage(t *testing.T) {
	app, out, _ := newApp(t)
	if code := app.Run(nil); code != 0 {
		t.Errorf("離開碼 = %d，預期 0", code)
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Errorf("裸打 todo 應印出用法，實得：%q", out.String())
	}
	if !strings.Contains(out.String(), "Commands:") {
		t.Errorf("全域說明應列出子指令，實得：%q", out.String())
	}
	if strings.Contains(out.String(), "沒有符合的待辦") {
		t.Error("裸打 todo 不該進入清單或 TUI")
	}
}

func TestRunHelp(t *testing.T) {
	for _, arg := range []string{"-h", "--help", "help"} {
		app, out, _ := newApp(t)
		if code := app.Run([]string{arg}); code != 0 {
			t.Errorf("%s 離開碼 = %d，預期 0", arg, code)
		}
		if !strings.Contains(out.String(), "Usage:") {
			t.Errorf("%s 沒印出用法", arg)
		}
	}
}

func TestSubcommandHelpIsSpecificToThatCommand(t *testing.T) {
	// A subcommand's flag set does not know -h, so it must be caught before dispatch,
	// and it must print that subcommand's own help rather than the global usage.
	app, out, errBuf := newApp(t)
	if code := app.Run([]string{"add", "-h"}); code != 0 {
		t.Errorf("離開碼 = %d，預期 0；stderr = %q", code, errBuf.String())
	}
	s := out.String()
	if !strings.Contains(s, "todo add <title>") {
		t.Errorf("應印出 add 自己的用法行，實得：%q", s)
	}
	if !strings.Contains(s, "-p, --project") || !strings.Contains(s, "-d, --due") {
		t.Errorf("應列出 add 的 flag，實得：%q", s)
	}
	if strings.Contains(s, "Commands:") {
		t.Errorf("子指令說明不該退回全域說明：%q", s)
	}
}

func TestEverySubcommandHasHelp(t *testing.T) {
	for _, name := range []string{"add", "ls", "done", "undone", "edit", "rm", "projects", "tags", "tui"} {
		app, out, errBuf := newApp(t)
		if code := app.Run([]string{name, "--help"}); code != 0 {
			t.Errorf("%s --help 離開碼 = %d；stderr = %q", name, code, errBuf.String())
			continue
		}
		s := out.String()
		if !strings.Contains(s, "Usage:") || !strings.Contains(s, "todo "+name) {
			t.Errorf("%s --help 沒印出自己的用法：%q", name, s)
		}
	}
}

func TestHelpSubcommandTakesACommandName(t *testing.T) {
	app, out, _ := newApp(t)
	if code := app.Run([]string{"help", "ls"}); code != 0 {
		t.Fatalf("離開碼 = %d", code)
	}
	if !strings.Contains(out.String(), "todo ls") || strings.Contains(out.String(), "Commands:") {
		t.Errorf("todo help ls 應印出 ls 的說明：%q", out.String())
	}
}

func TestHelpTextIsEnglish(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run(nil)
	for _, banned := range []string{"用法", "指令", "顯示"} {
		if strings.Contains(out.String(), banned) {
			t.Errorf("說明文字應為英文，卻含有 %q：\n%s", banned, out.String())
		}
	}
}

func TestRunUnknownCommand(t *testing.T) {
	app, _, errBuf := newApp(t)
	if code := app.Run([]string{"frobnicate"}); code != 2 {
		t.Errorf("離開碼 = %d，預期 2", code)
	}
	if !strings.Contains(errBuf.String(), `unknown command "frobnicate"`) {
		t.Errorf("stderr = %q", errBuf.String())
	}
}

func TestTUIOnlyOnExplicitSubcommand(t *testing.T) {
	app, _, _ := newApp(t)
	called := false
	app.RunTUI = func() error { called = true; return nil }

	if code := app.Run(nil); code != 0 || called {
		t.Error("裸打 todo 不該啟動 TUI")
	}
	if code := app.Run([]string{"tui"}); code != 0 {
		t.Errorf("離開碼 = %d，預期 0", code)
	}
	if !called {
		t.Error("todo tui 應該啟動 TUI")
	}
}

func TestTUIErrorBecomesExitCode1(t *testing.T) {
	app, _, errBuf := newApp(t)
	app.RunTUI = func() error { return errors.New("終端機壞了") }
	if code := app.Run([]string{"tui"}); code != 1 {
		t.Errorf("離開碼 = %d，預期 1", code)
	}
	if !strings.Contains(errBuf.String(), "終端機壞了") {
		t.Errorf("stderr = %q", errBuf.String())
	}
}

func TestSplitGlobal(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantDB   string
		wantRest []string
	}{
		{"沒有 --db", []string{"ls", "-a"}, "", []string{"ls", "-a"}},
		{"空格式", []string{"--db", "/tmp/x.db", "ls"}, "/tmp/x.db", []string{"ls"}},
		{"等號式", []string{"--db=/tmp/x.db", "ls"}, "/tmp/x.db", []string{"ls"}},
		{"只有 --db", []string{"--db=/tmp/x.db"}, "/tmp/x.db", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db, rest, err := SplitGlobal(c.args)
			if err != nil {
				t.Fatalf("非預期錯誤：%v", err)
			}
			if db != c.wantDB {
				t.Errorf("db = %q，預期 %q", db, c.wantDB)
			}
			if strings.Join(rest, " ") != strings.Join(c.wantRest, " ") {
				t.Errorf("rest = %v，預期 %v", rest, c.wantRest)
			}
		})
	}
	if _, _, err := SplitGlobal([]string{"--db"}); err == nil {
		t.Error("--db 缺值應該報錯")
	}
}

func TestParseIDs(t *testing.T) {
	got, err := parseIDs([]string{"3", "17"})
	if err != nil || len(got) != 2 || got[0] != 3 || got[1] != 17 {
		t.Errorf("= %v, %v", got, err)
	}
	for _, bad := range [][]string{{}, {"x"}, {"0"}, {"-1"}} {
		if _, err := parseIDs(bad); err == nil {
			t.Errorf("parseIDs(%v) 應該報錯", bad)
		}
	}
}
