package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

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

var fieldLabels = [fieldCount]string{"標題", "專案", "標籤", "截止日", "優先度"}

// formState 是新增／編輯共用的表單。editing 為 false 時代表新增。
type formState struct {
	editing  bool
	original task.Task
	inputs   [fieldCount]textinput.Model
	focus    int
	errText  string
}

// openForm 準備一份表單。編輯時以現有值預填。
func (m Model) openForm(t task.Task, editing bool) Model {
	f := formState{editing: editing, original: t}
	values := [fieldCount]string{
		t.Title,
		t.Project,
		strings.Join(t.Tags, ","),
		"",
		t.Priority.String(),
	}
	if t.Due != nil {
		values[fieldDue] = t.Due.Format("2006-01-02")
	}
	placeholders := [fieldCount]string{
		"要做什麼",
		"留空 = 全域未分類（ctrl+p 填入當前目錄）",
		"逗號分隔",
		"tomorrow、fri、+3d、2026-09-01",
		"low、med、high",
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
	// 同 updateSearch：不轉傳游標閃爍的計時器 cmd。
	m.form.inputs[m.form.focus], _ = m.form.inputs[m.form.focus].Update(msg)
	m.form.errText = ""
	return m, nil
}

// formTask 把表單內容組成一筆 Task，任何欄位不合法就回傳錯誤。
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
		t.Due = nil
	} else {
		d, err := datearg.Parse(v, now)
		if err != nil {
			return task.Task{}, err
		}
		t.Due = &d
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
	title := "新增待辦"
	if m.form.editing {
		title = "編輯 #" + itoa(m.form.original.ID)
	}
	var b strings.Builder
	b.WriteString(title + "\n\n")
	for i, in := range m.form.inputs {
		marker := "  "
		if i == m.form.focus {
			marker = "▸ "
		}
		b.WriteString(marker + pad(fieldLabels[i], 8) + in.View() + "\n")
	}
	b.WriteString("\n")
	if m.form.errText != "" {
		b.WriteString(styleErr.Render(m.form.errText) + "\n")
	}
	b.WriteString(styleHint.Render("tab next field · ctrl+p fill current directory · enter save · esc cancel"))
	return b.String()
}
