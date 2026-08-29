package store

import (
	"testing"
	"time"

	"todo.mirumo.net/internal/task"
)

// seed 建立一組固定資料，並回傳 title -> id。
func seed(t *testing.T, s Store) map[string]int64 {
	t.Helper()
	ids := map[string]int64{}
	add := func(ti task.Task) {
		ti.CreatedAt, ti.UpdatedAt = ref(), ref()
		got, err := s.Add(ti)
		if err != nil {
			t.Fatalf("Add %q：%v", ti.Title, err)
		}
		ids[ti.Title] = got.ID
	}
	add(task.Task{Title: "逾期的事", Due: day(2026, 8, 20), Priority: task.PriLow})
	add(task.Task{Title: "今天的事", Due: day(2026, 8, 29), Priority: task.PriHigh, Tags: []string{"急"}})
	add(task.Task{Title: "下週的事", Due: day(2026, 9, 10), Project: "/p/work"})
	add(task.Task{Title: "沒期限的事", Priority: task.PriMed, Tags: []string{"急", "雜"}})
	add(task.Task{Title: "工作上的事", Project: "/p/work", Tags: []string{"雜"}})
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
		t.Fatalf("= %v，預期 %v", g, want)
	}
	for i := range want {
		if g[i] != want[i] {
			t.Fatalf("= %v，預期 %v", g, want)
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
	assertTitles(t, got, "逾期的事", "今天的事", "下週的事", "沒期限的事", "工作上的事")
}

func TestListSortByPriority(t *testing.T) {
	s := newStore(t)
	seed(t, s)
	got, err := s.List(task.Filter{Sort: task.SortPriority}, ref())
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Title != "今天的事" || got[1].Title != "沒期限的事" {
		t.Errorf("= %v，預期 high 在前、med 次之", titles(got))
	}
}

func TestListFilterByProject(t *testing.T) {
	s := newStore(t)
	seed(t, s)
	got, err := s.List(task.Filter{Project: str("/p/work")}, ref())
	if err != nil {
		t.Fatal(err)
	}
	assertTitles(t, got, "下週的事", "工作上的事")
}

func TestListFilterByEmptyProjectMeansUncategorized(t *testing.T) {
	s := newStore(t)
	seed(t, s)
	got, err := s.List(task.Filter{Project: str("")}, ref())
	if err != nil {
		t.Fatal(err)
	}
	assertTitles(t, got, "逾期的事", "今天的事", "沒期限的事")
}

func TestListFilterByTagsIsAnd(t *testing.T) {
	s := newStore(t)
	seed(t, s)
	got, err := s.List(task.Filter{Tags: []string{"急", "雜"}}, ref())
	if err != nil {
		t.Fatal(err)
	}
	assertTitles(t, got, "沒期限的事")
}

func TestListDueRanges(t *testing.T) {
	s := newStore(t)
	seed(t, s)
	cases := []struct {
		name string
		f    task.Filter
		want []string
	}{
		{"today", task.Filter{DueRange: task.DueToday}, []string{"今天的事"}},
		{"overdue", task.Filter{DueRange: task.DueOverdue}, []string{"逾期的事"}},
		{"week", task.Filter{DueRange: task.DueWeek}, []string{"逾期的事", "今天的事"}},
		{"on", task.Filter{DueRange: task.DueOn, DueOn: *day(2026, 9, 10)}, []string{"下週的事"}},
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
	if err := s.SetDone(ids["今天的事"], true, ref()); err != nil {
		t.Fatal(err)
	}
	open, _ := s.List(task.Filter{}, ref())
	if len(open) != 4 {
		t.Errorf("預設應只列未完成，得到 %v", titles(open))
	}
	all, _ := s.List(task.Filter{IncludeDone: true}, ref())
	if len(all) != 5 {
		t.Errorf("IncludeDone 應列全部，得到 %v", titles(all))
	}
	done, _ := s.List(task.Filter{OnlyDone: true}, ref())
	assertTitles(t, done, "今天的事")
}

func TestListLoadsTags(t *testing.T) {
	s := newStore(t)
	seed(t, s)
	got, err := s.List(task.Filter{Search: "沒期限"}, ref())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Tags) != 2 {
		t.Errorf("清單項目應該帶著標籤，得到 %+v", got)
	}
}

var _ = time.Time{}
