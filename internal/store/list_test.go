package store

import (
	"testing"
	"time"

	"todo.mirumo.net/internal/task"
)

// seed inserts a fixed set of tasks and returns title -> id.
func seed(t *testing.T, s Store) map[string]int64 {
	t.Helper()
	ids := map[string]int64{}
	add := func(ti task.Task) {
		ti.CreatedAt, ti.UpdatedAt = ref(), ref()
		got, err := s.Add(ti)
		if err != nil {
			t.Fatalf("Add %q: %v", ti.Title, err)
		}
		ids[ti.Title] = got.ID
	}
	add(task.Task{Title: "overdue one", Due: day(2026, 8, 20), Priority: task.PriLow})
	add(task.Task{Title: "today one", Due: day(2026, 8, 29), Priority: task.PriHigh, Tags: []string{"urgent"}})
	add(task.Task{Title: "next week one", Due: day(2026, 9, 10), Project: "/p/work"})
	add(task.Task{Title: "undated one", Priority: task.PriMed, Tags: []string{"urgent", "misc"}})
	add(task.Task{Title: "work one", Project: "/p/work", Tags: []string{"misc"}})
	return ids
}

func titles(ts []task.Task) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Title
	}
	return out
}

func assertTitles(t *testing.T, got []task.Task, want ...string) {
	t.Helper()
	g := titles(got)
	if len(g) != len(want) {
		t.Fatalf("= %v, want %v", g, want)
	}
	for i := range want {
		if g[i] != want[i] {
			t.Fatalf("= %v, want %v", g, want)
		}
	}
}

func str(s string) *string { return &s }

func TestListDefaultsToOpenTasksSortedByDue(t *testing.T) {
	s := newStore(t)
	seed(t, s)
	got, err := s.List(task.Filter{Sort: task.SortDue}, ref())
	if err != nil {
		t.Fatal(err)
	}
	assertTitles(t, got, "overdue one", "today one", "next week one", "undated one", "work one")
}

func TestListSortByPriority(t *testing.T) {
	s := newStore(t)
	seed(t, s)
	got, err := s.List(task.Filter{Sort: task.SortPriority}, ref())
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Title != "today one" || got[1].Title != "undated one" {
		t.Errorf("= %v, want high first and med second", titles(got))
	}
}

func TestListFilterByProject(t *testing.T) {
	s := newStore(t)
	seed(t, s)
	got, err := s.List(task.Filter{Project: str("/p/work")}, ref())
	if err != nil {
		t.Fatal(err)
	}
	assertTitles(t, got, "next week one", "work one")
}

func TestListFilterByEmptyProjectMeansUncategorized(t *testing.T) {
	s := newStore(t)
	seed(t, s)
	got, err := s.List(task.Filter{Project: str("")}, ref())
	if err != nil {
		t.Fatal(err)
	}
	assertTitles(t, got, "overdue one", "today one", "undated one")
}

func TestListFilterByTagsIsAnd(t *testing.T) {
	s := newStore(t)
	seed(t, s)
	got, err := s.List(task.Filter{Tags: []string{"urgent", "misc"}}, ref())
	if err != nil {
		t.Fatal(err)
	}
	assertTitles(t, got, "undated one")
}

// Having no tags is a filter of its own: an untagged task cannot be reached
// through any tag, so without this it is only findable by clearing filters.
func TestListFilterUntagged(t *testing.T) {
	s := newStore(t)
	seed(t, s)
	got, err := s.List(task.Filter{Untagged: true}, ref())
	if err != nil {
		t.Fatal(err)
	}
	assertTitles(t, got, "overdue one", "next week one")
}

func TestListDueRanges(t *testing.T) {
	s := newStore(t)
	seed(t, s)
	cases := []struct {
		name string
		f    task.Filter
		want []string
	}{
		{"today", task.Filter{DueRange: task.DueToday}, []string{"today one"}},
		{"overdue", task.Filter{DueRange: task.DueOverdue}, []string{"overdue one"}},
		{"week", task.Filter{DueRange: task.DueWeek}, []string{"overdue one", "today one"}},
		{"on", task.Filter{DueRange: task.DueOn, DueOn: *day(2026, 9, 10)}, []string{"next week one"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := s.List(c.f, ref())
			if err != nil {
				t.Fatal(err)
			}
			assertTitles(t, got, c.want...)
		})
	}
}

func TestListSearchIsCaseInsensitiveSubstring(t *testing.T) {
	s := newStore(t)
	if _, err := s.Add(task.Task{Title: "Buy Milk", CreatedAt: ref(), UpdatedAt: ref()}); err != nil {
		t.Fatal(err)
	}
	got, err := s.List(task.Filter{Search: "buy"}, ref())
	if err != nil {
		t.Fatal(err)
	}
	assertTitles(t, got, "Buy Milk")
}

func TestListDoneVisibility(t *testing.T) {
	s := newStore(t)
	ids := seed(t, s)
	if err := s.SetDone(ids["today one"], true, ref()); err != nil {
		t.Fatal(err)
	}
	open, _ := s.List(task.Filter{}, ref())
	if len(open) != 4 {
		t.Errorf("the default should list open tasks only, got %v", titles(open))
	}
	all, _ := s.List(task.Filter{IncludeDone: true}, ref())
	if len(all) != 5 {
		t.Errorf("IncludeDone should list everything, got %v", titles(all))
	}
	done, _ := s.List(task.Filter{OnlyDone: true}, ref())
	assertTitles(t, done, "today one")
}

func TestListLoadsTags(t *testing.T) {
	s := newStore(t)
	seed(t, s)
	got, err := s.List(task.Filter{Search: "undated"}, ref())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Tags) != 2 {
		t.Errorf("listed tasks should carry their tags, got %+v", got)
	}
}

var _ = time.Time{}

// Due filters work by day, so a time of day must not hide a task from them.
func TestDueFiltersIgnoreTimeOfDay(t *testing.T) {
	s := newStore(t)
	morning := time.Date(2026, 8, 29, 9, 0, 0, 0, time.Local)
	weekEnd := time.Date(2026, 9, 5, 23, 30, 0, 0, time.Local)
	for _, ti := range []task.Task{
		{Title: "timed today", Due: &morning, DueHasTime: true, CreatedAt: ref(), UpdatedAt: ref()},
		{Title: "timed week edge", Due: &weekEnd, DueHasTime: true, CreatedAt: ref(), UpdatedAt: ref()},
	} {
		if _, err := s.Add(ti); err != nil {
			t.Fatal(err)
		}
	}

	got, err := s.List(task.Filter{DueRange: task.DueToday}, ref())
	if err != nil {
		t.Fatal(err)
	}
	assertTitles(t, got, "timed today")

	got, err = s.List(task.Filter{DueRange: task.DueWeek}, ref())
	if err != nil {
		t.Fatal(err)
	}
	assertTitles(t, got, "timed today", "timed week edge")

	on := time.Date(2026, 9, 5, 0, 0, 0, 0, time.Local)
	got, err = s.List(task.Filter{DueRange: task.DueOn, DueOn: on}, ref())
	if err != nil {
		t.Fatal(err)
	}
	assertTitles(t, got, "timed week edge")
}
