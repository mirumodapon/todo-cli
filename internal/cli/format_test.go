package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"todo.mirumo.net/internal/task"
)

func day(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	return &t
}

func TestPadUsesDisplayWidth(t *testing.T) {
	// The Chinese title is 3 runes, 9 bytes, and 6 display cells wide.
	if got := pad("買牛奶", 8); got != "買牛奶  " {
		t.Errorf("= %q，預期補到 8 格寬（兩個空白）", got)
	}
	if got := pad("abc", 2); got != "abc" {
		t.Errorf("= %q，超過寬度時應原樣回傳", got)
	}
}

func TestWriteListAlignsColumns(t *testing.T) {
	now := time.Date(2026, 8, 29, 15, 0, 0, 0, time.Local)
	ts := []task.Task{
		{ID: 1, Title: "買牛奶", Due: day(2026, 8, 29), Priority: task.PriHigh, Tags: []string{"購物"}},
		{ID: 12, Title: "繳房租", Project: "/p/home"},
	}
	var buf bytes.Buffer
	WriteList(&buf, ts, now, false)
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("預期兩行，實得 %d 行：%q", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[0], "1  [ ] !高 今天 買牛奶 ") {
		t.Errorf("第一行 = %q", lines[0])
	}
	if !strings.Contains(lines[0], "@購物") {
		t.Errorf("第一行少了標籤：%q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "12 [ ] ") {
		t.Errorf("第二行 = %q，id 欄應對齊到最寬的 id", lines[1])
	}
	if !strings.HasSuffix(lines[1], "home") {
		t.Errorf("第二行應以專案 basename 收尾：%q", lines[1])
	}
	for _, l := range lines {
		if strings.HasSuffix(l, " ") {
			t.Errorf("不該有尾隨空白：%q", l)
		}
		if strings.Contains(l, "\x1b[") {
			t.Errorf("color=false 時不該輸出 ANSI 碼：%q", l)
		}
	}
}

func TestWriteListEmpty(t *testing.T) {
	var buf bytes.Buffer
	WriteList(&buf, nil, time.Now(), false)
	if !strings.Contains(buf.String(), "沒有符合的待辦") {
		t.Errorf("= %q", buf.String())
	}
}

func TestWriteListColorMarksOverdueAndDone(t *testing.T) {
	now := time.Date(2026, 8, 29, 15, 0, 0, 0, time.Local)
	done := now
	ts := []task.Task{
		{ID: 1, Title: "逾期", Due: day(2026, 8, 1)},
		{ID: 2, Title: "完成", DoneAt: &done},
	}
	var buf bytes.Buffer
	WriteList(&buf, ts, now, true)
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if !strings.HasPrefix(lines[0], "\x1b[31m") {
		t.Errorf("逾期該是紅色：%q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "\x1b[2m") {
		t.Errorf("已完成該是暗色：%q", lines[1])
	}
}
