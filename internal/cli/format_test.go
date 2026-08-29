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
	WriteList(&buf, ts, ListOptions{Now: now, Dates: true})
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want two lines, got %d: %q", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[0], "1  [ ] !!! today 買牛奶 ") {
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

func TestWriteListShowsRemainingTimeByDefault(t *testing.T) {
	now := time.Date(2026, 8, 29, 15, 0, 0, 0, time.Local)
	soon := now.Add(90 * time.Minute)
	ts := []task.Task{
		{ID: 1, Title: "soon", Due: &soon, DueHasTime: true},
		{ID: 2, Title: "later", Due: day(2026, 9, 8)},
	}

	var buf bytes.Buffer
	WriteList(&buf, ts, ListOptions{Now: now})
	got := buf.String()
	if !strings.Contains(got, "1h") || !strings.Contains(got, "10d") {
		t.Errorf("the default should show time remaining: %q", got)
	}

	buf.Reset()
	WriteList(&buf, ts, ListOptions{Now: now, Dates: true})
	got = buf.String()
	if !strings.Contains(got, "09-08") {
		t.Errorf("Dates should show the calendar date: %q", got)
	}
	if strings.Contains(got, "10d") {
		t.Errorf("Dates should not also show the remaining time: %q", got)
	}
}

func TestWriteListEmpty(t *testing.T) {
	var buf bytes.Buffer
	WriteList(&buf, nil, ListOptions{Now: time.Now()})
	if !strings.Contains(buf.String(), "No matching tasks") {
		t.Errorf("= %q", buf.String())
	}
}

func TestWriteListColorsByUrgency(t *testing.T) {
	now := time.Date(2026, 8, 29, 15, 0, 0, 0, time.Local)
	done := now
	soon := time.Date(2026, 8, 30, 3, 0, 0, 0, time.Local) // twelve hours out
	ramp := time.Date(2026, 8, 31, 9, 0, 0, 0, time.Local) // 42 hours out, mid ramp
	ts := []task.Task{
		{ID: 1, Title: "overdue", Due: day(2026, 8, 1)},
		{ID: 2, Title: "soon", Due: &soon, DueHasTime: true},
		{ID: 3, Title: "mid ramp", Due: &ramp, DueHasTime: true},
		{ID: 4, Title: "far off", Due: day(2026, 12, 25)},
		{ID: 5, Title: "no due date"},
		{ID: 6, Title: "done", DoneAt: &done},
	}
	var buf bytes.Buffer
	WriteList(&buf, ts, ListOptions{Now: now, Color: true, Dates: true})
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")

	const macchiatoRed = "\x1b[38;2;237;135;150m"
	if !strings.HasPrefix(lines[0], macchiatoRed) {
		t.Errorf("an overdue task should be fully red: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], macchiatoRed) {
		t.Errorf("twelve hours out should be fully red: %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "\x1b[38;2;") || strings.HasPrefix(lines[2], macchiatoRed) {
		t.Errorf("mid ramp should be coloured but not red: %q", lines[2])
	}
	for _, i := range []int{3, 4} {
		if strings.Contains(lines[i], "\x1b[") {
			t.Errorf("nothing far off or undated should be coloured: %q", lines[i])
		}
	}
	if !strings.HasPrefix(lines[5], "\x1b[2m") {
		t.Errorf("a done task should be dimmed: %q", lines[5])
	}
}
