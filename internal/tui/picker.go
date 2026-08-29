package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"todo.mirumo.net/internal/project"
	"todo.mirumo.net/internal/store"
)

type pickerKind int

const (
	pickProject pickerKind = iota
	pickTag
)

// pickerItem is one row of the menu. The entry with clear set means no filtering.
type pickerItem struct {
	label string
	value string
	clear bool
}

type pickerState struct {
	kind   pickerKind
	items  []pickerItem
	cursor int
}

func projectItems(ps []store.ProjectCount) []pickerItem {
	items := []pickerItem{{label: "All", clear: true}}
	for _, p := range ps {
		label := project.Label(p.Path)
		if label == "" {
			label = "(uncategorized)"
		}
		items = append(items, pickerItem{label: label, value: p.Path})
	}
	return items
}

func tagItems(tags []string) []pickerItem {
	items := []pickerItem{{label: "All", clear: true}}
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
	case "j", "down":
		if m.picker.cursor < len(m.picker.items)-1 {
			m.picker.cursor++
		}
	case "k", "up":
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
			if it.clear {
				m.filter.Tags = nil
			} else {
				m.filter.Tags = []string{it.value}
			}
		}
		m.mode = modeList
		m.cursor = 0
		return m, m.loadCmd()
	}
	return m, nil
}

func (m Model) viewPicker() string {
	title := "Filter by project"
	if m.picker.kind == pickTag {
		title = "Filter by tag"
	}
	var b strings.Builder
	b.WriteString(title + "\n\n")
	for i, it := range m.picker.items {
		marker := "  "
		line := it.label
		if i == m.picker.cursor {
			marker = "▸ "
			line = styleCursor.Render(line)
		}
		b.WriteString(marker + line + "\n")
	}
	b.WriteString("\n" + styleHint.Render("j/k move · enter select · esc cancel"))
	return b.String()
}
