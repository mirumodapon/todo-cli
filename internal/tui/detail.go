package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"todo.mirumo.net/internal/datearg"
	"todo.mirumo.net/internal/project"
	"todo.mirumo.net/internal/task"
)

const stampLayout = "2006-01-02 15:04"

// detailRows lists what a task is, one label and value per row. Empty fields are
// left out: a column of blank labels buries the ones that say something. The
// labels and the order match todo details, so the two views read the same.
func (m Model) detailRows(t task.Task) [][2]string {
	status := "open"
	if t.Done() {
		status = "done " + t.DoneAt.Format(stampLayout)
	}
	rows := [][2]string{{"status", status}}
	if t.Due != nil {
		layout := "2006-01-02"
		if t.DueHasTime {
			layout = stampLayout
		}
		rows = append(rows, [2]string{"due", fmt.Sprintf("%s  (%s)",
			t.Due.Format(layout), datearg.Remaining(*t.Due, t.DueHasTime, m.now()))})
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
	return rows
}

func (m Model) viewDetail() string {
	t, ok := m.current()
	if !ok {
		return m.viewList()
	}
	rows := m.detailRows(t)
	var w int
	for _, r := range rows {
		w = max(w, lipgloss.Width(r[0]))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "#%d  %s\n\n", t.ID, t.Title)
	for _, r := range rows {
		b.WriteString("  " + styleDim.Render(pad(r[0], w)) + "  " + r[1] + "\n")
	}
	b.WriteString("\n")
	if t.Desc == "" {
		b.WriteString(styleDim.Render("  No description") + "\n")
		return m.screen(b.String(), styleHint.Render("Press any key to go back"))
	}
	// The description is wrapped to the frame: a long line would otherwise wrap
	// itself and push the rows below it off the bottom of the screen.
	for _, line := range strings.Split(t.Desc, "\n") {
		if line == "" {
			b.WriteString("\n")
			continue
		}
		// Width pads every rendered line out to the full frame, so the padding is
		// trimmed back off: trailing spaces are invisible until something copies them.
		for _, wrapped := range strings.Split(styleDescBody.Width(max(3, m.width)).Render(line), "\n") {
			b.WriteString(strings.TrimRight(wrapped, " ") + "\n")
		}
	}
	return m.screen(b.String(), styleHint.Render("Press any key to go back"))
}
