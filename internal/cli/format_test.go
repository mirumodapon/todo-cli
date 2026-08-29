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
		t.Errorf("= %q, 預期補到 8 格寬（兩個空白）", got)
	}
	if got := pad("abc", 2); got != "abc" {
		t.Errorf("= %q, 超過寬度時應原樣回傳", got)
	}
}

func TestWriteListAlignsColumns(t *testing.T) {
	now := time.Date(2026, 8, 29, 15, 0, 0, 0, time.Local)
	ts := []task.Task{
		{ID: 1, Title: "買牛奶", Due: day(2026, 8, 29), Priority: task.PriHigh, Tags: []string{"shopping"}},
		{ID: 12, Title: "繳房租", Project: "/p/home"},
	}
	var buf bytes.Buffer
	WriteList(&buf, ts, now, false)
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want two lines, got %d: %q", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[0], "1  [ ] !high today 買牛奶 ") {
		t.Errorf("first line = %q", lines[0])
	}
	if !strings.Contains(lines[0], "@shopping") {
		t.Errorf("the first line is missing its tags: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "12 [ ] ") {
		t.Errorf("second line = %q, the id column should align to the widest id", lines[1])
	}
	if !strings.HasSuffix(lines[1], "home") {
		t.Errorf("the second line should end with the project basename: %q", lines[1])
	}
	for _, l := range lines {
		if strings.HasSuffix(l, " ") {
			t.Errorf("no trailing whitespace allowed: %q", l)
		}
		if strings.Contains(l, "\x1b[") {
			t.Errorf("color=false must emit no ANSI codes: %q", l)
		}
	}
}

func TestWriteListEmpty(t *testing.T) {
	var buf bytes.Buffer
	WriteList(&buf, nil, time.Now(), false)
	if !strings.Contains(buf.String(), "No matching tasks") {
		t.Errorf("= %q", buf.String())
	}
}

func TestWriteListColorMarksOverdueAndDone(t *testing.T) {
	now := time.Date(2026, 8, 29, 15, 0, 0, 0, time.Local)
	done := now
	ts := []task.Task{
		{ID: 1, Title: "overdue", Due: day(2026, 8, 1)},
		{ID: 2, Title: "done", DoneAt: &done},
	}
	var buf bytes.Buffer
	WriteList(&buf, ts, now, true)
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if !strings.HasPrefix(lines[0], "\x1b[31m") {
		t.Errorf("an overdue task should be red: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "\x1b[2m") {
		t.Errorf("a done task should be dimmed: %q", lines[1])
	}
}
