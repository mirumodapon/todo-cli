package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// sized returns a model on a terminal of the given size, with a description on
// the first task so the pane has something to show.
func sized(t *testing.T, w, h int) Model {
	t.Helper()
	m, _ := newModel(t)
	m = withDesc(t, m, "semi-skimmed")
	m, _ = send(t, m, tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

// lineWith returns the first line containing every one of want.
func lineWith(v string, want ...string) string {
	for _, l := range strings.Split(v, "\n") {
		ok := true
		for _, w := range want {
			if !strings.Contains(l, w) {
				ok = false
				break
			}
		}
		if ok {
			return l
		}
	}
	return ""
}

// A wide terminal has room to the side, which is where the space actually is:
// task lines are short and the screen is not.
func TestWideTerminalPutsTheDetailBesideTheList(t *testing.T) {
	v := sized(t, 120, 24).View()
	if lineWith(v, cursorMarker, "#1") == "" {
		t.Fatalf("the list row and the detail should share a line:\n%s", v)
	}
	if !strings.Contains(v, "status") {
		t.Errorf("the pane should carry the detail rows:\n%s", v)
	}
}

// Narrow but tall: the room is underneath.
func TestTallNarrowTerminalStacksTheDetailUnderTheList(t *testing.T) {
	v := sized(t, 80, 40).View()
	if !strings.Contains(v, "status") {
		t.Fatalf("the pane should be showing:\n%s", v)
	}
	if lineWith(v, cursorMarker, "#1") != "" {
		t.Errorf("stacked, nothing shares a line with the list:\n%s", v)
	}
	rows := strings.Split(v, "\n")
	var lastTask, firstDetail int
	for i, l := range rows {
		if strings.Contains(l, "third") {
			lastTask = i
		}
		if strings.Contains(l, "status") && firstDetail == 0 {
			firstDetail = i
		}
	}
	if firstDetail < lastTask {
		t.Errorf("the pane should sit below the list, not above it:\n%s", v)
	}
}

func TestSmallTerminalShowsNoPane(t *testing.T) {
	v := sized(t, 80, 24).View()
	if strings.Contains(v, "status") {
		t.Errorf("there is no room for a pane on this terminal:\n%s", v)
	}
	if !strings.Contains(v, "third") {
		t.Errorf("the list should still be whole:\n%s", v)
	}
}

func TestVTogglesThePane(t *testing.T) {
	m := sized(t, 120, 24)
	if !strings.Contains(m.View(), "status") {
		t.Fatal("the pane should start open on a terminal this wide")
	}
	m = press(t, m, "v")
	if strings.Contains(m.View(), "status") {
		t.Errorf("v should close the pane:\n%s", m.View())
	}
	if !strings.Contains(m.View(), "third") {
		t.Errorf("and give the whole width back to the list:\n%s", m.View())
	}
	m = press(t, m, "v")
	if !strings.Contains(m.View(), "status") {
		t.Error("v should open it again")
	}
}

func TestThePaneFollowsTheCursor(t *testing.T) {
	m := sized(t, 120, 24)
	if !strings.Contains(m.View(), "semi-skimmed") {
		t.Fatalf("the pane should show the task under the cursor:\n%s", m.View())
	}
	m = press(t, m, "j")
	v := m.View()
	if lineWith(v, "#2") == "" {
		t.Errorf("moving should move the pane with it:\n%s", v)
	}
	if strings.Contains(v, "semi-skimmed") {
		t.Errorf("and stop showing the one it left:\n%s", v)
	}
}

// Whatever the layout, the frame holds: the hint stays on the last row and
// nothing runs past the right edge.
func TestThePaneKeepsTheFrame(t *testing.T) {
	for _, size := range [][2]int{{120, 24}, {80, 40}, {80, 24}, {100, 30}} {
		m := sized(t, size[0], size[1])
		v := m.View()
		rows := strings.Split(v, "\n")
		if len(rows) != size[1] {
			t.Errorf("%dx%d: the screen is %d rows:\n%s", size[0], size[1], len(rows), v)
		}
		for _, l := range rows {
			if lipgloss.Width(l) > size[0] {
				t.Errorf("%dx%d: a line is %d columns wide: %q", size[0], size[1], lipgloss.Width(l), l)
			}
			if strings.HasSuffix(l, " ") {
				t.Errorf("%dx%d: trailing whitespace: %q", size[0], size[1], l)
			}
		}
		if !strings.Contains(rows[len(rows)-1], "q quit") {
			t.Errorf("%dx%d: the hint should be on the last row, got %q", size[0], size[1], rows[len(rows)-1])
		}
	}
}

// An empty list has nothing to show beside it, and a pane of nothing is worse
// than no pane at all.
func TestNoPaneWhenThereIsNothingUnderTheCursor(t *testing.T) {
	m := sized(t, 120, 24)
	m = press(t, m, "/")
	m = press(t, m, "zzzz")
	v := m.View()
	if strings.Contains(v, "status") {
		t.Errorf("nothing is selected, so there is nothing to detail:\n%s", v)
	}
}
