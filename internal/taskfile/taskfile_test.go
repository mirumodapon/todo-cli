package taskfile

import (
	"strings"
	"testing"
	"time"

	"todo.mirumo.net/internal/task"
)

func ref() time.Time { return time.Date(2026, 8, 29, 15, 0, 0, 0, time.Local) }

func sample() task.Task {
	due := time.Date(2026, 9, 12, 0, 0, 0, 0, time.Local)
	return task.Task{
		ID: 7, Title: "renew the passport", Project: "/p/home",
		Due: &due, Priority: task.PriHigh, Tags: []string{"admin", "paperwork"},
		Desc:      "form DS-82\ntwo photos",
		CreatedAt: ref(), UpdatedAt: ref(),
	}
}

func TestFormatIsHeadersThenTheDescription(t *testing.T) {
	got := Format(sample())
	head, body, ok := strings.Cut(got, "\n\n")
	if !ok {
		t.Fatalf("a blank line should separate the fields from the description:\n%s", got)
	}
	for _, want := range []string{
		"title: renew the passport", "project: /p/home",
		"tags: admin, paperwork", "due: 2026-09-12", "priority: high",
	} {
		if !strings.Contains(head, want) {
			t.Errorf("the header is missing %q:\n%s", want, head)
		}
	}
	if strings.TrimSpace(body) != "form DS-82\ntwo photos" {
		t.Errorf("body = %q", body)
	}
}

func TestFormatWritesTheTimeOfDayWhenThereIsOne(t *testing.T) {
	tk := sample()
	due := time.Date(2026, 9, 12, 15, 30, 0, 0, time.Local)
	tk.Due, tk.DueHasTime = &due, true
	if !strings.Contains(Format(tk), "due: 2026-09-12 15:30") {
		t.Errorf("a timed due date should keep its time:\n%s", Format(tk))
	}
}

// What Format writes, Parse must read back unchanged.
func TestFormatParseRoundTrips(t *testing.T) {
	in := sample()
	got, err := Parse(Format(in), task.Task{ID: in.ID, CreatedAt: in.CreatedAt}, ref())
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != in.Title || got.Project != in.Project || got.Desc != in.Desc {
		t.Errorf("= %+v", got)
	}
	if got.Priority != in.Priority || len(got.Tags) != 2 {
		t.Errorf("priority = %v, tags = %v", got.Priority, got.Tags)
	}
	if got.Due == nil || !got.Due.Equal(*in.Due) || got.DueHasTime {
		t.Errorf("due = %v, hasTime = %v", got.Due, got.DueHasTime)
	}
}

func TestParseAppliesEdits(t *testing.T) {
	text := `# a comment, ignored
title: a new title
project:
tags: one, two
due: tomorrow
priority: !

the description
over two lines`
	got, err := Parse(text, sample(), ref())
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "a new title" {
		t.Errorf("title = %q", got.Title)
	}
	if got.Project != "" {
		t.Errorf("project = %q, an empty value should clear it", got.Project)
	}
	if strings.Join(got.Tags, ",") != "one,two" {
		t.Errorf("tags = %v", got.Tags)
	}
	if got.Due == nil || got.Due.Format("2006-01-02") != "2026-08-30" {
		t.Errorf("due = %v", got.Due)
	}
	if got.Priority != task.PriLow {
		t.Errorf("priority = %v, the marks should be accepted too", got.Priority)
	}
	if got.Desc != "the description\nover two lines" {
		t.Errorf("desc = %q", got.Desc)
	}
	if got.ID != 7 || !got.CreatedAt.Equal(ref()) {
		t.Errorf("the fields the file does not carry must survive: %+v", got)
	}
}

// Deleting a line is not the same gesture as emptying its value: one is an
// accident waiting to happen, the other is deliberate.
func TestParseLeavesAbsentFieldsAlone(t *testing.T) {
	got, err := Parse("title: still here\n\nbody", sample(), ref())
	if err != nil {
		t.Fatal(err)
	}
	if got.Due == nil || got.Priority != task.PriHigh || got.Project != "/p/home" || len(got.Tags) != 2 {
		t.Errorf("a missing line should leave its field untouched: %+v", got)
	}
}

func TestParseClearsAnEmptyValue(t *testing.T) {
	got, err := Parse("due:\npriority:\ntags:\n\nbody", sample(), ref())
	if err != nil {
		t.Fatal(err)
	}
	if got.Due != nil || got.DueHasTime {
		t.Errorf("due = %v, an empty value should clear it", got.Due)
	}
	if got.Priority != task.PriNone || len(got.Tags) != 0 {
		t.Errorf("priority = %v, tags = %v", got.Priority, got.Tags)
	}
}

func TestParseRejectsWhatItCannotUnderstand(t *testing.T) {
	cases := []struct {
		name, text, want string
	}{
		{"unknown field", "colour: red\n\nbody", "colour"},
		{"not a field", "just some words\n\nbody", "just some words"},
		{"empty title", "title:   \n\nbody", "title"},
		{"bad date", "due: someday\n\nbody", "someday"},
		{"bad priority", "priority: urgent\n\nbody", "urgent"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse(c.text, sample(), ref())
			if err == nil {
				t.Fatalf("%q should not parse", c.text)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, it should name %q", err, c.want)
			}
		})
	}
}

// A description may be anything at all, including lines that look like fields
// or comments; only the header is interpreted.
func TestParseTreatsTheBodyAsText(t *testing.T) {
	got, err := Parse("title: x\n\n# a heading\ntitle: not a field\n\nstill the body", sample(), ref())
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "x" {
		t.Errorf("title = %q", got.Title)
	}
	if got.Desc != "# a heading\ntitle: not a field\n\nstill the body" {
		t.Errorf("desc = %q", got.Desc)
	}
}

func TestParseWithNoBlankLineHasNoDescription(t *testing.T) {
	got, err := Parse("title: x\n", sample(), ref())
	if err != nil {
		t.Fatal(err)
	}
	if got.Desc != "" {
		t.Errorf("desc = %q", got.Desc)
	}
}
