package task

import (
	"testing"
	"time"
)

func TestParsePriority(t *testing.T) {
	cases := []struct {
		in      string
		want    Priority
		wantErr bool
	}{
		{"", PriNone, false},
		{"low", PriLow, false},
		{"med", PriMed, false},
		{"high", PriHigh, false},
		{"HIGH", PriHigh, false},
		{"!", PriLow, false},
		{"!!", PriMed, false},
		{"!!!", PriHigh, false},
		{"  !!  ", PriMed, false},
		{"urgent", PriNone, true},
		{"!!!!", PriNone, true},
	}
	for _, c := range cases {
		got, err := ParsePriority(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("ParsePriority(%q) err = %v, want an error = %v", c.in, err, c.wantErr)
			continue
		}
		if err == nil && got != c.want {
			t.Errorf("ParsePriority(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestPriorityMarks(t *testing.T) {
	for p, want := range map[Priority]string{
		PriNone: "", PriLow: "!", PriMed: "!!", PriHigh: "!!!",
	} {
		if got := p.Marks(); got != want {
			t.Errorf("%v.Marks() = %q, want %q", p, got, want)
		}
	}
}

// Whatever Marks renders must parse back to the same priority.
func TestMarksRoundTrip(t *testing.T) {
	for _, p := range []Priority{PriNone, PriLow, PriMed, PriHigh} {
		got, err := ParsePriority(p.Marks())
		if err != nil {
			t.Errorf("ParsePriority(%q): %v", p.Marks(), err)
			continue
		}
		if got != p {
			t.Errorf("%q parsed back as %v, want %v", p.Marks(), got, p)
		}
	}
}

func TestPriorityOrderingIsAscending(t *testing.T) {
	if !(PriNone < PriLow && PriLow < PriMed && PriMed < PriHigh) {
		t.Error("Priority must ascend from low to high so SQL can ORDER BY priority DESC")
	}
}

func TestValidateTitle(t *testing.T) {
	got, err := ValidateTitle("  buy milk  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "buy milk" {
		t.Errorf("= %q, want buy milk with surrounding whitespace trimmed", got)
	}
	if _, err := ValidateTitle("   "); err == nil {
		t.Error("an all-whitespace title should fail")
	}
}

func TestDone(t *testing.T) {
	if (Task{}).Done() {
		t.Error("Done() should be false when DoneAt is nil")
	}
	now := time.Now()
	if !(Task{DoneAt: &now}).Done() {
		t.Error("Done() should be true when DoneAt is set")
	}
}

func TestNormalizeTags(t *testing.T) {
	got := NormalizeTags([]string{" shopping ", "chores", "shopping", ""})
	if len(got) != 2 || got[0] != "shopping" || got[1] != "chores" {
		t.Errorf("= %v, want [shopping chores]: trimmed, deduped, empties dropped, order kept", got)
	}
}

func TestParseSortBy(t *testing.T) {
	for in, want := range map[string]SortBy{"due": SortDue, "pri": SortPriority, "created": SortCreated} {
		got, err := ParseSortBy(in)
		if err != nil || got != want {
			t.Errorf("ParseSortBy(%q) = %v, %v", in, got, err)
		}
	}
	if _, err := ParseSortBy("title"); err == nil {
		t.Error("an unknown sort field should fail")
	}
}
