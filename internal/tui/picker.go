package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"todo.mirumo.net/internal/project"
	"todo.mirumo.net/internal/store"
	"todo.mirumo.net/internal/task"
)

type pickerKind int

const (
	pickProject pickerKind = iota
	pickTag
	pickSort
)

// pickerItem is one row of the menu. The entry with clear set means no
// filtering, and the one with none set means the absence of a value: no
// project, no tags. note is optional trailing detail, such as a project's
// open task count.
type pickerItem struct {
	label string
	note  string
	value string
	clear bool
	none  bool
}

type pickerState struct {
	kind   pickerKind
	items  []pickerItem
	cursor int
}

func projectItems(ps []store.ProjectCount) []pickerItem {
	items := []pickerItem{{label: "All projects", clear: true}}
	// Projects only reports paths some task already uses, but uncategorized is
	// where the list starts and must stay reachable even when it is empty.
	// The query orders by path, and the empty one sorts first.
	if !slices.ContainsFunc(ps, func(p store.ProjectCount) bool { return p.Path == "" }) {
		ps = append([]store.ProjectCount{{}}, ps...)
	}
	for _, p := range ps {
		label := project.Label(p.Path)
		if label == "" {
			label = "(uncategorized)"
		}
		items = append(items, pickerItem{
			label: label,
			note:  fmt.Sprintf("%d open", p.Open),
			value: p.Path,
			none:  p.Path == "",
		})
	}
	return items
}

// sortItems lists the orderings. The value is the word the command line takes,
// so the menu and -s cannot come to mean different things.
func sortItems(current task.Filter) []pickerItem {
	items := make([]pickerItem, 0, task.SortCount)
	for _, s := range []task.SortBy{task.SortID, task.SortDue, task.SortPriority, task.SortCreated} {
		it := pickerItem{label: sortLabel(s), value: sortWord(s)}
		if s == current.Sort {
			it.note = "current"
		}
		items = append(items, it)
	}
	return items
}

func tagItems(tags []string) []pickerItem {
	// A tag can never name the tasks that have none, so untagged is its own
	// entry rather than something the list of tags could contain.
	items := []pickerItem{
		{label: "All tags", clear: true},
		{label: "(untagged)", none: true},
	}
	for _, t := range tags {
		items = append(items, pickerItem{label: "@" + t, value: t})
	}
	return items
}

// updatePicker handles keys in menu mode.
func (m Model) updatePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.mode = modeList
		return m, nil
	case "j", "down", "ctrl+n":
		if m.picker.cursor < len(m.picker.items)-1 {
			m.picker.cursor++
		}
	case "k", "up", "ctrl+p":
		if m.picker.cursor > 0 {
			m.picker.cursor--
		}
	case "enter":
		if m.picker.cursor >= len(m.picker.items) {
			m.mode = modeList
			return m, nil
		}
		it := m.picker.items[m.picker.cursor]
		switch m.picker.kind {
		case pickProject:
			if it.clear {
				m.filter.Project = nil
			} else {
				v := it.value
				m.filter.Project = &v
			}
		case pickTag:
			switch {
			case it.clear:
				m.filter.Tags, m.filter.Untagged = nil, false
			case it.none:
				m.filter.Tags, m.filter.Untagged = nil, true
			default:
				m.filter.Tags, m.filter.Untagged = []string{it.value}, false
			}
		case pickSort:
			s, err := task.ParseSortBy(it.value)
			if err != nil {
				m.err = err
				m.mode = modeList
				return m, nil
			}
			m.filter.Sort = s
			m.mode = modeList
			// The same tasks in a different order, so the cursor keeps its task
			// rather than starting again at the top.
			return m, m.reloadCmd()
		}
		m.mode = modeList
		m.cursor = 0
		return m, m.loadCmd()
	}
	return m, nil
}

// pickerMarker uses the same arrow as the task list, so selection looks the
// same everywhere in the interface.
func (m Model) pickerMarker(i int) string {
	if i == m.picker.cursor {
		return pad(cursorMarker, markerWidth)
	}
	return pad("", markerWidth)
}

func (m Model) viewPicker() string {
	title := "Filter by project"
	switch m.picker.kind {
	case pickTag:
		title = "Filter by tag"
	case pickSort:
		title = "Sort by"
	}
	var w int
	for _, it := range m.picker.items {
		if it.note != "" {
			w = max(w, lipgloss.Width(it.label))
		}
	}
	var b strings.Builder
	b.WriteString(title + "\n\n")
	for i, it := range m.picker.items {
		line := it.label
		if it.note != "" {
			line = pad(line, w) + "  " + styleDim.Render(it.note)
		}
		if i == m.picker.cursor {
			line = styleCursor.Render(line)
		}
		b.WriteString(m.pickerMarker(i) + line + "\n")
	}
	return m.screen(b.String(), m.hint(styleHint.Render("j/k or ctrl+n/p move · enter select · esc cancel")))
}
