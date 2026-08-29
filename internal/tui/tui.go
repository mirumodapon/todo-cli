// Package tui provides the interactive interface.
package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"todo.mirumo.net/internal/store"
	"todo.mirumo.net/internal/task"
)

type mode int

const (
	modeList mode = iota
	modeSearch
	modePicker
	modeForm
	modeHelp
	modeConfirm
)

// Model is the root model. Every substate hangs off it and Update dispatches on mode.
type Model struct {
	store store.Store
	now   func() time.Time
	cwd   string

	mode   mode
	tasks  []task.Task
	cursor int
	filter task.Filter

	search  textinput.Model
	picker  pickerState
	form    formState
	confirm confirmState
	// undo keeps a single level: the last deleted item, discarded when the TUI exits.
	undo *task.Task

	status        string
	err           error
	width, height int
}

// defaultFilter is what the list starts on and what esc returns to:
// uncategorized tasks only, matching what a plain "todo ls" prints.
// Project tasks are one P away.
func defaultFilter() task.Filter {
	uncategorized := ""
	return task.Filter{Project: &uncategorized}
}

// New builds a Model.
func New(s store.Store, now func() time.Time, cwd string) Model {
	ti := textinput.New()
	ti.Prompt = "/"
	ti.Placeholder = "search titles"
	return Model{
		store: s, now: now, cwd: cwd,
		mode: modeList, search: ti, filter: defaultFilter(),
		width: 80, height: 24,
	}
}

// Run starts the interactive interface.
func Run(s store.Store, now func() time.Time, cwd string) error {
	_, err := tea.NewProgram(New(s, now, cwd), tea.WithAltScreen()).Run()
	return err
}

func (m Model) Init() tea.Cmd { return m.loadCmd() }

// current returns the item under the cursor.
func (m Model) current() (task.Task, bool) {
	if m.cursor < 0 || m.cursor >= len(m.tasks) {
		return task.Task{}, false
	}
	return m.tasks[m.cursor], true
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tasksMsg:
		m.tasks = []task.Task(msg)
		m.err = nil
		if m.cursor >= len(m.tasks) {
			m.cursor = max(0, len(m.tasks)-1)
		}
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil

	case savedMsg:
		m.status, m.err = msg.note, nil
		return m, m.loadCmd()

	case deletedMsg:
		t := msg.t
		m.undo = &t
		m.status = `deleted "` + t.Title + `" · u to undo`
		m.err = nil
		return m, m.loadCmd()

	case projectsMsg:
		m.picker = pickerState{kind: pickProject, items: projectItems(msg)}
		m.mode = modePicker
		return m, nil

	case tagsMsg:
		m.picker = pickerState{kind: pickTag, items: tagItems(msg)}
		m.mode = modePicker
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeList:
			return m.updateList(msg)
		case modeSearch:
			return m.updateSearch(msg)
		case modePicker:
			return m.updatePicker(msg)
		case modeForm:
			return m.updateForm(msg)
		case modeHelp:
			m.mode = modeList
			return m, nil
		case modeConfirm:
			return m.updateConfirm(msg)
		}
	}
	return m, nil
}

func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		if m.cursor < len(m.tasks)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "g":
		m.cursor = 0
	case "G":
		m.cursor = max(0, len(m.tasks)-1)
	case " ":
		if t, ok := m.current(); ok {
			// Un-completing needs no confirmation: completing is already gated,
			// so an accidental completion cannot happen, and reversing one is the
			// recovery move rather than a destructive act.
			if t.Done() {
				return m, m.toggleCmd(t)
			}
			return m.askConfirm(`Mark "`+t.Title+`" done? (y/n)`, m.toggleCmd(t)), nil
		}
	case "a":
		return m.openForm(task.Task{}, false), nil
	case "e":
		if t, ok := m.current(); ok {
			return m.openForm(t, true), nil
		}
	case "?":
		m.mode = modeHelp
		return m, nil
	case "d":
		if t, ok := m.current(); ok {
			return m.askConfirm(`Delete "`+t.Title+`"? (y/n)`, m.deleteCmd(t)), nil
		}
	case "u":
		if m.undo == nil {
			m.status = "nothing to undo"
			return m, nil
		}
		t := *m.undo
		m.undo = nil
		return m, m.restoreCmd(t)
	case "/":
		m.mode = modeSearch
		m.search.SetValue(m.filter.Search)
		m.search.CursorEnd()
		m.search.Focus()
		return m, nil
	case "P":
		return m, m.projectsCmd()
	case "T":
		return m, m.tagsCmd()
	case "A":
		m.filter.IncludeDone = !m.filter.IncludeDone
		return m, m.loadCmd()
	case "s":
		m.filter.Sort = (m.filter.Sort + 1) % 3
		m.status = "sort: " + sortLabel(m.filter.Sort)
		return m, m.loadCmd()
	case "esc":
		m.filter = defaultFilter()
		m.search.SetValue("")
		m.status = ""
		return m, m.loadCmd()
	}
	return m, nil
}

// updateSearch drives incremental search, requerying on every keystroke. The list is small,
// so the cost is negligible and the view never drifts from the database.
func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.mode = modeList
		m.search.Blur()
		return m, nil
	case "esc":
		m.mode = modeList
		m.search.Blur()
		m.search.SetValue("")
		m.filter.Search = ""
		return m, m.loadCmd()
	}
	// The cmd textinput returns is dropped on purpose: it is the cursor blink timer.
	// Forwarding it would make Update tests wait on a timer, and blinking is decoration.
	m.search, _ = m.search.Update(msg)
	m.filter.Search = m.search.Value()
	m.cursor = 0
	return m, m.loadCmd()
}

func sortLabel(s task.SortBy) string {
	switch s {
	case task.SortPriority:
		return "priority"
	case task.SortCreated:
		return "created"
	}
	return "due date"
}

func (m Model) View() string {
	switch m.mode {
	case modePicker:
		return m.viewPicker()
	case modeForm:
		return m.viewForm()
	case modeHelp:
		return viewHelp()
	}
	return m.viewList()
}
