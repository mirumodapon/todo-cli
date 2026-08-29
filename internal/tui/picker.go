package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"todo.mirumo.net/internal/project"
	"todo.mirumo.net/internal/store"
)

type pickerKind int

const (
	pickProject pickerKind = iota
	pickTag
)

// pickerItem is one row of the menu. The entry with clear set means no filtering.
// note is optional trailing detail, such as a project's open task count.
type pickerItem struct {
	label string
	note  string
	value string
	clear bool
}

type pickerState struct {
	kind   pickerKind
	items  []pickerItem
	cursor int
}

func projectItems(ps []store.ProjectCount) []pickerItem {
	items := []pickerItem{{label: "All projects", clear: true}}
	for _, p := range ps {
		label := project.Label(p.Path)
		if label == "" {
			label = "(uncategorized)"
		}
		items = append(items, pickerItem{
			label: label,
			note:  fmt.Sprintf("%d open", p.Open),
			value: p.Path,
		})
	}
	return items
}

func tagItems(tags []string) []pickerItem {
	items := []pickerItem{{label: "All tags", clear: true}}
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
	if m.picker.kind == pickTag {
		title = "Filter by tag"
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
	b.WriteString("\n" + styleHint.Render("j/k move · enter select · esc cancel"))
	return b.String()
}
