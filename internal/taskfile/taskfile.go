// Package taskfile renders a task as the text an editor shows and reads the
// result back. The shape is a mail header: fields, a blank line, then the
// description, which means the description can contain anything at all —
// including lines that look like fields — without an escaping rule.
package taskfile

import (
	"fmt"
	"strings"
	"time"

	"todo.mirumo.net/internal/datearg"
	"todo.mirumo.net/internal/task"
)

const (
	dateLayout     = "2006-01-02"
	dateTimeLayout = "2006-01-02 15:04"
)

// header is the guidance at the top of the file. It sits in the field region,
// where # marks a comment; the description below the blank line has no such
// rule, so a description may start with #.
const header = `# Fields above the blank line, description below it. # starts a comment.
# Removing a line leaves that field alone; emptying its value clears it.
`

// Format renders t for editing.
func Format(t task.Task) string {
	due := ""
	if t.Due != nil {
		layout := dateLayout
		if t.DueHasTime {
			layout = dateTimeLayout
		}
		due = t.Due.Format(layout)
	}
	var b strings.Builder
	b.WriteString(header)
	fmt.Fprintf(&b, "title: %s\n", t.Title)
	fmt.Fprintf(&b, "project: %s\n", t.Project)
	fmt.Fprintf(&b, "tags: %s\n", strings.Join(t.Tags, ", "))
	fmt.Fprintf(&b, "due: %s\n", due)
	fmt.Fprintf(&b, "priority: %s\n", t.Priority.String())
	b.WriteString("\n")
	b.WriteString(t.Desc)
	b.WriteString("\n")
	return b.String()
}

// Parse applies an edited file to base, so everything the file does not carry —
// the id, the timestamps, whether the task is done — survives the round trip.
func Parse(text string, base task.Task, now time.Time) (task.Task, error) {
	head, body, _ := strings.Cut(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n")
	t := base
	for _, line := range strings.Split(head, "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return task.Task{}, fmt.Errorf("not a field: %q (a field reads \"name: value\")", strings.TrimSpace(line))
		}
		if err := assign(&t, strings.ToLower(strings.TrimSpace(key)), strings.TrimSpace(value), now); err != nil {
			return task.Task{}, err
		}
	}
	t.Desc = strings.TrimRight(body, " \t\n")
	return t, nil
}

func assign(t *task.Task, key, value string, now time.Time) error {
	switch key {
	case "title":
		title, err := task.ValidateTitle(value)
		if err != nil {
			return fmt.Errorf("title: %w", err)
		}
		t.Title = title
	case "project":
		t.Project = value
	case "tags":
		t.Tags = task.NormalizeTags(strings.Split(value, ","))
	case "due":
		if value == "" {
			t.Due, t.DueHasTime = nil, false
			return nil
		}
		d, hasTime, err := datearg.Parse(value, now)
		if err != nil {
			return err
		}
		t.Due, t.DueHasTime = &d, hasTime
	case "priority":
		p, err := task.ParsePriority(value)
		if err != nil {
			return err
		}
		t.Priority = p
	default:
		return fmt.Errorf("unknown field %q", key)
	}
	return nil
}
