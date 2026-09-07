package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// A pane is worth having only when there is room for both halves. At 100
// columns the list keeps about sixty and the pane takes forty, which is enough
// for a label column and a sentence. Height is the fallback: a narrow but tall
// terminal has its spare room underneath instead of beside.
const (
	paneMinWidth  = 100
	paneWidth     = 40
	paneMinHeight = 30
	paneHeight    = 10
	paneGap       = " │ "
)

type layout int

const (
	layoutPlain layout = iota
	layoutSide
	layoutStacked
)

// paneLayout decides where the detail goes, if anywhere. Terminal size decides
// it; v turns it off for the ones who would rather have the plain list.
func (m Model) paneLayout() layout {
	if m.paneOff {
		return layoutPlain
	}
	// Nothing under the cursor is nothing to detail, and a pane of nothing is
	// worse than no pane.
	if _, ok := m.current(); !ok {
		return layoutPlain
	}
	switch {
	case m.width >= paneMinWidth:
		return layoutSide
	case m.height >= paneMinHeight:
		return layoutStacked
	}
	return layoutPlain
}

// clip cuts a line to a width rather than letting it wrap. A wrapped line would
// push the row below it off the frame, which is the one thing the layout promises.
func clip(s string, w int) string {
	return lipgloss.NewStyle().MaxWidth(max(0, w)).Render(s)
}

// paneLines renders the task under the cursor into at most h lines of width w.
// It shows what the detail view shows, in the same order, so the two never
// disagree about what a task is.
func (m Model) paneLines(w, h int) []string {
	t, ok := m.current()
	if !ok || h <= 0 || w <= 0 {
		return nil
	}
	var out []string
	add := func(s string) { out = append(out, clip(s, w)) }

	add(styleCursor.Render(fmt.Sprintf("#%d  %s", t.ID, t.Title)))
	add("")
	rows := m.detailRows(t)
	var labelWidth int
	for _, r := range rows {
		labelWidth = max(labelWidth, lipgloss.Width(r[0]))
	}
	for _, r := range rows {
		add(styleDim.Render(pad(r[0], labelWidth)) + "  " + r[1])
	}
	add("")
	if t.Desc == "" {
		add(styleDim.Render("No description"))
	}
	for _, line := range strings.Split(t.Desc, "\n") {
		if t.Desc == "" {
			break
		}
		if line == "" {
			add("")
			continue
		}
		for _, wrapped := range strings.Split(lipgloss.NewStyle().Width(w).Render(line), "\n") {
			add(strings.TrimRight(wrapped, " "))
		}
	}
	if len(out) > h {
		// Say that it was cut rather than ending mid-sentence: enter opens the
		// whole thing.
		out = out[:h]
		out[h-1] = clip(styleDim.Render("… enter for the rest"), w)
	}
	return out
}

// beside puts the list and the pane in two columns, with a rule the full height
// of the body so the split reads as one shape rather than a ragged edge.
func (m Model) beside(rows []string) string {
	leftWidth := m.width - paneWidth - lipgloss.Width(paneGap)
	right := m.paneLines(paneWidth, m.listHeight())
	var b strings.Builder
	for i := range m.listHeight() {
		left, detail := "", ""
		if i < len(rows) {
			left = rows[i]
		}
		if i < len(right) {
			detail = right[i]
		}
		line := pad(clip(left, leftWidth), leftWidth) + styleDim.Render(paneGap) + detail
		b.WriteString(strings.TrimRight(line, " ") + "\n")
	}
	return b.String()
}

// under puts the pane below the list, with the list padded to its full height so
// the rule does not move every time the number of tasks changes.
func (m Model) under(rows []string) string {
	var b strings.Builder
	for i := range m.listHeight() {
		if i < len(rows) {
			b.WriteString(clip(rows[i], m.width) + "\n")
			continue
		}
		b.WriteString("\n")
	}
	b.WriteString(styleDim.Render(strings.Repeat("─", m.width)) + "\n")
	for _, l := range m.paneLines(m.width, paneHeight) {
		b.WriteString(l + "\n")
	}
	return b.String()
}
