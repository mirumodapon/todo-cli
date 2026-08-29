package argparse

import (
	"strings"
	"testing"
)

func specs() *Set {
	return New(
		Spec{Long: "all", Short: "a", Kind: Bool, Usage: "含已完成"},
		Spec{Long: "due", Short: "d", Kind: String, Usage: "截止日"},
		Spec{Long: "tag", Short: "t", Kind: StringSlice, Usage: "標籤，可重複"},
		Spec{Long: "project", Short: "p", Kind: OptionalString, Usage: "專案"},
	)
}

func TestParseLongAndShortForms(t *testing.T) {
	r, err := specs().Parse([]string{"買牛奶", "--due", "2026-09-01", "-a", "-t", "購物", "--tag=家務"})
	if err != nil {
		t.Fatalf("非預期錯誤：%v", err)
	}
	if got := r.Args(); len(got) != 1 || got[0] != "買牛奶" {
		t.Errorf("位置參數 = %v，預期 [買牛奶]", got)
	}
	if !r.Bool("all") {
		t.Error("--all 應為 true")
	}
	if got := r.String("due"); got != "2026-09-01" {
		t.Errorf("due = %q，預期 2026-09-01", got)
	}
	if got := r.Strings("tag"); len(got) != 2 || got[0] != "購物" || got[1] != "家務" {
		t.Errorf("tag = %v，預期 [購物 家務]", got)
	}
}

func TestOptionalStringThreeStates(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		changed  bool
		hasValue bool
		value    string
	}{
		{"沒給", []string{"x"}, false, false, ""},
		{"給了但無值", []string{"x", "-p"}, true, false, ""},
		{"無值且後面接別的 flag", []string{"x", "-p", "-a"}, true, false, ""},
		{"空格式給值", []string{"x", "-p", "work"}, true, true, "work"},
		{"等號式給值", []string{"x", "--project=work"}, true, true, "work"},
		{"等號式給空值", []string{"x", "-p="}, true, true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := specs().Parse(c.args)
			if err != nil {
				t.Fatalf("非預期錯誤：%v", err)
			}
			if r.Changed("project") != c.changed {
				t.Errorf("Changed = %v，預期 %v", r.Changed("project"), c.changed)
			}
			v, has := r.Optional("project")
			if has != c.hasValue || v != c.value {
				t.Errorf("Optional = (%q, %v)，預期 (%q, %v)", v, has, c.value, c.hasValue)
			}
		})
	}
}

func TestStringFlagAcceptsEmptyValue(t *testing.T) {
	r, err := specs().Parse([]string{"--due", ""})
	if err != nil {
		t.Fatalf("非預期錯誤：%v", err)
	}
	if !r.Changed("due") || r.String("due") != "" {
		t.Error("--due \"\" 應視為「有給且值為空」")
	}
}

func TestDoubleDashEndsFlags(t *testing.T) {
	r, err := specs().Parse([]string{"--", "-a", "--due"})
	if err != nil {
		t.Fatalf("非預期錯誤：%v", err)
	}
	if got := r.Args(); len(got) != 2 || got[0] != "-a" || got[1] != "--due" {
		t.Errorf("位置參數 = %v，預期 [-a --due]", got)
	}
	if r.Bool("all") {
		t.Error("-- 之後不應再解析 flag")
	}
}

func TestErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"未知長 flag", []string{"--nope"}, "未知的 flag：--nope"},
		{"未知短 flag", []string{"-z"}, "未知的 flag：-z"},
		{"字串 flag 缺值", []string{"--due"}, "flag --due 需要一個值"},
		{"布林 flag 不接受值", []string{"--all=1"}, "flag --all 不接受值"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := specs().Parse(c.args)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v，預期含 %q", err, c.want)
			}
		})
	}
}

func TestUsageListsFlags(t *testing.T) {
	u := specs().Usage()
	for _, want := range []string{"-a, --all", "-d, --due", "含已完成"} {
		if !strings.Contains(u, want) {
			t.Errorf("Usage 缺少 %q，實際：\n%s", want, u)
		}
	}
}
