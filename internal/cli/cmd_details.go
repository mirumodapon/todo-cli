package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"todo.mirumo.net/internal/argparse"
	"todo.mirumo.net/internal/datearg"
	"todo.mirumo.net/internal/project"
	"todo.mirumo.net/internal/task"
)

const stampLayout = "2006-01-02 15:04"

func (a *App) cmdDetails(args []string) error {
	ids, err := parseIDs(args)
	if err != nil {
		return err
	}
	for i, id := range ids {
		t, err := a.Store.Get(id)
		if err != nil {
			return fmt.Errorf("#%d: %w", id, err)
		}
		if i > 0 {
			fmt.Fprintln(a.Out)
		}
		a.writeDetails(t)
	}
	return nil
}

// writeDetails prints one task in full. Fields with nothing in them are left
// out entirely: a column of empty labels says nothing and buries what is there.
func (a *App) writeDetails(t task.Task) {
	fmt.Fprintf(a.Out, "#%d  %s\n", t.ID, t.Title)

	status := "open"
	if t.Done() {
		status = "done " + t.DoneAt.Format(stampLayout)
	}
	rows := [][2]string{{"status", status}}
	if t.Due != nil {
		// The full date, not the listing's short form: this view has no columns
		// to keep narrow, and it is where you come to settle what a date is.
		layout := "2006-01-02"
		if t.DueHasTime {
			layout = stampLayout
		}
		rows = append(rows, [2]string{"due", fmt.Sprintf("%s  (%s)",
			t.Due.Format(layout), datearg.Remaining(*t.Due, t.DueHasTime, a.Now()))})
	}
	if t.Priority != task.PriNone {
		rows = append(rows, [2]string{"priority", t.Priority.Marks() + " " + t.Priority.String()})
	}
	if t.Project != "" {
		rows = append(rows, [2]string{"project", project.Label(t.Project) + "  " + t.Project})
	}
	if len(t.Tags) > 0 {
		rows = append(rows, [2]string{"tags", "@" + strings.Join(t.Tags, " @")})
	}
	rows = append(rows, [2]string{"created", t.CreatedAt.Format(stampLayout)})
	if !t.UpdatedAt.Equal(t.CreatedAt) {
		rows = append(rows, [2]string{"updated", t.UpdatedAt.Format(stampLayout)})
	}

	var w int
	for _, r := range rows {
		w = max(w, lipgloss.Width(r[0]))
	}
	for _, r := range rows {
		fmt.Fprintf(a.Out, "  %s  %s\n", pad(r[0], w), r[1])
	}

	if t.Desc == "" {
		return
	}
	fmt.Fprintln(a.Out)
	for _, line := range strings.Split(t.Desc, "\n") {
		if line == "" {
			fmt.Fprintln(a.Out)
			continue
		}
		fmt.Fprintln(a.Out, "  "+line)
	}
}

// descValue reads the three states of --desc: absent leaves current alone, a
// value replaces it (an empty one clears it), and no value at all hands the
// text to the editor. A description is a paragraph, and a paragraph does not
// belong on a command line.
func (a *App) descValue(r *argparse.Result, current string) (string, error) {
	if !r.Changed("desc") {
		return current, nil
	}
	if v, has := r.Optional("desc"); has {
		return v, nil
	}
	return a.editText(current)
}
