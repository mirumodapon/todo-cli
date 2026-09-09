package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func helpOn(t *testing.T, w, h int) Model {
	t.Helper()
	m, _ := newModel(t)
	m, _ = send(t, m, tea.WindowSizeMsg{Width: w, Height: h})
	return press(t, m, "?")
}

// The last row of the help is the one most likely to be cut, and the one that
// says how to leave.
const lastHelpRow = "ctrl+c / ctrl+d"

func TestTheHelpFitsATallTerminal(t *testing.T) {
	m := helpOn(t, 80, 40)
	v := m.View()
	for _, r := range helpRows {
		if !strings.Contains(v, r[0]) {
			t.Errorf("the help is missing %q:\n%s", r[0], v)
		}
	}
	if strings.Contains(v, "more") {
		t.Errorf("nothing is cut, so nothing should say so:\n%s", v)
	}
}

// A short terminal cannot show every key at once, so it says so rather than
// quietly dropping the ones that did not fit.
func TestAShortTerminalSaysThereIsMore(t *testing.T) {
	m := helpOn(t, 80, 14)
	v := m.View()
	if strings.Contains(v, lastHelpRow) {
		t.Fatalf("this terminal cannot fit it all:\n%s", v)
	}
	if !strings.Contains(v, "more") {
		t.Errorf("it should say that there is more:\n%s", v)
	}
	if len(strings.Split(v, "\n")) != 14 {
		t.Errorf("the frame should still hold:\n%s", v)
	}
}

func TestScrollingReachesTheLastKey(t *testing.T) {
	m := helpOn(t, 80, 14)
	for range len(helpRows) {
		m = press(t, m, "j")
	}
	if m.mode != modeHelp {
		t.Fatalf("j should scroll rather than close, mode = %v", m.mode)
	}
	if !strings.Contains(m.View(), lastHelpRow) {
		t.Errorf("scrolling should reach the end:\n%s", m.View())
	}
	// And stop there.
	if !strings.Contains(m.View(), helpRows[len(helpRows)-1][1]) {
		t.Errorf("the bottom row should stay in view:\n%s", m.View())
	}
	for range len(helpRows) {
		m = press(t, m, "k")
	}
	if !strings.Contains(m.View(), helpRows[0][1]) {
		t.Errorf("k should scroll back to the top:\n%s", m.View())
	}
}

func TestAnyOtherKeyStillCloses(t *testing.T) {
	for _, k := range []string{"esc", "?", "q", "x"} {
		m := helpOn(t, 80, 14)
		m = press(t, m, k)
		if m.mode != modeList {
			t.Errorf("%q should close the help, mode = %v", k, m.mode)
		}
	}
}

// Opening it again starts at the top: where you left off scrolling is not
// where you want to start reading.
func TestReopeningStartsAtTheTop(t *testing.T) {
	m := helpOn(t, 80, 14)
	m = press(t, m, "j")
	m = press(t, m, "j")
	m = press(t, m, "esc")
	m = press(t, m, "?")
	if !strings.Contains(m.View(), helpRows[0][1]) {
		t.Errorf("it should open at the top:\n%s", m.View())
	}
}
