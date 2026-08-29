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
		{"urgent", PriNone, true},
	}
	for _, c := range cases {
		got, err := ParsePriority(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("ParsePriority(%q) err = %v，預期錯誤 = %v", c.in, err, c.wantErr)
			continue
		}
		if err == nil && got != c.want {
			t.Errorf("ParsePriority(%q) = %v，預期 %v", c.in, got, c.want)
		}
	}
}

func TestPriorityOrderingIsAscending(t *testing.T) {
	if !(PriNone < PriLow && PriLow < PriMed && PriMed < PriHigh) {
		t.Error("Priority 必須由低到高遞增，SQL 才能用 ORDER BY priority DESC")
	}
}

func TestValidateTitle(t *testing.T) {
	got, err := ValidateTitle("  買牛奶  ")
	if err != nil {
		t.Fatalf("非預期錯誤：%v", err)
	}
	if got != "買牛奶" {
		t.Errorf("= %q，預期去掉頭尾空白的 買牛奶", got)
	}
	if _, err := ValidateTitle("   "); err == nil {
		t.Error("全空白的標題應該報錯")
	}
}

func TestDone(t *testing.T) {
	if (Task{}).Done() {
		t.Error("DoneAt 為 nil 時 Done() 應為 false")
	}
	now := time.Now()
	if !(Task{DoneAt: &now}).Done() {
		t.Error("DoneAt 非 nil 時 Done() 應為 true")
	}
}

func TestNormalizeTags(t *testing.T) {
	got := NormalizeTags([]string{" 購物 ", "家務", "購物", ""})
	if len(got) != 2 || got[0] != "購物" || got[1] != "家務" {
		t.Errorf("= %v，預期 [購物 家務]：去空白、去重、去空字串、保留出現順序", got)
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
		t.Error("未知的排序欄位應該報錯")
	}
}
