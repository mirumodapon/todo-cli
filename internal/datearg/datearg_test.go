package datearg

import (
	"testing"
	"time"
)

// 2026-08-29 is a Saturday.
func ref() time.Time {
	return time.Date(2026, 8, 29, 15, 4, 5, 0, time.Local)
}

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"today", "2026-08-29"},
		{"tomorrow", "2026-08-30"},
		{"yesterday", "2026-08-28"},
		{"sat", "2026-08-29"},
		{"mon", "2026-08-31"},
		{"+3d", "2026-09-01"},
		{"+2w", "2026-09-12"},
		{"2026-12-25", "2026-12-25"},
		{"  TOMORROW  ", "2026-08-30"},
	}
	for _, c := range cases {
		got, err := Parse(c.in, ref())
		if err != nil {
			t.Errorf("Parse(%q) 非預期錯誤：%v", c.in, err)
			continue
		}
		if got.Format("2006-01-02") != c.want {
			t.Errorf("Parse(%q) = %s，預期 %s", c.in, got.Format("2006-01-02"), c.want)
		}
	}
}

func TestParseReturnsMidnight(t *testing.T) {
	got, err := Parse("today", ref())
	if err != nil {
		t.Fatal(err)
	}
	if got.Hour() != 0 || got.Minute() != 0 || got.Second() != 0 {
		t.Errorf("= %v，預期當日零時", got)
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	for _, in := range []string{"someday", "2026-13-45", "+3x", "", "+d"} {
		if _, err := Parse(in, ref()); err == nil {
			t.Errorf("Parse(%q) 應該報錯", in)
		}
	}
}

func TestFormat(t *testing.T) {
	cases := []struct {
		due  string
		want string
	}{
		{"2026-08-27", "逾期 2 天"},
		{"2026-08-28", "逾期 1 天"},
		{"2026-08-29", "今天"},
		{"2026-08-30", "明天"},
		{"2026-08-31", "週一"},
		{"2026-09-04", "週五"},
		{"2026-09-05", "09-05"},
		{"2027-01-02", "2027-01-02"},
	}
	for _, c := range cases {
		due, err := time.ParseInLocation("2006-01-02", c.due, time.Local)
		if err != nil {
			t.Fatal(err)
		}
		if got := Format(due, ref()); got != c.want {
			t.Errorf("Format(%s) = %q，預期 %q", c.due, got, c.want)
		}
	}
}
