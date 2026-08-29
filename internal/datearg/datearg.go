// Package datearg 負責截止日的輸入解析與人類化顯示。
package datearg

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Day 把時間截成當地時區的當日零時。
func Day(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

var weekdays = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday,
	"wed": time.Wednesday, "thu": time.Thursday,
	"fri": time.Friday, "sat": time.Saturday,
}

var zhWeekday = [7]string{"週日", "週一", "週二", "週三", "週四", "週五", "週六"}

// Parse 解析使用者輸入的截止日，回傳當地時區的當日零時。
// 接受 today / tomorrow / yesterday、星期簡稱、+3d、+2w、YYYY-MM-DD。
func Parse(s string, now time.Time) (time.Time, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	base := Day(now)
	switch s {
	case "today":
		return base, nil
	case "tomorrow":
		return base.AddDate(0, 0, 1), nil
	case "yesterday":
		return base.AddDate(0, 0, -1), nil
	}
	if wd, ok := weekdays[s]; ok {
		// 未來七天內（含今天）第一個符合的日子。
		for i := 0; i < 7; i++ {
			if d := base.AddDate(0, 0, i); d.Weekday() == wd {
				return d, nil
			}
		}
	}
	if strings.HasPrefix(s, "+") && len(s) > 2 {
		unit := s[len(s)-1]
		if n, err := strconv.Atoi(s[1 : len(s)-1]); err == nil {
			switch unit {
			case 'd':
				return base.AddDate(0, 0, n), nil
			case 'w':
				return base.AddDate(0, 0, 7*n), nil
			}
		}
	}
	if t, err := time.ParseInLocation("2006-01-02", s, now.Location()); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("看不懂的日期：%q（可用 today、tomorrow、fri、+3d、2026-09-01）", s)
}

// Format 產生顯示用字串：逾期 N 天 / 今天 / 明天 / 週五 / 09-05 / 2027-01-02。
func Format(due, now time.Time) string {
	d, base := Day(due), Day(now)
	diff := int(math.Round(d.Sub(base).Hours() / 24))
	switch {
	case diff < 0:
		return fmt.Sprintf("逾期 %d 天", -diff)
	case diff == 0:
		return "今天"
	case diff == 1:
		return "明天"
	case diff < 7:
		return zhWeekday[int(d.Weekday())]
	case d.Year() == base.Year():
		return d.Format("01-02")
	default:
		return d.Format("2006-01-02")
	}
}
