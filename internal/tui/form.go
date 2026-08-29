package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"todo.mirumo.net/internal/datearg"
	"todo.mirumo.net/internal/project"
	"todo.mirumo.net/internal/task"
)

const (
	fieldTitle = iota
	fieldProject
	fieldTags
	fieldDue
	fieldPri
	fieldCount
)

var fieldLabels = [fieldCount]string{"Title", "Project", "Tags", "Due", "Priority"}

// formState is the form shared by add and edit. editing false means add.
type formState struct {
	editing  bool
	original task.Task
	inputs   [fieldCount]textinput.Model
	focus    int
	errText  string
}

// openForm prepares a form, prefilling current values when editing.
func (m Model) openForm(t task.Task, editing bool) Model {
	f := formState{editing: editing, original: t}
	values := [fieldCount]string{
		t.Title,
		t.Project,
		strings.Join(t.Tags, ","),
		"",
		t.Priority.Marks(),
	}
	if t.Due != nil {
		layout := "2006-01-02"
		if t.DueHasTime {
			layout = "2006-01-02 15:04"
		}
		values[fieldDue] = t.Due.Format(layout)
	}
	placeholders := [fieldCount]string{
		"what needs doing",
		"empty = uncategorized (ctrl+p fills the current directory)",
		"comma separated",
		"tomorrow, fri, +3d, 2026-09-01, today 15:00",
		"low, med, high or !, !!, !!!",
	}
	for i := range f.inputs {
		ti := textinput.New()
		ti.Prompt = ""
		ti.SetValue(values[i])
		ti.Placeholder = placeholders[i]
		ti.CursorEnd()
		f.inputs[i] = ti
	}
	f.inputs[fieldTitle].Focus()
	m.form = f
	m.mode = modeForm
	return m
}

func (m Model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		return m, nil
	case "tab", "down":
		m.form.inputs[m.form.focus].Blur()
		m.form.focus = (m.form.focus + 1) % fieldCount
		m.form.inputs[m.form.focus].Focus()
		return m, nil
	case "shift+tab", "up":
		m.form.inputs[m.form.focus].Blur()
		m.form.focus = (m.form.focus - 1 + fieldCount) % fieldCount
		m.form.inputs[m.form.focus].Focus()
		return m, nil
	case "ctrl+p":
		p, err := project.Current(m.cwd)
		if err != nil {
			m.form.errText = err.Error()
			return m, nil
		}
		m.form.inputs[fieldProject].SetValue(p)
		m.form.inputs[fieldProject].CursorEnd()
		return m, nil
	case "enter":
		t, err := m.formTask()
		if err != nil {
			m.form.errText = err.Error()
			return m, nil
		}
		m.mode = modeList
		return m, m.saveCmd(t, m.form.editing)
	}
	// As in updateSearch, the cursor blink timer cmd is not forwarded.
	m.form.inputs[m.form.focus], _ = m.form.inputs[m.form.focus].Update(msg)
	m.form.errText = ""
	return m, nil
}

// formTask assembles a Task from the form, returning an error if any field is invalid.
func (m Model) formTask() (task.Task, error) {
	f := m.form
	t := f.original
	now := m.now()

	title, err := task.ValidateTitle(f.inputs[fieldTitle].Value())
	if err != nil {
		return task.Task{}, err
	}
	t.Title = title
	t.Project = strings.TrimSpace(f.inputs[fieldProject].Value())
	t.Tags = task.NormalizeTags(strings.Split(f.inputs[fieldTags].Value(), ","))

	if v := strings.TrimSpace(f.inputs[fieldDue].Value()); v == "" {
		t.Due, t.DueHasTime = nil, false
	} else {
		d, hasTime, err := datearg.Parse(v, now)
		if err != nil {
			return task.Task{}, err
		}
		t.Due, t.DueHasTime = &d, hasTime
	}
	if t.Priority, err = task.ParsePriority(f.inputs[fieldPri].Value()); err != nil {
		return task.Task{}, err
	}

	t.UpdatedAt = now
	if !f.editing {
		t.CreatedAt = now
	}
	return t, nil
}

func (m Model) viewForm() string {
	title := "New task"
	if m.form.editing {
		title = "Edit #" + itoa(m.form.original.ID)
	}
	// Derive the label column from the labels themselves; a hard-coded width
	// silently runs the longest label into its input.
	var labelWidth int
	for _, l := range fieldLabels {
		labelWidth = max(labelWidth, lipgloss.Width(l))
	}
	labelWidth++ // at least one space before the input

	var b strings.Builder
	b.WriteString(title + "\n\n")
	for i, in := range m.form.inputs {
		marker := pad("", markerWidth)
		if i == m.form.focus {
			marker = pad(cursorMarker, markerWidth)
		}
		b.WriteString(marker + pad(fieldLabels[i], labelWidth) + in.View() + "\n")
	}
	b.WriteString("\n")
	if m.form.errText != "" {
		b.WriteString(styleErr.Render(m.form.errText) + "\n")
	}
	b.WriteString(styleHint.Render("tab next field · ctrl+p fill current directory · enter save · esc cancel"))
	return b.String()
}
