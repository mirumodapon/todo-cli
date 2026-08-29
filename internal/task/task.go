// Package task 定義待辦事項的領域型別。此套件不做任何 IO。
package task

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Priority 是優先度。數值由低到高遞增，SQL 可直接 ORDER BY。
type Priority int

const (
	PriNone Priority = iota
	PriLow
	PriMed
	PriHigh
)

// ParsePriority 解析使用者輸入。空字串代表未設定。
func ParsePriority(s string) (Priority, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return PriNone, nil
	case "low":
		return PriLow, nil
	case "med":
		return PriMed, nil
	case "high":
		return PriHigh, nil
	}
	return PriNone, fmt.Errorf("看不懂的優先度：%q（可用 low、med、high）", s)
}

// String 回傳 CLI 用的英文代號。
func (p Priority) String() string {
	switch p {
	case PriLow:
		return "low"
	case PriMed:
		return "med"
	case PriHigh:
		return "high"
	}
	return ""
}

// Label 回傳顯示用的中文標記。
func (p Priority) Label() string {
	switch p {
	case PriLow:
		return "低"
	case PriMed:
		return "中"
	case PriHigh:
		return "高"
	}
	return ""
}

// Task 是一條待辦事項。Project 為空字串代表全域未分類。
type Task struct {
	ID        int64
	Title     string
	Project   string
	Due       *time.Time
	Priority  Priority
	DoneAt    *time.Time
	Tags      []string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Done 回報是否已完成。
func (t Task) Done() bool { return t.DoneAt != nil }

// ValidateTitle 去掉頭尾空白並確認非空。
func ValidateTitle(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("標題不能是空的")
	}
	return s, nil
}

// NormalizeTags 去空白、去空字串、去重，並保留首次出現的順序。
func NormalizeTags(tags []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// SortBy 是清單排序方式。
type SortBy int

const (
	SortDue SortBy = iota
	SortPriority
	SortCreated
)

// ParseSortBy 解析 -s 的值。
func ParseSortBy(s string) (SortBy, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "due":
		return SortDue, nil
	case "pri":
		return SortPriority, nil
	case "created":
		return SortCreated, nil
	}
	return SortDue, fmt.Errorf("看不懂的排序：%q（可用 due、pri、created）", s)
}

// DueRange 是截止日的過濾範圍。
type DueRange int

const (
	DueAny DueRange = iota
	DueToday
	DueWeek
	DueOverdue
	DueOn
)

// Filter 描述一次查詢。Project 為 nil 代表不依專案過濾；
// 指向空字串則代表「只看全域未分類」。
type Filter struct {
	Project     *string
	Tags        []string
	DueRange    DueRange
	DueOn       time.Time
	Priority    *Priority
	Search      string
	IncludeDone bool
	OnlyDone    bool
	Sort        SortBy
}
