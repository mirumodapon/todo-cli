# todo CLI + TUI 實作計畫

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建出一個純本機的待辦事項工具 `todo`，CLI 為主要介面、`todo tui` 進入 Bubble Tea 瀏覽模式，資料存於 `~/.todo/todo.db`。

**Architecture:** 由外向內分層，內層零 IO。`argparse`、`task`、`datearg`、`project` 是純函式層；`store` 定義 `Store` 介面並以 SQLite 實作；`cli` 與 `tui` 只依賴 `Store` 介面，彼此不互相 import（`cli` 透過注入的 `RunTUI` 函式呼叫 TUI）；`cmd/todo` 負責組裝。

**Tech Stack:** Go 1.26、Bubble Tea + Bubbles + Lip Gloss、`modernc.org/sqlite`（純 Go，免 cgo）、自寫參數解析。

設計文件：`docs/superpowers/specs/2026-08-29-todo-cli-tui-design.md`

## Global Constraints

- Go 1.26；module path `todo.mirumo.net`；binary 名 `todo`
- 依賴只允許這四個：`github.com/charmbracelet/bubbletea@v1`、`github.com/charmbracelet/bubbles@v0`、`github.com/charmbracelet/lipgloss@v1`、`modernc.org/sqlite@v1`。不得引入 Cobra、pflag，也不得使用標準庫 `flag`
- 資料庫預設路徑 `~/.todo/todo.db`，目錄權限 0700；`--db <path>` flag 與 `TODO_DB` 環境變數可覆寫
- **任何測試都不得讀寫 `~/.todo`**。需要真實檔案時用 `t.TempDir()`；store 測試用 `:memory:`
- 使用者可見的訊息、錯誤、TUI 文案一律繁體中文
- 每個 task 都是 TDD：先寫失敗的測試 → 確認失敗 → 最小實作 → 確認通過 → commit
- commit 用 `git commit --no-gpg-sign`（此環境的 pinentry 無法在非 TTY 下啟動）
- 時間格式：時間戳存 RFC3339，截止日存 `YYYY-MM-DD`
- 空字串的 `project` 是合法的一等狀態，代表「全域未分類」，不是缺漏

---

## File Structure

| 檔案 | 職責 |
|---|---|
| `go.mod` | module `todo.mirumo.net` |
| `internal/argparse/argparse.go` | GNU 風格參數解析，支援可選值 flag |
| `internal/task/task.go` | `Task`、`Priority`、`Filter`、`SortBy`、驗證 |
| `internal/datearg/datearg.go` | 日期字串解析與人類化顯示 |
| `internal/project/project.go` | 由目錄推導專案路徑（往上找 `.git`） |
| `internal/store/store.go` | `Store` 介面、`ProjectCount`、`ErrNotFound` |
| `internal/store/sqlite.go` | SQLite 實作與 schema |
| `internal/cli/app.go` | `App`、`Run` dispatch、`SplitGlobal`、usage |
| `internal/cli/format.go` | 清單輸出排版與顏色 |
| `internal/cli/cmd_add.go` | `add` |
| `internal/cli/cmd_ls.go` | `ls` |
| `internal/cli/cmd_mark.go` | `done` / `undone` / `rm` |
| `internal/cli/cmd_edit.go` | `edit` |
| `internal/cli/cmd_meta.go` | `projects` / `tags` |
| `internal/tui/tui.go` | 根 Model、Init/Update/View、`Run` |
| `internal/tui/cmds.go` | `tea.Cmd` 與 msg 型別 |
| `internal/tui/list.go` | 清單渲染 |
| `internal/tui/picker.go` | 專案 / 標籤選單 |
| `internal/tui/form.go` | 新增 / 編輯表單 |
| `cmd/todo/main.go` | 組裝與離開碼 |

---

### Task 1: 專案骨架與參數解析

**Files:**
- Create: `go.mod`
- Create: `internal/argparse/argparse.go`
- Test: `internal/argparse/argparse_test.go`

**Interfaces:**
- Consumes: 無
- Produces: `argparse.Kind`（`Bool` / `String` / `StringSlice` / `OptionalString`）、`argparse.Spec{Long, Short string; Kind Kind; Usage string}`、`argparse.New(...Spec) *Set`、`(*Set).Parse([]string) (*Result, error)`、`(*Result).Changed(long string) bool`、`.Bool(long string) bool`、`.String(long string) string`、`.Strings(long string) []string`、`.Optional(long string) (string, bool)`、`.Args() []string`、`(*Set).Usage() string`

- [ ] **Step 1: 建立 module 骨架**

```bash
go mod init todo.mirumo.net
mkdir -p internal/argparse internal/task internal/datearg internal/project internal/store internal/cli internal/tui cmd/todo
```

- [ ] **Step 2: 寫失敗的測試**

建立 `internal/argparse/argparse_test.go`：

```go
package argparse

import (
	"strings"
	"testing"
)

func specs() *Set {
	return New(
		Spec{Long: "all", Short: "a", Kind: Bool, Usage: "含已完成"},
		Spec{Long: "due", Short: "d", Kind: String, Usage: "截止日"},
		Spec{Long: "tag", Short: "t", Kind: StringSlice, Usage: "標籤，可重複"},
		Spec{Long: "project", Short: "p", Kind: OptionalString, Usage: "專案"},
	)
}

func TestParseLongAndShortForms(t *testing.T) {
	r, err := specs().Parse([]string{"買牛奶", "--due", "2026-09-01", "-a", "-t", "購物", "--tag=家務"})
	if err != nil {
		t.Fatalf("非預期錯誤：%v", err)
	}
	if got := r.Args(); len(got) != 1 || got[0] != "買牛奶" {
		t.Errorf("位置參數 = %v，預期 [買牛奶]", got)
	}
	if !r.Bool("all") {
		t.Error("--all 應為 true")
	}
	if got := r.String("due"); got != "2026-09-01" {
		t.Errorf("due = %q，預期 2026-09-01", got)
	}
	if got := r.Strings("tag"); len(got) != 2 || got[0] != "購物" || got[1] != "家務" {
		t.Errorf("tag = %v，預期 [購物 家務]", got)
	}
}

func TestOptionalStringThreeStates(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		changed  bool
		hasValue bool
		value    string
	}{
		{"沒給", []string{"x"}, false, false, ""},
		{"給了但無值", []string{"x", "-p"}, true, false, ""},
		{"無值且後面接別的 flag", []string{"x", "-p", "-a"}, true, false, ""},
		{"空格式給值", []string{"x", "-p", "work"}, true, true, "work"},
		{"等號式給值", []string{"x", "--project=work"}, true, true, "work"},
		{"等號式給空值", []string{"x", "-p="}, true, true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := specs().Parse(c.args)
			if err != nil {
				t.Fatalf("非預期錯誤：%v", err)
			}
			if r.Changed("project") != c.changed {
				t.Errorf("Changed = %v，預期 %v", r.Changed("project"), c.changed)
			}
			v, has := r.Optional("project")
			if has != c.hasValue || v != c.value {
				t.Errorf("Optional = (%q, %v)，預期 (%q, %v)", v, has, c.value, c.hasValue)
			}
		})
	}
}

func TestStringFlagAcceptsEmptyValue(t *testing.T) {
	r, err := specs().Parse([]string{"--due", ""})
	if err != nil {
		t.Fatalf("非預期錯誤：%v", err)
	}
	if !r.Changed("due") || r.String("due") != "" {
		t.Error("--due \"\" 應視為「有給且值為空」")
	}
}

func TestDoubleDashEndsFlags(t *testing.T) {
	r, err := specs().Parse([]string{"--", "-a", "--due"})
	if err != nil {
		t.Fatalf("非預期錯誤：%v", err)
	}
	if got := r.Args(); len(got) != 2 || got[0] != "-a" || got[1] != "--due" {
		t.Errorf("位置參數 = %v，預期 [-a --due]", got)
	}
	if r.Bool("all") {
		t.Error("-- 之後不應再解析 flag")
	}
}

func TestErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"未知長 flag", []string{"--nope"}, "未知的 flag：--nope"},
		{"未知短 flag", []string{"-z"}, "未知的 flag：-z"},
		{"字串 flag 缺值", []string{"--due"}, "flag --due 需要一個值"},
		{"布林 flag 不接受值", []string{"--all=1"}, "flag --all 不接受值"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := specs().Parse(c.args)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v，預期含 %q", err, c.want)
			}
		})
	}
}

func TestUsageListsFlags(t *testing.T) {
	u := specs().Usage()
	for _, want := range []string{"-a, --all", "-d, --due", "含已完成"} {
		if !strings.Contains(u, want) {
			t.Errorf("Usage 缺少 %q，實際：\n%s", want, u)
		}
	}
}
```

- [ ] **Step 3: 執行測試確認失敗**

Run: `go test ./internal/argparse/`
Expected: FAIL，編譯錯誤 `undefined: New`

- [ ] **Step 4: 實作**

建立 `internal/argparse/argparse.go`：

```go
// Package argparse 提供 GNU 風格的命令列參數解析。
//
// 不使用 pflag 或標準庫 flag 的原因：本工具的 -p/--project 需要「可選值」——
// 不給值代表當前目錄，給值代表指定專案名。兩個現成庫都不支援這種 flag。
package argparse

import (
	"fmt"
	"strings"
)

// Kind 決定一個 flag 如何吃掉後續的 token。
type Kind int

const (
	// Bool 永不吃下一個 token。
	Bool Kind = iota
	// String 必須有值，缺值視為錯誤。
	String
	// StringSlice 同 String，但可重複出現、累積成清單。
	StringSlice
	// OptionalString 的下一個 token 若存在且不以 "-" 開頭就吃掉，否則視為「有給但無值」。
	OptionalString
)

// Spec 描述一個 flag。Short 不含前導的 "-"，可為空字串。
type Spec struct {
	Long  string
	Short string
	Kind  Kind
	Usage string
}

// Set 是一個子指令的 flag 定義集合。
type Set struct{ specs []Spec }

// New 建立 Set。
func New(specs ...Spec) *Set { return &Set{specs: specs} }

type value struct {
	set      bool
	hasValue bool
	str      string
	strs     []string
}

// Result 是一次解析的結果。
type Result struct {
	vals map[string]*value
	args []string
}

// Changed 回報該 flag 有沒有出現在命令列上。
func (r *Result) Changed(long string) bool {
	v, ok := r.vals[long]
	return ok && v.set
}

// Bool 回傳布林 flag 是否出現。
func (r *Result) Bool(long string) bool { return r.Changed(long) }

// String 回傳字串 flag 的值；沒給時為空字串（用 Changed 區分）。
func (r *Result) String(long string) string {
	if v, ok := r.vals[long]; ok {
		return v.str
	}
	return ""
}

// Strings 回傳可重複 flag 累積的值。
func (r *Result) Strings(long string) []string {
	if v, ok := r.vals[long]; ok {
		return v.strs
	}
	return nil
}

// Optional 回傳可選值 flag 的值與「有沒有帶值」。
// 三態判讀：Changed 為 false 是沒給；Changed 為 true 但 hasValue 為 false 是給了但無值。
func (r *Result) Optional(long string) (string, bool) {
	v, ok := r.vals[long]
	if !ok || !v.set {
		return "", false
	}
	return v.str, v.hasValue
}

// Args 回傳位置參數。
func (r *Result) Args() []string { return r.args }

// Usage 產生 flag 說明文字。
func (s *Set) Usage() string {
	var b strings.Builder
	for _, sp := range s.specs {
		name := "    --" + sp.Long
		if sp.Short != "" {
			name = "-" + sp.Short + ", --" + sp.Long
		}
		fmt.Fprintf(&b, "  %-20s %s\n", name, sp.Usage)
	}
	return b.String()
}

func (s *Set) find(name string, long bool) (Spec, bool) {
	for _, sp := range s.specs {
		if long && sp.Long == name {
			return sp, true
		}
		if !long && sp.Short != "" && sp.Short == name {
			return sp, true
		}
	}
	return Spec{}, false
}

func cut(s string) (name, val string, has bool) {
	if k := strings.IndexByte(s, '='); k >= 0 {
		return s[:k], s[k+1:], true
	}
	return s, "", false
}

// Parse 解析 args（不含程式名與子指令名）。
func (s *Set) Parse(args []string) (*Result, error) {
	r := &Result{vals: map[string]*value{}}
	for _, sp := range s.specs {
		r.vals[sp.Long] = &value{}
	}
	for i := 0; i < len(args); {
		a := args[i]
		switch {
		case a == "--":
			r.args = append(r.args, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(a, "--"):
			name, inline, hasInline := cut(a[2:])
			sp, ok := s.find(name, true)
			if !ok {
				return nil, fmt.Errorf("未知的 flag：--%s", name)
			}
			used, err := s.assign(r, sp, inline, hasInline, args, i)
			if err != nil {
				return nil, err
			}
			i += used
		case len(a) > 1 && strings.HasPrefix(a, "-"):
			name, inline, hasInline := cut(a[1:])
			sp, ok := s.find(name, false)
			if !ok {
				return nil, fmt.Errorf("未知的 flag：-%s", name)
			}
			used, err := s.assign(r, sp, inline, hasInline, args, i)
			if err != nil {
				return nil, err
			}
			i += used
		default:
			r.args = append(r.args, a)
			i++
		}
	}
	return r, nil
}

// assign 套用一個 flag，回傳吃掉幾個 token。
func (s *Set) assign(r *Result, sp Spec, inline string, hasInline bool, args []string, i int) (int, error) {
	v := r.vals[sp.Long]
	v.set = true
	switch sp.Kind {
	case Bool:
		if hasInline {
			return 0, fmt.Errorf("flag --%s 不接受值", sp.Long)
		}
		return 1, nil
	case String, StringSlice:
		val, used := inline, 1
		if !hasInline {
			if i+1 >= len(args) {
				return 0, fmt.Errorf("flag --%s 需要一個值", sp.Long)
			}
			val, used = args[i+1], 2
		}
		if sp.Kind == String {
			v.str, v.hasValue = val, true
		} else {
			v.strs = append(v.strs, val)
		}
		return used, nil
	case OptionalString:
		if hasInline {
			v.str, v.hasValue = inline, true
			return 1, nil
		}
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			v.str, v.hasValue = args[i+1], true
			return 2, nil
		}
		return 1, nil
	}
	return 1, nil
}
```

- [ ] **Step 5: 執行測試確認通過**

Run: `go test ./internal/argparse/ -v`
Expected: PASS，全部 case 綠燈

- [ ] **Step 6: Commit**

```bash
git add go.mod internal/argparse/
git commit --no-gpg-sign -m "feat(argparse): GNU 風格參數解析，支援可選值 flag"
```

---

### Task 2: 領域型別

**Files:**
- Create: `internal/task/task.go`
- Test: `internal/task/task_test.go`

**Interfaces:**
- Consumes: 無
- Produces: `task.Priority`（`PriNone`/`PriLow`/`PriMed`/`PriHigh`）、`task.ParsePriority(string) (Priority, error)`、`(Priority).String() string`、`(Priority).Label() string`、`task.Task{ID int64; Title, Project string; Due *time.Time; Priority Priority; DoneAt *time.Time; Tags []string; CreatedAt, UpdatedAt time.Time}`、`(Task).Done() bool`、`task.ValidateTitle(string) (string, error)`、`task.SortBy`（`SortDue`/`SortPriority`/`SortCreated`）、`task.ParseSortBy(string) (SortBy, error)`、`task.DueRange`（`DueAny`/`DueToday`/`DueWeek`/`DueOverdue`/`DueOn`）、`task.Filter`、`task.NormalizeTags([]string) []string`

- [ ] **Step 1: 寫失敗的測試**

建立 `internal/task/task_test.go`：

```go
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
```

- [ ] **Step 2: 執行測試確認失敗**

Run: `go test ./internal/task/`
Expected: FAIL，`undefined: ParsePriority`

- [ ] **Step 3: 實作**

建立 `internal/task/task.go`：

```go
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
```

- [ ] **Step 4: 執行測試確認通過**

Run: `go test ./internal/task/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/task/
git commit --no-gpg-sign -m "feat(task): 待辦事項領域型別與驗證"
```

---

### Task 3: 日期解析與顯示

**Files:**
- Create: `internal/datearg/datearg.go`
- Test: `internal/datearg/datearg_test.go`

**Interfaces:**
- Consumes: 無
- Produces: `datearg.Parse(s string, now time.Time) (time.Time, error)`、`datearg.Format(due, now time.Time) string`、`datearg.Day(t time.Time) time.Time`

- [ ] **Step 1: 寫失敗的測試**

建立 `internal/datearg/datearg_test.go`：

```go
package datearg

import (
	"testing"
	"time"
)

// 2026-08-29 是星期六。
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
		{"sat", "2026-08-29"},  // 當天就是週六，指向今天
		{"mon", "2026-08-31"},  // 未來七天內第一個週一
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
```

- [ ] **Step 2: 執行測試確認失敗**

Run: `go test ./internal/datearg/`
Expected: FAIL，`undefined: Parse`

- [ ] **Step 3: 實作**

建立 `internal/datearg/datearg.go`：

```go
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
```

- [ ] **Step 4: 執行測試確認通過**

Run: `go test ./internal/datearg/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/datearg/
git commit --no-gpg-sign -m "feat(datearg): 截止日解析與人類化顯示"
```

---

### Task 4: 專案路徑推導

**Files:**
- Create: `internal/project/project.go`
- Test: `internal/project/project_test.go`

**Interfaces:**
- Consumes: 無
- Produces: `project.Current(dir string) (string, error)`、`project.Label(path string) string`

- [ ] **Step 1: 寫失敗的測試**

建立 `internal/project/project_test.go`：

```go
package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCurrentFindsGitRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "internal", "store")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Current(sub)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(root)
	gotResolved, _ := filepath.EvalSymlinks(got)
	if gotResolved != want {
		t.Errorf("= %q，預期 repo 根 %q", gotResolved, want)
	}
}

func TestCurrentFallsBackToDir(t *testing.T) {
	dir := t.TempDir()
	got, err := Current(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("= %q，預期絕對路徑", got)
	}
	gotResolved, _ := filepath.EvalSymlinks(got)
	want, _ := filepath.EvalSymlinks(dir)
	if gotResolved != want {
		t.Errorf("= %q，沒有 .git 時應回傳目錄本身 %q", gotResolved, want)
	}
}

func TestCurrentAcceptsGitFile(t *testing.T) {
	// git worktree 的 .git 是檔案不是目錄。
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: /elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "a")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got, _ := Current(sub)
	gotResolved, _ := filepath.EvalSymlinks(got)
	want, _ := filepath.EvalSymlinks(root)
	if gotResolved != want {
		t.Errorf("= %q，.git 是檔案時仍應視為 repo 根 %q", gotResolved, want)
	}
}

func TestLabel(t *testing.T) {
	if got := Label("/Users/me/Projects/todo.mirumo.net"); got != "todo.mirumo.net" {
		t.Errorf("= %q，預期 basename", got)
	}
	if got := Label(""); got != "" {
		t.Errorf("= %q，空字串應原樣回傳（全域未分類）", got)
	}
}
```

- [ ] **Step 2: 執行測試確認失敗**

Run: `go test ./internal/project/`
Expected: FAIL，`undefined: Current`

- [ ] **Step 3: 實作**

建立 `internal/project/project.go`：

```go
// Package project 由檔案系統位置推導待辦事項所屬的專案。
package project

import (
	"os"
	"path/filepath"
)

// Current 從 dir 往上找 .git；找到就回傳該目錄，找不到則回傳 dir 本身。
// 一律回傳絕對路徑——目錄名會撞（兩個 repo 都可能有 docs/），路徑才唯一。
func Current(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for d := abs; ; {
		// git worktree 的 .git 是檔案，一般 repo 是目錄，兩種都算。
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return d, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return abs, nil
}

// Label 回傳顯示用的短名。空字串代表全域未分類，原樣回傳。
func Label(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Base(path)
}
```

- [ ] **Step 4: 執行測試確認通過**

Run: `go test ./internal/project/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/project/
git commit --no-gpg-sign -m "feat(project): 由當前目錄推導專案路徑"
```

---

### Task 5: Store 介面與 SQLite 基礎（schema、Add、Get、Close）

**Files:**
- Create: `internal/store/store.go`
- Create: `internal/store/sqlite.go`
- Test: `internal/store/sqlite_test.go`

**Interfaces:**
- Consumes: `task.Task`、`task.Priority`、`task.Filter`
- Produces: `store.Store` 介面、`store.ProjectCount{Path string; Open int}`、`store.ErrNotFound`、`store.OpenSQLite(path string) (Store, error)`

`Store` 介面完整定義（本 task 建立，後兩個 task 補齊實作）：

```go
type Store interface {
	Add(t task.Task) (task.Task, error)
	Get(id int64) (task.Task, error)
	List(f task.Filter, now time.Time) ([]task.Task, error)
	Update(t task.Task) error
	Delete(id int64) error
	SetDone(id int64, done bool, now time.Time) error
	Restore(t task.Task) error
	Tags() ([]string, error)
	Projects() ([]ProjectCount, error)
	Close() error
}
```

- [ ] **Step 1: 加入 SQLite 依賴**

```bash
go get modernc.org/sqlite@v1
```

- [ ] **Step 2: 寫失敗的測試**

建立 `internal/store/sqlite_test.go`：

```go
package store

import (
	"errors"
	"testing"
	"time"

	"todo.mirumo.net/internal/task"
)

func ref() time.Time { return time.Date(2026, 8, 29, 15, 0, 0, 0, time.Local) }

func day(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	return &t
}

// newStore 開一個 in-memory 的 store，測試永不碰 ~/.todo。
func newStore(t *testing.T) Store {
	t.Helper()
	s, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite：%v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func sample() task.Task {
	return task.Task{
		Title:     "買牛奶",
		Project:   "/Users/me/Projects/home",
		Due:       day(2026, 9, 1),
		Priority:  task.PriHigh,
		Tags:      []string{"購物", "家務"},
		CreatedAt: ref(),
		UpdatedAt: ref(),
	}
}

func TestAddAssignsIDAndRoundTrips(t *testing.T) {
	s := newStore(t)
	got, err := s.Add(sample())
	if err != nil {
		t.Fatalf("Add：%v", err)
	}
	if got.ID == 0 {
		t.Fatal("Add 應該回填 ID")
	}
	back, err := s.Get(got.ID)
	if err != nil {
		t.Fatalf("Get：%v", err)
	}
	if back.Title != "買牛奶" || back.Project != "/Users/me/Projects/home" {
		t.Errorf("標題/專案沒存對：%+v", back)
	}
	if back.Priority != task.PriHigh {
		t.Errorf("priority = %v，預期 high", back.Priority)
	}
	if back.Due == nil || back.Due.Format("2006-01-02") != "2026-09-01" {
		t.Errorf("due = %v，預期 2026-09-01", back.Due)
	}
	if back.Done() {
		t.Error("新增的任務不該是已完成")
	}
	if len(back.Tags) != 2 {
		t.Errorf("tags = %v，預期兩個", back.Tags)
	}
}

func TestAddAcceptsEmptyProjectAndNilDue(t *testing.T) {
	s := newStore(t)
	in := task.Task{Title: "繳房租", CreatedAt: ref(), UpdatedAt: ref()}
	got, err := s.Add(in)
	if err != nil {
		t.Fatalf("Add：%v", err)
	}
	back, err := s.Get(got.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.Project != "" {
		t.Errorf("project = %q，全域未分類應為空字串", back.Project)
	}
	if back.Due != nil {
		t.Errorf("due = %v，預期 nil", back.Due)
	}
	if len(back.Tags) != 0 {
		t.Errorf("tags = %v，預期空", back.Tags)
	}
}

func TestGetMissingReturnsErrNotFound(t *testing.T) {
	s := newStore(t)
	_, err := s.Get(999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v，預期 ErrNotFound", err)
	}
}

func TestIDsAreNotReused(t *testing.T) {
	s := newStore(t)
	a, _ := s.Add(sample())
	if err := s.Delete(a.ID); err != nil {
		t.Fatalf("Delete：%v", err)
	}
	b, _ := s.Add(sample())
	if b.ID == a.ID {
		t.Errorf("id 被重用了（%d），AUTOINCREMENT 應該保證不重用", b.ID)
	}
}
```

（`TestIDsAreNotReused` 依賴 Task 7 的 `Delete`；本 task 先讓其餘測試通過，`Delete` 在 Task 7 補上前這個 case 會編譯失敗——所以本 task 的 `sqlite.go` 需先放上所有介面方法的骨架，未實作者回傳 `errors.New("尚未實作")`。）

- [ ] **Step 3: 執行測試確認失敗**

Run: `go test ./internal/store/`
Expected: FAIL，`undefined: OpenSQLite`

- [ ] **Step 4: 實作介面**

建立 `internal/store/store.go`：

```go
// Package store 負責待辦事項的持久化。
package store

import (
	"errors"
	"time"

	"todo.mirumo.net/internal/task"
)

// ErrNotFound 表示指定 id 的任務不存在。
var ErrNotFound = errors.New("找不到該任務")

// ProjectCount 是一個專案與它的未完成數。
type ProjectCount struct {
	Path string
	Open int
}

// Store 是待辦事項的儲存介面。CLI 與 TUI 只認這個介面，
// 測試時換成 :memory: 的實作，永不碰使用者的真實資料。
type Store interface {
	// Add 新增一筆並回傳含 ID 的結果。
	Add(t task.Task) (task.Task, error)
	// Get 依 id 取一筆，不存在時回傳 ErrNotFound。
	Get(id int64) (task.Task, error)
	// List 依 f 查詢；now 用於解析 today/week/overdue 這類相對條件。
	List(f task.Filter, now time.Time) ([]task.Task, error)
	// Update 依 t.ID 覆寫全部欄位（含標籤）。
	Update(t task.Task) error
	// Delete 刪除一筆，連帶清掉它的標籤關聯。
	Delete(id int64) error
	// SetDone 設定或取消完成狀態。
	SetDone(id int64, done bool, now time.Time) error
	// Restore 以 t.ID 原號重新插入，供 TUI 的刪除復原使用。
	Restore(t task.Task) error
	// Tags 列出至少被一個任務引用的標籤。
	Tags() ([]string, error)
	// Projects 列出所有專案與各自的未完成數。
	Projects() ([]ProjectCount, error)
	Close() error
}
```

建立 `internal/store/sqlite.go`：

```go
package store

import (
	"database/sql"
	"errors"
	"time"

	_ "modernc.org/sqlite"

	"todo.mirumo.net/internal/task"
)

const dateLayout = "2006-01-02"

const schema = `
CREATE TABLE IF NOT EXISTS tasks (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  title      TEXT    NOT NULL,
  project    TEXT    NOT NULL DEFAULT '',
  due        TEXT    NULL,
  priority   INTEGER NOT NULL DEFAULT 0,
  done_at    TEXT    NULL,
  created_at TEXT    NOT NULL,
  updated_at TEXT    NOT NULL
);
CREATE TABLE IF NOT EXISTS tags (
  id   INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE
);
CREATE TABLE IF NOT EXISTS task_tags (
  task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  tag_id  INTEGER NOT NULL REFERENCES tags(id)  ON DELETE CASCADE,
  PRIMARY KEY (task_id, tag_id)
);
CREATE INDEX IF NOT EXISTS idx_tasks_done ON tasks(done_at);
CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project);
`

const taskCols = `id, title, project, due, priority, done_at, created_at, updated_at`

type sqlStore struct{ db *sql.DB }

// OpenSQLite 開啟（必要時建立）資料庫。path 可以是檔案路徑或 ":memory:"。
func OpenSQLite(path string) (Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// 單一使用者的 CLI 不需要連線池；限制為一條連線，
	// PRAGMA 才會確實套用在之後所有查詢上（SQLite 預設關閉外鍵）。
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &sqlStore{db: db}, nil
}

func (s *sqlStore) Close() error { return s.db.Close() }

func dueVal(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format(dateLayout)
}

func tsVal(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339)
}

func parseNull(ns sql.NullString, layout string) (*time.Time, error) {
	if !ns.Valid {
		return nil, nil
	}
	t, err := time.ParseInLocation(layout, ns.String, time.Local)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

type scanner interface{ Scan(dest ...any) error }

func scanTask(sc scanner) (task.Task, error) {
	var (
		t                task.Task
		due, doneAt      sql.NullString
		created, updated string
		pri              int
	)
	if err := sc.Scan(&t.ID, &t.Title, &t.Project, &due, &pri, &doneAt, &created, &updated); err != nil {
		return task.Task{}, err
	}
	t.Priority = task.Priority(pri)
	var err error
	if t.Due, err = parseNull(due, dateLayout); err != nil {
		return task.Task{}, err
	}
	if t.DoneAt, err = parseNull(doneAt, time.RFC3339); err != nil {
		return task.Task{}, err
	}
	if t.CreatedAt, err = time.ParseInLocation(time.RFC3339, created, time.Local); err != nil {
		return task.Task{}, err
	}
	if t.UpdatedAt, err = time.ParseInLocation(time.RFC3339, updated, time.Local); err != nil {
		return task.Task{}, err
	}
	return t, nil
}

// setTags 覆寫一筆任務的標籤關聯。
func (s *sqlStore) setTags(id int64, tags []string) error {
	if _, err := s.db.Exec(`DELETE FROM task_tags WHERE task_id = ?`, id); err != nil {
		return err
	}
	for _, name := range task.NormalizeTags(tags) {
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO tags (name) VALUES (?)`, name); err != nil {
			return err
		}
		if _, err := s.db.Exec(
			`INSERT OR IGNORE INTO task_tags (task_id, tag_id) SELECT ?, id FROM tags WHERE name = ?`,
			id, name); err != nil {
			return err
		}
	}
	return nil
}

func (s *sqlStore) loadTags(id int64) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT g.name FROM task_tags tt JOIN tags g ON g.id = tt.tag_id WHERE tt.task_id = ? ORDER BY g.name`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func (s *sqlStore) Add(t task.Task) (task.Task, error) {
	res, err := s.db.Exec(
		`INSERT INTO tasks (title, project, due, priority, done_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.Title, t.Project, dueVal(t.Due), int(t.Priority), tsVal(t.DoneAt),
		t.CreatedAt.Format(time.RFC3339), t.UpdatedAt.Format(time.RFC3339))
	if err != nil {
		return task.Task{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return task.Task{}, err
	}
	t.ID = id
	t.Tags = task.NormalizeTags(t.Tags)
	if err := s.setTags(id, t.Tags); err != nil {
		return task.Task{}, err
	}
	return t, nil
}

func (s *sqlStore) Get(id int64) (task.Task, error) {
	row := s.db.QueryRow(`SELECT `+taskCols+` FROM tasks WHERE id = ?`, id)
	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return task.Task{}, ErrNotFound
	}
	if err != nil {
		return task.Task{}, err
	}
	if t.Tags, err = s.loadTags(id); err != nil {
		return task.Task{}, err
	}
	return t, nil
}

// 以下四個方法在 Task 6、Task 7 實作。
func (s *sqlStore) List(f task.Filter, now time.Time) ([]task.Task, error) {
	return nil, errors.New("尚未實作")
}
func (s *sqlStore) Update(t task.Task) error { return errors.New("尚未實作") }
func (s *sqlStore) Delete(id int64) error {
	res, err := s.db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *sqlStore) SetDone(id int64, done bool, now time.Time) error {
	return errors.New("尚未實作")
}
func (s *sqlStore) Restore(t task.Task) error         { return errors.New("尚未實作") }
func (s *sqlStore) Tags() ([]string, error)           { return nil, errors.New("尚未實作") }
func (s *sqlStore) Projects() ([]ProjectCount, error) { return nil, errors.New("尚未實作") }
```

- [ ] **Step 5: 執行測試確認通過**

Run: `go test ./internal/store/ -v`
Expected: PASS（五個 case 全綠）

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/store/
git commit --no-gpg-sign -m "feat(store): Store 介面與 SQLite schema、Add/Get/Delete"
```

---

### Task 6: 查詢與過濾

**Files:**
- Modify: `internal/store/sqlite.go`（取代 `List` 的骨架）
- Test: `internal/store/list_test.go`

**Interfaces:**
- Consumes: `task.Filter`、`task.SortBy`、`task.DueRange`、Task 5 的 `sqlStore`、`scanTask`、`loadTags`
- Produces: 可用的 `(*sqlStore).List(f task.Filter, now time.Time) ([]task.Task, error)`

- [ ] **Step 1: 寫失敗的測試**

建立 `internal/store/list_test.go`：

```go
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
	// 有期限者由近到遠，無期限者排最後。
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
```

- [ ] **Step 2: 執行測試確認失敗**

Run: `go test ./internal/store/ -run TestList`
Expected: FAIL，`尚未實作`

- [ ] **Step 3: 實作**

在 `internal/store/sqlite.go` 中，把 `List` 的骨架換成下面這段，並在 import 補上 `"fmt"` 與 `"strings"`：

```go
func (s *sqlStore) List(f task.Filter, now time.Time) ([]task.Task, error) {
	var where []string
	var args []any

	switch {
	case f.OnlyDone:
		where = append(where, `done_at IS NOT NULL`)
	case !f.IncludeDone:
		where = append(where, `done_at IS NULL`)
	}
	if f.Project != nil {
		where = append(where, `project = ?`)
		args = append(args, *f.Project)
	}
	if f.Priority != nil {
		where = append(where, `priority = ?`)
		args = append(args, int(*f.Priority))
	}
	if f.Search != "" {
		where = append(where, `LOWER(title) LIKE ?`)
		args = append(args, "%"+strings.ToLower(f.Search)+"%")
	}
	today := datearg.Day(now).Format(dateLayout)
	switch f.DueRange {
	case task.DueToday:
		where = append(where, `due = ?`)
		args = append(args, today)
	case task.DueOverdue:
		where = append(where, `due IS NOT NULL AND due < ?`)
		args = append(args, today)
	case task.DueWeek:
		where = append(where, `due IS NOT NULL AND due <= ?`)
		args = append(args, datearg.Day(now).AddDate(0, 0, 7).Format(dateLayout))
	case task.DueOn:
		where = append(where, `due = ?`)
		args = append(args, datearg.Day(f.DueOn).Format(dateLayout))
	}
	if tags := task.NormalizeTags(f.Tags); len(tags) > 0 {
		ph := strings.TrimSuffix(strings.Repeat("?,", len(tags)), ",")
		where = append(where, fmt.Sprintf(
			`(SELECT COUNT(DISTINCT g.name) FROM task_tags tt JOIN tags g ON g.id = tt.tag_id
			  WHERE tt.task_id = tasks.id AND g.name IN (%s)) = ?`, ph))
		for _, tg := range tags {
			args = append(args, tg)
		}
		args = append(args, len(tags))
	}

	// 無期限者一律排在有期限者之後：SQLite 中 (due IS NULL) 為 0/1，升冪即可。
	order := `(due IS NULL), due ASC, priority DESC, id ASC`
	switch f.Sort {
	case task.SortPriority:
		order = `priority DESC, (due IS NULL), due ASC, id ASC`
	case task.SortCreated:
		order = `created_at ASC, id ASC`
	}

	q := `SELECT ` + taskCols + ` FROM tasks`
	if len(where) > 0 {
		q += ` WHERE ` + strings.Join(where, ` AND `)
	}
	q += ` ORDER BY ` + order

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	var out []task.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	// 清單規模是個人待辦，逐筆載入標籤的 N+1 成本可忽略，換來的是簡單。
	for i := range out {
		tags, err := s.loadTags(out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Tags = tags
	}
	return out, nil
}
```

import 區塊改成：

```go
import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"todo.mirumo.net/internal/datearg"
	"todo.mirumo.net/internal/task"
)
```

- [ ] **Step 4: 執行測試確認通過**

Run: `go test ./internal/store/ -v`
Expected: 除 `TestListDoneVisibility`（需要 Task 7 的 `SetDone`）外全部 PASS

若 `TestListDoneVisibility` 因 `尚未實作` 失敗，先跳過：`go test ./internal/store/ -run 'TestList' -skip TestListDoneVisibility`，Task 7 完成後再全跑。

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit --no-gpg-sign -m "feat(store): 清單查詢、過濾與排序"
```

---

### Task 7: 變更操作與後設查詢

**Files:**
- Modify: `internal/store/sqlite.go`（取代 `Update`/`SetDone`/`Restore`/`Tags`/`Projects` 的骨架）
- Test: `internal/store/mutate_test.go`

**Interfaces:**
- Consumes: Task 5 與 Task 6 的全部
- Produces: 完整可用的 `Store` 實作

- [ ] **Step 1: 寫失敗的測試**

建立 `internal/store/mutate_test.go`：

```go
package store

import (
	"errors"
	"testing"

	"todo.mirumo.net/internal/task"
)

func TestSetDoneAndUndone(t *testing.T) {
	s := newStore(t)
	got, _ := s.Add(sample())
	if err := s.SetDone(got.ID, true, ref()); err != nil {
		t.Fatalf("SetDone：%v", err)
	}
	back, _ := s.Get(got.ID)
	if !back.Done() {
		t.Fatal("應為已完成")
	}
	if back.DoneAt.Format("2006-01-02") != "2026-08-29" {
		t.Errorf("done_at = %v，預期記錄完成時間", back.DoneAt)
	}
	if err := s.SetDone(got.ID, false, ref()); err != nil {
		t.Fatal(err)
	}
	back, _ = s.Get(got.ID)
	if back.Done() {
		t.Error("取消完成後 DoneAt 應為 nil")
	}
}

func TestSetDoneMissingID(t *testing.T) {
	s := newStore(t)
	if err := s.SetDone(42, true, ref()); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v，預期 ErrNotFound", err)
	}
}

func TestUpdateOverwritesFieldsAndTags(t *testing.T) {
	s := newStore(t)
	got, _ := s.Add(sample())
	got.Title = "買豆漿"
	got.Project = ""
	got.Due = nil
	got.Priority = task.PriLow
	got.Tags = []string{"早餐"}
	if err := s.Update(got); err != nil {
		t.Fatalf("Update：%v", err)
	}
	back, _ := s.Get(got.ID)
	if back.Title != "買豆漿" || back.Project != "" || back.Due != nil || back.Priority != task.PriLow {
		t.Errorf("欄位沒更新：%+v", back)
	}
	if len(back.Tags) != 1 || back.Tags[0] != "早餐" {
		t.Errorf("tags = %v，預期整組被取代成 [早餐]", back.Tags)
	}
}

func TestUpdateMissingID(t *testing.T) {
	s := newStore(t)
	if err := s.Update(task.Task{ID: 7, Title: "x", CreatedAt: ref(), UpdatedAt: ref()}); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v，預期 ErrNotFound", err)
	}
}

func TestDeleteRemovesTagLinks(t *testing.T) {
	s := newStore(t)
	got, _ := s.Add(sample())
	if err := s.Delete(got.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(got.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v，預期 ErrNotFound", err)
	}
	// 標籤本身留著無妨，但不該再被任何任務引用。
	tags, err := s.Tags()
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 0 {
		t.Errorf("Tags() = %v，預期空：只列被引用的標籤", tags)
	}
}

func TestRestoreReusesOriginalID(t *testing.T) {
	s := newStore(t)
	got, _ := s.Add(sample())
	original, _ := s.Get(got.ID)
	if err := s.Delete(got.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Restore(original); err != nil {
		t.Fatalf("Restore：%v", err)
	}
	back, err := s.Get(original.ID)
	if err != nil {
		t.Fatalf("復原後應能用原 id 取回：%v", err)
	}
	if back.Title != original.Title || len(back.Tags) != len(original.Tags) {
		t.Errorf("復原內容不符：%+v", back)
	}
}

func TestTagsListsOnlyReferenced(t *testing.T) {
	s := newStore(t)
	seed(t, s)
	tags, err := s.Tags()
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 || tags[0] != "急" || tags[1] != "雜" {
		t.Errorf("= %v，預期 [急 雜] 依名稱排序", tags)
	}
}

func TestProjectsCountsOpenTasks(t *testing.T) {
	s := newStore(t)
	ids := seed(t, s)
	if err := s.SetDone(ids["工作上的事"], true, ref()); err != nil {
		t.Fatal(err)
	}
	ps, err := s.Projects()
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 2 {
		t.Fatalf("= %+v，預期兩個專案（含空字串的未分類）", ps)
	}
	if ps[0].Path != "" || ps[0].Open != 3 {
		t.Errorf("ps[0] = %+v，預期未分類 3 筆未完成", ps[0])
	}
	if ps[1].Path != "/p/work" || ps[1].Open != 1 {
		t.Errorf("ps[1] = %+v，預期 /p/work 1 筆未完成（另一筆已完成）", ps[1])
	}
}
```

- [ ] **Step 2: 執行測試確認失敗**

Run: `go test ./internal/store/ -run 'TestSetDone|TestUpdate|TestRestore|TestTags|TestProjects'`
Expected: FAIL，`尚未實作`

- [ ] **Step 3: 實作**

在 `internal/store/sqlite.go` 把五個骨架方法換成：

```go
func (s *sqlStore) Update(t task.Task) error {
	res, err := s.db.Exec(
		`UPDATE tasks SET title = ?, project = ?, due = ?, priority = ?, done_at = ?, updated_at = ?
		 WHERE id = ?`,
		t.Title, t.Project, dueVal(t.Due), int(t.Priority), tsVal(t.DoneAt),
		t.UpdatedAt.Format(time.RFC3339), t.ID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return s.setTags(t.ID, t.Tags)
}

func (s *sqlStore) SetDone(id int64, done bool, now time.Time) error {
	var doneAt any
	if done {
		doneAt = now.Format(time.RFC3339)
	}
	res, err := s.db.Exec(
		`UPDATE tasks SET done_at = ?, updated_at = ? WHERE id = ?`,
		doneAt, now.Format(time.RFC3339), id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Restore 以原 id 重新插入。AUTOINCREMENT 不重用號碼，該 id 必定仍空著。
func (s *sqlStore) Restore(t task.Task) error {
	_, err := s.db.Exec(
		`INSERT INTO tasks (id, title, project, due, priority, done_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Title, t.Project, dueVal(t.Due), int(t.Priority), tsVal(t.DoneAt),
		t.CreatedAt.Format(time.RFC3339), t.UpdatedAt.Format(time.RFC3339))
	if err != nil {
		return err
	}
	return s.setTags(t.ID, t.Tags)
}

// Tags 只列出至少被一個任務引用的標籤；刪除任務留下的孤兒標籤不清理也不顯示。
func (s *sqlStore) Tags() ([]string, error) {
	rows, err := s.db.Query(
		`SELECT DISTINCT g.name FROM tags g JOIN task_tags tt ON tt.tag_id = g.id ORDER BY g.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func (s *sqlStore) Projects() ([]ProjectCount, error) {
	rows, err := s.db.Query(
		`SELECT project, SUM(CASE WHEN done_at IS NULL THEN 1 ELSE 0 END)
		 FROM tasks GROUP BY project ORDER BY project`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProjectCount
	for rows.Next() {
		var p ProjectCount
		if err := rows.Scan(&p.Path, &p.Open); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: 執行整包測試確認通過**

Run: `go test ./... -v`
Expected: PASS，`internal/store` 全部 case 綠燈（含 Task 5 的 `TestIDsAreNotReused` 與 Task 6 的 `TestListDoneVisibility`）

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit --no-gpg-sign -m "feat(store): 更新、完成切換、復原與後設查詢"
```

---

### Task 8: CLI 骨架、dispatch 與 `todo tui`

**Files:**
- Create: `internal/cli/app.go`
- Test: `internal/cli/app_test.go`
- Test: `internal/cli/helper_test.go`

**Interfaces:**
- Consumes: `store.Store`、`argparse.Result`、`project.Current`
- Produces: `cli.App{Store store.Store; Out, Err io.Writer; Now func() time.Time; Cwd string; Color bool; RunTUI func() error}`、`(*App).Run(args []string) int`、`cli.SplitGlobal(args []string) (dbPath string, rest []string, err error)`、內部輔助 `(*App).commands()`、`parseIDs([]string) ([]int64, error)`、`(*App).resolveProject(*argparse.Result) (string, bool, error)`

**測試用 Store 的選擇**：spec 寫「假 Store」，實作上改用 `store.OpenSQLite(":memory:")`。理由是它更短、跑真實 SQL、仍然完全隔離（不碰檔案系統），維護成本比手寫 fake 低。這是對 spec 的刻意收斂，不是遺漏。

- [ ] **Step 1: 寫失敗的測試**

建立 `internal/cli/helper_test.go`：

```go
package cli

import (
	"bytes"
	"testing"
	"time"

	"todo.mirumo.net/internal/store"
)

func refTime() time.Time { return time.Date(2026, 8, 29, 15, 0, 0, 0, time.Local) }

// newApp 建一個完全隔離的 App：in-memory 資料庫、緩衝輸出、固定時鐘、暫存目錄當 cwd。
func newApp(t *testing.T) (*App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	st, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite：%v", err)
	}
	t.Cleanup(func() { st.Close() })
	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	app := &App{
		Store: st, Out: out, Err: errBuf,
		Now: refTime, Cwd: t.TempDir(), Color: false,
	}
	return app, out, errBuf
}
```

建立 `internal/cli/app_test.go`：

```go
package cli

import (
	"errors"
	"strings"
	"testing"
)

func TestRunNoArgsPrintsUsage(t *testing.T) {
	app, out, _ := newApp(t)
	if code := app.Run(nil); code != 0 {
		t.Errorf("離開碼 = %d，預期 0", code)
	}
	if !strings.Contains(out.String(), "用法：") {
		t.Errorf("裸打 todo 應印出用法，實得：%q", out.String())
	}
	if strings.Contains(out.String(), "沒有符合的待辦") {
		t.Error("裸打 todo 不該進入清單或 TUI")
	}
}

func TestRunHelp(t *testing.T) {
	for _, arg := range []string{"-h", "--help", "help"} {
		app, out, _ := newApp(t)
		if code := app.Run([]string{arg}); code != 0 {
			t.Errorf("%s 離開碼 = %d，預期 0", arg, code)
		}
		if !strings.Contains(out.String(), "用法：") {
			t.Errorf("%s 沒印出用法", arg)
		}
	}
}

func TestHelpFlagWorksAfterSubcommand(t *testing.T) {
	// 子指令的 flag 集不認得 -h，必須在 dispatch 之前攔下來。
	app, out, errBuf := newApp(t)
	if code := app.Run([]string{"add", "-h"}); code != 0 {
		t.Errorf("離開碼 = %d，預期 0；stderr = %q", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "用法：") {
		t.Errorf("todo add -h 應印出用法，實得：%q", out.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	app, _, errBuf := newApp(t)
	if code := app.Run([]string{"frobnicate"}); code != 2 {
		t.Errorf("離開碼 = %d，預期 2", code)
	}
	if !strings.Contains(errBuf.String(), "未知的指令：frobnicate") {
		t.Errorf("stderr = %q", errBuf.String())
	}
}

func TestTUIOnlyOnExplicitSubcommand(t *testing.T) {
	app, _, _ := newApp(t)
	called := false
	app.RunTUI = func() error { called = true; return nil }

	if code := app.Run(nil); code != 0 || called {
		t.Error("裸打 todo 不該啟動 TUI")
	}
	if code := app.Run([]string{"tui"}); code != 0 {
		t.Errorf("離開碼 = %d，預期 0", code)
	}
	if !called {
		t.Error("todo tui 應該啟動 TUI")
	}
}

func TestTUIErrorBecomesExitCode1(t *testing.T) {
	app, _, errBuf := newApp(t)
	app.RunTUI = func() error { return errors.New("終端機壞了") }
	if code := app.Run([]string{"tui"}); code != 1 {
		t.Errorf("離開碼 = %d，預期 1", code)
	}
	if !strings.Contains(errBuf.String(), "終端機壞了") {
		t.Errorf("stderr = %q", errBuf.String())
	}
}

func TestSplitGlobal(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		wantDB string
		wantRest []string
	}{
		{"沒有 --db", []string{"ls", "-a"}, "", []string{"ls", "-a"}},
		{"空格式", []string{"--db", "/tmp/x.db", "ls"}, "/tmp/x.db", []string{"ls"}},
		{"等號式", []string{"--db=/tmp/x.db", "ls"}, "/tmp/x.db", []string{"ls"}},
		{"只有 --db", []string{"--db=/tmp/x.db"}, "/tmp/x.db", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db, rest, err := SplitGlobal(c.args)
			if err != nil {
				t.Fatalf("非預期錯誤：%v", err)
			}
			if db != c.wantDB {
				t.Errorf("db = %q，預期 %q", db, c.wantDB)
			}
			if strings.Join(rest, " ") != strings.Join(c.wantRest, " ") {
				t.Errorf("rest = %v，預期 %v", rest, c.wantRest)
			}
		})
	}
	if _, _, err := SplitGlobal([]string{"--db"}); err == nil {
		t.Error("--db 缺值應該報錯")
	}
}

func TestParseIDs(t *testing.T) {
	got, err := parseIDs([]string{"3", "17"})
	if err != nil || len(got) != 2 || got[0] != 3 || got[1] != 17 {
		t.Errorf("= %v, %v", got, err)
	}
	for _, bad := range [][]string{{}, {"x"}, {"0"}, {"-1"}} {
		if _, err := parseIDs(bad); err == nil {
			t.Errorf("parseIDs(%v) 應該報錯", bad)
		}
	}
}
```

- [ ] **Step 2: 執行測試確認失敗**

Run: `go test ./internal/cli/`
Expected: FAIL，`undefined: App`

- [ ] **Step 3: 實作**

建立 `internal/cli/app.go`：

```go
// Package cli 實作 todo 的命令列介面。
package cli

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"todo.mirumo.net/internal/argparse"
	"todo.mirumo.net/internal/project"
	"todo.mirumo.net/internal/store"
)

// App 持有一次執行所需的全部依賴。全部可注入，測試才能完全隔離。
type App struct {
	Store store.Store
	Out   io.Writer
	Err   io.Writer
	Now   func() time.Time
	Cwd   string
	Color bool
	// RunTUI 由 cmd/todo 注入。cli 不 import tui，兩者保持平行。
	RunTUI func() error
}

const usageText = `todo — 本機待辦事項

用法：
  todo <指令> [flags]

指令：
  add <標題>      新增一條待辦
  ls              列出待辦
  done <id>...    標記完成
  undone <id>...  取消完成
  edit <id>       修改欄位
  rm <id>...      刪除
  projects        列出專案與各自未完成數
  tags            列出標籤
  tui             進入互動介面

全域 flag：
  --db <路徑>     指定資料庫（預設 ~/.todo/todo.db，或環境變數 TODO_DB）
  -h, --help      顯示這份說明
`

// commands 是子指令表。每加一個子指令就在這裡登記一行。
func (a *App) commands() map[string]func([]string) error {
	return map[string]func([]string) error{
		"tui": a.cmdTUI,
	}
}

// Run 執行一次命令，回傳行程離開碼：0 成功、1 執行失敗、2 用法錯誤。
func (a *App) Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(a.Out, usageText)
		return 0
	}
	name, rest := args[0], args[1:]
	// -h 放在哪都算。子指令的 flag 集不認得 -h，
	// 不先攔下來的話 todo add -h 會變成「未知的 flag」，那是很差的體驗。
	for _, a2 := range args {
		if a2 == "-h" || a2 == "--help" {
			fmt.Fprint(a.Out, usageText)
			return 0
		}
	}
	if name == "help" {
		fmt.Fprint(a.Out, usageText)
		return 0
	}
	cmd, ok := a.commands()[name]
	if !ok {
		fmt.Fprintf(a.Err, "未知的指令：%s\n\n", name)
		fmt.Fprint(a.Err, usageText)
		return 2
	}
	if err := cmd(rest); err != nil {
		fmt.Fprintf(a.Err, "錯誤：%s\n", err)
		return 1
	}
	return 0
}

func (a *App) cmdTUI(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("todo tui 不接受參數，收到 %q", args[0])
	}
	if a.RunTUI == nil {
		return errors.New("這個組建沒有啟用 TUI")
	}
	return a.RunTUI()
}

// SplitGlobal 取出開頭連續的 --db，回傳剩下的參數。
// 只掃描開頭：--db 是全域 flag，位置固定才不會與子指令的 flag 混淆。
func SplitGlobal(args []string) (dbPath string, rest []string, err error) {
	for len(args) > 0 {
		switch a := args[0]; {
		case a == "--db":
			if len(args) < 2 {
				return "", nil, errors.New("flag --db 需要一個值")
			}
			dbPath, args = args[1], args[2:]
		case strings.HasPrefix(a, "--db="):
			dbPath, args = strings.TrimPrefix(a, "--db="), args[1:]
		default:
			return dbPath, args, nil
		}
	}
	return dbPath, nil, nil
}

func parseIDs(args []string) ([]int64, error) {
	if len(args) == 0 {
		return nil, errors.New("需要至少一個 id")
	}
	ids := make([]int64, 0, len(args))
	for _, a := range args {
		n, err := strconv.ParseInt(a, 10, 64)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("不是合法的 id：%q", a)
		}
		ids = append(ids, n)
	}
	return ids, nil
}

// resolveProject 判讀 -p 的三態。
// 回傳的 bool 代表「要不要動 project 欄位」，字串才是新值（可為空字串）。
func (a *App) resolveProject(r *argparse.Result) (string, bool, error) {
	if !r.Changed("project") {
		return "", false, nil
	}
	if v, has := r.Optional("project"); has {
		return v, true, nil
	}
	p, err := project.Current(a.Cwd)
	if err != nil {
		return "", false, err
	}
	return p, true, nil
}
```

- [ ] **Step 4: 執行測試確認通過**

Run: `go test ./internal/cli/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/
git commit --no-gpg-sign -m "feat(cli): App 骨架、指令 dispatch 與 todo tui"
```

---

### Task 9: `todo add`

**Files:**
- Create: `internal/cli/cmd_add.go`
- Modify: `internal/cli/app.go`（在 `commands()` 加一行）
- Test: `internal/cli/cmd_add_test.go`

**Interfaces:**
- Consumes: `(*App).resolveProject`、`argparse`、`task`、`datearg`
- Produces: `(*App).cmdAdd([]string) error`、`addFlags() *argparse.Set`（`edit` 會重用）

- [ ] **Step 1: 寫失敗的測試**

建立 `internal/cli/cmd_add_test.go`：

```go
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"todo.mirumo.net/internal/task"
)

func TestAddMinimal(t *testing.T) {
	app, out, _ := newApp(t)
	if code := app.Run([]string{"add", "  買牛奶  "}); code != 0 {
		t.Fatalf("離開碼 = %d", code)
	}
	if !strings.Contains(out.String(), "已新增 #1：買牛奶") {
		t.Errorf("stdout = %q", out.String())
	}
	got, err := app.Store.Get(1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "買牛奶" {
		t.Errorf("標題 = %q，預期去掉頭尾空白", got.Title)
	}
	if got.Project != "" {
		t.Errorf("project = %q，沒給 -p 就該是全域未分類", got.Project)
	}
}

func TestAddAllFlags(t *testing.T) {
	app, _, _ := newApp(t)
	code := app.Run([]string{"add", "買牛奶", "-t", "購物", "--tag=家務", "-d", "tomorrow", "--pri", "high"})
	if code != 0 {
		t.Fatalf("離開碼 = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.Due == nil || got.Due.Format("2006-01-02") != "2026-08-30" {
		t.Errorf("due = %v，預期 2026-08-30", got.Due)
	}
	if got.Priority != task.PriHigh {
		t.Errorf("priority = %v", got.Priority)
	}
	if len(got.Tags) != 2 {
		t.Errorf("tags = %v", got.Tags)
	}
}

func TestAddProjectFromCwd(t *testing.T) {
	app, _, _ := newApp(t)
	root := app.Cwd
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if code := app.Run([]string{"add", "修 bug", "-p"}); code != 0 {
		t.Fatalf("離開碼 = %d", code)
	}
	got, _ := app.Store.Get(1)
	want, _ := filepath.EvalSymlinks(root)
	gotResolved, _ := filepath.EvalSymlinks(got.Project)
	if gotResolved != want {
		t.Errorf("project = %q，預期當前 repo 根 %q", gotResolved, want)
	}
}

func TestAddProjectExplicitName(t *testing.T) {
	app, _, _ := newApp(t)
	if code := app.Run([]string{"add", "修 bug", "-p", "work"}); code != 0 {
		t.Fatalf("離開碼 = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.Project != "work" {
		t.Errorf("project = %q，預期 work", got.Project)
	}
}

func TestAddMissingTitleExplainsTheFootgun(t *testing.T) {
	app, _, errBuf := newApp(t)
	// -p 吃掉了本該是標題的參數。
	if code := app.Run([]string{"add", "-p", "買牛奶"}); code != 1 {
		t.Fatalf("離開碼 = %d，預期 1", code)
	}
	msg := errBuf.String()
	if !strings.Contains(msg, "缺少標題") || !strings.Contains(msg, "--project=買牛奶") {
		t.Errorf("錯誤訊息應指出被 -p 吃掉並給出修正寫法，實得：%q", msg)
	}
}

func TestAddRejectsBadValues(t *testing.T) {
	cases := [][]string{
		{"add", "x", "--pri", "urgent"},
		{"add", "x", "-d", "someday"},
		{"add", "   "},
		{"add", "a", "b"},
	}
	for _, args := range cases {
		app, _, errBuf := newApp(t)
		if code := app.Run(args); code == 0 {
			t.Errorf("%v 應該失敗", args)
		}
		if errBuf.Len() == 0 {
			t.Errorf("%v 應該印出錯誤訊息", args)
		}
	}
}
```

- [ ] **Step 2: 執行測試確認失敗**

Run: `go test ./internal/cli/ -run TestAdd`
Expected: FAIL，`未知的指令：add`

- [ ] **Step 3: 實作**

建立 `internal/cli/cmd_add.go`：

```go
package cli

import (
	"errors"
	"fmt"
	"strings"

	"todo.mirumo.net/internal/argparse"
	"todo.mirumo.net/internal/datearg"
	"todo.mirumo.net/internal/task"
)

// addFlags 是 add 與 edit 共用的欄位 flag。
func addFlags() *argparse.Set {
	return argparse.New(
		argparse.Spec{Long: "project", Short: "p", Kind: argparse.OptionalString, Usage: "專案；不給值時用當前目錄"},
		argparse.Spec{Long: "tag", Short: "t", Kind: argparse.StringSlice, Usage: "標籤，可重複"},
		argparse.Spec{Long: "due", Short: "d", Kind: argparse.String, Usage: "截止日：tomorrow、fri、+3d、2026-09-01"},
		argparse.Spec{Long: "pri", Kind: argparse.String, Usage: "優先度：low、med、high"},
	)
}

func (a *App) cmdAdd(args []string) error {
	r, err := addFlags().Parse(args)
	if err != nil {
		return err
	}
	pos := r.Args()
	if len(pos) == 0 {
		// 最常見的誤用：todo add -p "買牛奶"，標題被 -p 吃掉了。
		if v, has := r.Optional("project"); has {
			return fmt.Errorf("缺少標題；看起來 %q 被當成 --project 的值。標題請放在 flag 前面（todo add %q -p），或用 --project=%s", v, v, v)
		}
		return errors.New("用法：todo add <標題> [flags]")
	}
	if len(pos) > 1 {
		return fmt.Errorf("只能有一個標題，收到 %d 個位置參數；含空白的標題請用引號包起來", len(pos))
	}
	title, err := task.ValidateTitle(pos[0])
	if err != nil {
		return err
	}

	now := a.Now()
	t := task.Task{
		Title:     title,
		Tags:      task.NormalizeTags(r.Strings("tag")),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if p, ok, err := a.resolveProject(r); err != nil {
		return err
	} else if ok {
		t.Project = p
	}
	if r.Changed("due") && strings.TrimSpace(r.String("due")) != "" {
		d, err := datearg.Parse(r.String("due"), now)
		if err != nil {
			return err
		}
		t.Due = &d
	}
	if t.Priority, err = task.ParsePriority(r.String("pri")); err != nil {
		return err
	}

	got, err := a.Store.Add(t)
	if err != nil {
		return err
	}
	fmt.Fprintf(a.Out, "已新增 #%d：%s\n", got.ID, got.Title)
	return nil
}
```

在 `internal/cli/app.go` 的 `commands()` 加入 `add`：

```go
	return map[string]func([]string) error{
		"add": a.cmdAdd,
		"tui": a.cmdTUI,
	}
```

- [ ] **Step 4: 執行測試確認通過**

Run: `go test ./internal/cli/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/
git commit --no-gpg-sign -m "feat(cli): todo add"
```

---

### Task 10: `todo ls` 與清單排版

**Files:**
- Create: `internal/cli/format.go`
- Create: `internal/cli/cmd_ls.go`
- Modify: `internal/cli/app.go`（`commands()` 加一行）
- Test: `internal/cli/format_test.go`
- Test: `internal/cli/cmd_ls_test.go`

**Interfaces:**
- Consumes: `store.Store.List`、`task.Filter`、`datearg.Format`、`project.Label`
- Produces: `cli.WriteList(w io.Writer, ts []task.Task, now time.Time, color bool)`、內部 `pad(string, int) string`、`(*App).cmdLs([]string) error`

- [ ] **Step 1: 加入 Lip Gloss 依賴**

```bash
go get github.com/charmbracelet/lipgloss@v1
```

只用它的 `lipgloss.Width`：中文字在終端機佔兩格，`len()` 與 `text/tabwriter` 都會算錯而導致欄位跑掉。

- [ ] **Step 2: 寫失敗的測試**

建立 `internal/cli/format_test.go`：

```go
package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"todo.mirumo.net/internal/task"
)

func day(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	return &t
}

func TestPadUsesDisplayWidth(t *testing.T) {
	// "買牛奶" 是 3 個 rune、9 個 byte、6 格寬。
	if got := pad("買牛奶", 8); got != "買牛奶  " {
		t.Errorf("= %q，預期補到 8 格寬（兩個空白）", got)
	}
	if got := pad("abc", 2); got != "abc" {
		t.Errorf("= %q，超過寬度時應原樣回傳", got)
	}
}

func TestWriteListAlignsColumns(t *testing.T) {
	now := time.Date(2026, 8, 29, 15, 0, 0, 0, time.Local)
	ts := []task.Task{
		{ID: 1, Title: "買牛奶", Due: day(2026, 8, 29), Priority: task.PriHigh, Tags: []string{"購物"}},
		{ID: 12, Title: "繳房租", Project: "/p/home"},
	}
	var buf bytes.Buffer
	WriteList(&buf, ts, now, false)
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("預期兩行，實得 %d 行：%q", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[0], "1  [ ] !高 今天 買牛奶 ") {
		t.Errorf("第一行 = %q", lines[0])
	}
	if !strings.Contains(lines[0], "@購物") {
		t.Errorf("第一行少了標籤：%q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "12 [ ] ") {
		t.Errorf("第二行 = %q，id 欄應對齊到最寬的 id", lines[1])
	}
	if !strings.HasSuffix(lines[1], "home") {
		t.Errorf("第二行應以專案 basename 收尾：%q", lines[1])
	}
	for _, l := range lines {
		if strings.HasSuffix(l, " ") {
			t.Errorf("不該有尾隨空白：%q", l)
		}
		if strings.Contains(l, "\x1b[") {
			t.Errorf("color=false 時不該輸出 ANSI 碼：%q", l)
		}
	}
}

func TestWriteListEmpty(t *testing.T) {
	var buf bytes.Buffer
	WriteList(&buf, nil, time.Now(), false)
	if !strings.Contains(buf.String(), "沒有符合的待辦") {
		t.Errorf("= %q", buf.String())
	}
}

func TestWriteListColorMarksOverdueAndDone(t *testing.T) {
	now := time.Date(2026, 8, 29, 15, 0, 0, 0, time.Local)
	done := now
	ts := []task.Task{
		{ID: 1, Title: "逾期", Due: day(2026, 8, 1)},
		{ID: 2, Title: "完成", DoneAt: &done},
	}
	var buf bytes.Buffer
	WriteList(&buf, ts, now, true)
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if !strings.HasPrefix(lines[0], "\x1b[31m") {
		t.Errorf("逾期該是紅色：%q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "\x1b[2m") {
		t.Errorf("已完成該是暗色：%q", lines[1])
	}
}
```

建立 `internal/cli/cmd_ls_test.go`：

```go
package cli

import (
	"strings"
	"testing"
)

// 先塞幾筆資料再測 ls。
func seedCLI(t *testing.T, app *App) {
	t.Helper()
	cases := [][]string{
		{"add", "逾期的事", "-d", "2026-08-20"},
		{"add", "今天的事", "-d", "today", "--pri", "high", "-t", "急"},
		{"add", "工作的事", "-p", "work", "-t", "雜"},
		{"add", "沒期限的事"},
	}
	for _, args := range cases {
		if code := app.Run(args); code != 0 {
			t.Fatalf("%v 失敗", args)
		}
	}
}

func TestLsDefaultsToOpenOnly(t *testing.T) {
	app, out, _ := newApp(t)
	seedCLI(t, app)
	if code := app.Run([]string{"done", "1"}); code != 0 {
		t.Skip("done 尚未實作，Task 11 後再跑")
	}
	out.Reset()
	app.Run([]string{"ls"})
	if strings.Contains(out.String(), "逾期的事") {
		t.Errorf("預設不該列已完成：%q", out.String())
	}
	out.Reset()
	app.Run([]string{"ls", "-a"})
	if !strings.Contains(out.String(), "逾期的事") {
		t.Errorf("-a 應含已完成：%q", out.String())
	}
}

func TestLsFilterByProjectAndNoProject(t *testing.T) {
	app, out, _ := newApp(t)
	seedCLI(t, app)

	out.Reset()
	app.Run([]string{"ls", "-p", "work"})
	if !strings.Contains(out.String(), "工作的事") || strings.Contains(out.String(), "今天的事") {
		t.Errorf("-p work = %q", out.String())
	}

	out.Reset()
	app.Run([]string{"ls", "--no-project"})
	if strings.Contains(out.String(), "工作的事") {
		t.Errorf("--no-project 不該含有專案的項目：%q", out.String())
	}
	if !strings.Contains(out.String(), "今天的事") {
		t.Errorf("--no-project 應含未分類項目：%q", out.String())
	}
}

func TestLsRejectsConflictingProjectFlags(t *testing.T) {
	app, _, errBuf := newApp(t)
	if code := app.Run([]string{"ls", "-p", "work", "--no-project"}); code != 1 {
		t.Errorf("離開碼 = %d，預期 1", code)
	}
	if !strings.Contains(errBuf.String(), "不能同時使用") {
		t.Errorf("stderr = %q", errBuf.String())
	}
}

func TestLsDueKeywordsAndTags(t *testing.T) {
	app, out, _ := newApp(t)
	seedCLI(t, app)

	out.Reset()
	app.Run([]string{"ls", "-d", "today"})
	if !strings.Contains(out.String(), "今天的事") || strings.Contains(out.String(), "逾期的事") {
		t.Errorf("-d today = %q", out.String())
	}

	out.Reset()
	app.Run([]string{"ls", "-d", "overdue"})
	if !strings.Contains(out.String(), "逾期的事") {
		t.Errorf("-d overdue = %q", out.String())
	}

	out.Reset()
	app.Run([]string{"ls", "-t", "急"})
	if !strings.Contains(out.String(), "今天的事") || strings.Contains(out.String(), "工作的事") {
		t.Errorf("-t 急 = %q", out.String())
	}
}

func TestLsRejectsPositionalArgs(t *testing.T) {
	app, _, errBuf := newApp(t)
	if code := app.Run([]string{"ls", "垃圾"}); code != 1 {
		t.Errorf("離開碼 = %d，預期 1", code)
	}
	if !strings.Contains(errBuf.String(), "不接受位置參數") {
		t.Errorf("stderr = %q", errBuf.String())
	}
}

func TestLsBadSortAndPriority(t *testing.T) {
	app, _, _ := newApp(t)
	if code := app.Run([]string{"ls", "-s", "title"}); code == 0 {
		t.Error("未知的排序應該失敗")
	}
	if code := app.Run([]string{"ls", "--pri", "urgent"}); code == 0 {
		t.Error("未知的優先度應該失敗")
	}
}
```

- [ ] **Step 3: 執行測試確認失敗**

Run: `go test ./internal/cli/ -run 'TestPad|TestWriteList|TestLs'`
Expected: FAIL，`undefined: pad`

- [ ] **Step 4: 實作排版**

建立 `internal/cli/format.go`：

```go
package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"todo.mirumo.net/internal/datearg"
	"todo.mirumo.net/internal/project"
	"todo.mirumo.net/internal/task"
)

const (
	ansiReset  = "\x1b[0m"
	ansiDim    = "\x1b[2m"
	ansiRed    = "\x1b[31m"
	ansiYellow = "\x1b[33m"
)

// pad 依終端顯示寬度補空白。中文字佔兩格，len() 與 text/tabwriter 都會算錯。
func pad(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

type row struct{ id, status, pri, due, title, proj, tags string }

func toRow(t task.Task, now time.Time) row {
	r := row{
		id:     fmt.Sprintf("%d", t.ID),
		status: "[ ]",
		title:  t.Title,
		proj:   project.Label(t.Project),
	}
	if t.Done() {
		r.status = "[x]"
	}
	if p := t.Priority.Label(); p != "" {
		r.pri = "!" + p
	}
	if t.Due != nil {
		r.due = datearg.Format(*t.Due, now)
	}
	if len(t.Tags) > 0 {
		r.tags = "@" + strings.Join(t.Tags, " @")
	}
	return r
}

func colorize(line string, t task.Task, now time.Time) string {
	switch {
	case t.Done():
		return ansiDim + line + ansiReset
	case t.Due != nil && datearg.Day(*t.Due).Before(datearg.Day(now)):
		return ansiRed + line + ansiReset
	case t.Priority == task.PriHigh:
		return ansiYellow + line + ansiReset
	}
	return line
}

// WriteList 輸出對齊的待辦清單。color 為 false 時完全不輸出 ANSI 碼。
func WriteList(w io.Writer, ts []task.Task, now time.Time, color bool) {
	if len(ts) == 0 {
		fmt.Fprintln(w, "沒有符合的待辦")
		return
	}
	rows := make([]row, len(ts))
	var wID, wPri, wDue, wTitle, wProj int
	for i, t := range ts {
		rows[i] = toRow(t, now)
		wID = max(wID, lipgloss.Width(rows[i].id))
		wPri = max(wPri, lipgloss.Width(rows[i].pri))
		wDue = max(wDue, lipgloss.Width(rows[i].due))
		wTitle = max(wTitle, lipgloss.Width(rows[i].title))
		wProj = max(wProj, lipgloss.Width(rows[i].proj))
	}
	for i, r := range rows {
		line := strings.TrimRight(strings.Join([]string{
			pad(r.id, wID), r.status, pad(r.pri, wPri), pad(r.due, wDue),
			pad(r.title, wTitle), pad(r.proj, wProj), r.tags,
		}, " "), " ")
		if color {
			line = colorize(line, ts[i], now)
		}
		fmt.Fprintln(w, line)
	}
}
```

- [ ] **Step 5: 實作 ls**

建立 `internal/cli/cmd_ls.go`：

```go
package cli

import (
	"errors"
	"fmt"
	"strings"

	"todo.mirumo.net/internal/argparse"
	"todo.mirumo.net/internal/datearg"
	"todo.mirumo.net/internal/task"
)

func (a *App) cmdLs(args []string) error {
	set := argparse.New(
		argparse.Spec{Long: "project", Short: "p", Kind: argparse.OptionalString, Usage: "只看某專案；不給值時用當前目錄"},
		argparse.Spec{Long: "no-project", Kind: argparse.Bool, Usage: "只看全域未分類"},
		argparse.Spec{Long: "tag", Short: "t", Kind: argparse.StringSlice, Usage: "標籤，可重複，取交集"},
		argparse.Spec{Long: "due", Short: "d", Kind: argparse.String, Usage: "today、week、overdue 或一個日期"},
		argparse.Spec{Long: "pri", Kind: argparse.String, Usage: "優先度：low、med、high"},
		argparse.Spec{Long: "all", Short: "a", Kind: argparse.Bool, Usage: "含已完成"},
		argparse.Spec{Long: "done", Kind: argparse.Bool, Usage: "只看已完成"},
		argparse.Spec{Long: "sort", Short: "s", Kind: argparse.String, Usage: "排序：due、pri、created"},
	)
	r, err := set.Parse(args)
	if err != nil {
		return err
	}
	if pos := r.Args(); len(pos) > 0 {
		return fmt.Errorf("ls 不接受位置參數，收到 %q；要搜尋標題請用 todo tui 的 / 鍵", pos[0])
	}

	f := task.Filter{
		IncludeDone: r.Bool("all"),
		OnlyDone:    r.Bool("done"),
		Tags:        r.Strings("tag"),
	}
	if r.Bool("no-project") && r.Changed("project") {
		return errors.New("-p 與 --no-project 不能同時使用")
	}
	if r.Bool("no-project") {
		empty := ""
		f.Project = &empty
	} else if p, ok, err := a.resolveProject(r); err != nil {
		return err
	} else if ok {
		f.Project = &p
	}
	if r.Changed("pri") {
		p, err := task.ParsePriority(r.String("pri"))
		if err != nil {
			return err
		}
		f.Priority = &p
	}
	if r.Changed("due") {
		switch v := strings.ToLower(strings.TrimSpace(r.String("due"))); v {
		case "today":
			f.DueRange = task.DueToday
		case "week":
			f.DueRange = task.DueWeek
		case "overdue":
			f.DueRange = task.DueOverdue
		default:
			d, err := datearg.Parse(v, a.Now())
			if err != nil {
				return err
			}
			f.DueRange, f.DueOn = task.DueOn, d
		}
	}
	if f.Sort, err = task.ParseSortBy(r.String("sort")); err != nil {
		return err
	}

	ts, err := a.Store.List(f, a.Now())
	if err != nil {
		return err
	}
	WriteList(a.Out, ts, a.Now(), a.Color)
	return nil
}
```

在 `commands()` 加入 `"ls": a.cmdLs,`。

- [ ] **Step 6: 執行測試確認通過**

Run: `go test ./internal/cli/ -v`
Expected: PASS（`TestLsDefaultsToOpenOnly` 會 SKIP，Task 11 後轉綠）

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/cli/
git commit --no-gpg-sign -m "feat(cli): todo ls 與寬度正確的清單排版"
```

---

### Task 11: `done` / `undone` / `rm`

**Files:**
- Create: `internal/cli/cmd_mark.go`
- Modify: `internal/cli/app.go`（`commands()` 加三行）
- Test: `internal/cli/cmd_mark_test.go`

**Interfaces:**
- Consumes: `parseIDs`、`store.Store.Get/SetDone/Delete`、`store.ErrNotFound`
- Produces: `(*App).cmdDone`、`(*App).cmdUndone`、`(*App).cmdRm`

- [ ] **Step 1: 寫失敗的測試**

建立 `internal/cli/cmd_mark_test.go`：

```go
package cli

import (
	"strings"
	"testing"
)

func TestDoneAndUndone(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "買牛奶"})
	out.Reset()

	if code := app.Run([]string{"done", "1"}); code != 0 {
		t.Fatalf("離開碼 = %d", code)
	}
	if !strings.Contains(out.String(), "已完成 #1：買牛奶") {
		t.Errorf("stdout = %q", out.String())
	}
	got, _ := app.Store.Get(1)
	if !got.Done() {
		t.Error("應為已完成")
	}

	out.Reset()
	app.Run([]string{"undone", "1"})
	if !strings.Contains(out.String(), "已取消完成 #1") {
		t.Errorf("stdout = %q", out.String())
	}
	got, _ = app.Store.Get(1)
	if got.Done() {
		t.Error("應為未完成")
	}
}

func TestDoneAcceptsMultipleIDs(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "a"})
	app.Run([]string{"add", "b"})
	out.Reset()
	if code := app.Run([]string{"done", "1", "2"}); code != 0 {
		t.Fatalf("離開碼 = %d", code)
	}
	if strings.Count(out.String(), "已完成") != 2 {
		t.Errorf("stdout = %q，預期兩行", out.String())
	}
}

func TestMarkMissingIDNamesTheID(t *testing.T) {
	app, _, errBuf := newApp(t)
	if code := app.Run([]string{"done", "42"}); code != 1 {
		t.Fatalf("離開碼 = %d，預期 1", code)
	}
	if !strings.Contains(errBuf.String(), "#42") {
		t.Errorf("錯誤訊息應指出是哪個 id：%q", errBuf.String())
	}
}

func TestRm(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "買牛奶"})
	out.Reset()
	if code := app.Run([]string{"rm", "1"}); code != 0 {
		t.Fatalf("離開碼 = %d", code)
	}
	if !strings.Contains(out.String(), "已刪除 #1：買牛奶") {
		t.Errorf("stdout = %q", out.String())
	}
	if _, err := app.Store.Get(1); err == nil {
		t.Error("應該被刪掉了")
	}
}

func TestMarkRequiresID(t *testing.T) {
	for _, cmd := range []string{"done", "undone", "rm"} {
		app, _, errBuf := newApp(t)
		if code := app.Run([]string{cmd}); code != 1 {
			t.Errorf("%s 沒給 id 時離開碼 = %d，預期 1", cmd, code)
		}
		if errBuf.Len() == 0 {
			t.Errorf("%s 應該印出錯誤", cmd)
		}
	}
}
```

- [ ] **Step 2: 執行測試確認失敗**

Run: `go test ./internal/cli/ -run 'TestDone|TestRm|TestMark'`
Expected: FAIL，`未知的指令：done`

- [ ] **Step 3: 實作**

建立 `internal/cli/cmd_mark.go`：

```go
package cli

import "fmt"

func (a *App) cmdDone(args []string) error   { return a.setDone(args, true) }
func (a *App) cmdUndone(args []string) error { return a.setDone(args, false) }

func (a *App) setDone(args []string, done bool) error {
	ids, err := parseIDs(args)
	if err != nil {
		return err
	}
	verb := "已完成"
	if !done {
		verb = "已取消完成"
	}
	for _, id := range ids {
		t, err := a.Store.Get(id)
		if err != nil {
			return fmt.Errorf("#%d：%w", id, err)
		}
		if err := a.Store.SetDone(id, done, a.Now()); err != nil {
			return fmt.Errorf("#%d：%w", id, err)
		}
		fmt.Fprintf(a.Out, "%s #%d：%s\n", verb, id, t.Title)
	}
	return nil
}

func (a *App) cmdRm(args []string) error {
	ids, err := parseIDs(args)
	if err != nil {
		return err
	}
	for _, id := range ids {
		// 先取回來，刪除訊息才能帶上標題，讓使用者確認自己刪對了。
		t, err := a.Store.Get(id)
		if err != nil {
			return fmt.Errorf("#%d：%w", id, err)
		}
		if err := a.Store.Delete(id); err != nil {
			return fmt.Errorf("#%d：%w", id, err)
		}
		fmt.Fprintf(a.Out, "已刪除 #%d：%s\n", id, t.Title)
	}
	return nil
}
```

在 `commands()` 加入：

```go
		"done":   a.cmdDone,
		"undone": a.cmdUndone,
		"rm":     a.cmdRm,
```

- [ ] **Step 4: 執行測試確認通過**

Run: `go test ./internal/cli/ -v`
Expected: PASS，且 `TestLsDefaultsToOpenOnly` 不再 SKIP

- [ ] **Step 5: Commit**

```bash
git add internal/cli/
git commit --no-gpg-sign -m "feat(cli): done、undone、rm"
```

---

### Task 12: `todo edit`

**Files:**
- Create: `internal/cli/cmd_edit.go`
- Modify: `internal/cli/app.go`（`commands()` 加一行）
- Test: `internal/cli/cmd_edit_test.go`

**Interfaces:**
- Consumes: `addFlags()`、`(*App).resolveProject`、`store.Store.Get/Update`
- Produces: `(*App).cmdEdit([]string) error`

**對 spec 的補充**：spec 的 `edit` 只列了 flag。實作再接受一個選用的第二位置參數當新標題（`todo edit 3 "新標題"`），否則 CLI 無法改標題，只能進 TUI 改——那是缺口不是設計。

- [ ] **Step 1: 寫失敗的測試**

建立 `internal/cli/cmd_edit_test.go`：

```go
package cli

import (
	"strings"
	"testing"

	"todo.mirumo.net/internal/task"
)

func TestEditOnlyTouchesGivenFlags(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "買牛奶", "-d", "tomorrow", "--pri", "high", "-t", "購物", "-p", "work"})
	out.Reset()

	if code := app.Run([]string{"edit", "1", "--pri", "low"}); code != 0 {
		t.Fatalf("離開碼 = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.Priority != task.PriLow {
		t.Errorf("priority = %v，預期 low", got.Priority)
	}
	if got.Due == nil {
		t.Error("沒給 --due 就不該動到截止日")
	}
	if got.Project != "work" {
		t.Errorf("project = %q，沒給 -p 就不該動", got.Project)
	}
	if len(got.Tags) != 1 {
		t.Errorf("tags = %v，沒給 -t 就不該動", got.Tags)
	}
}

func TestEditEmptyDueClearsIt(t *testing.T) {
	app, _, _ := newApp(t)
	app.Run([]string{"add", "買牛奶", "-d", "tomorrow"})
	if code := app.Run([]string{"edit", "1", "--due", ""}); code != 0 {
		t.Fatalf("離開碼 = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.Due != nil {
		t.Errorf("due = %v，--due \"\" 應該清掉期限", got.Due)
	}
}

func TestEditEmptyProjectMakesItGlobal(t *testing.T) {
	app, _, _ := newApp(t)
	app.Run([]string{"add", "買牛奶", "-p", "work"})
	if code := app.Run([]string{"edit", "1", "--project="}); code != 0 {
		t.Fatalf("離開碼 = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.Project != "" {
		t.Errorf("project = %q，--project= 應該改回全域未分類", got.Project)
	}
}

func TestEditReplacesTagsWholesale(t *testing.T) {
	app, _, _ := newApp(t)
	app.Run([]string{"add", "買牛奶", "-t", "購物", "-t", "家務"})
	app.Run([]string{"edit", "1", "-t", "早餐"})
	got, _ := app.Store.Get(1)
	if len(got.Tags) != 1 || got.Tags[0] != "早餐" {
		t.Errorf("tags = %v，-t 應整組取代而非累加", got.Tags)
	}
}

func TestEditTitleViaSecondPositional(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "買牛奶"})
	out.Reset()
	if code := app.Run([]string{"edit", "1", "買豆漿"}); code != 0 {
		t.Fatalf("離開碼 = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.Title != "買豆漿" {
		t.Errorf("title = %q", got.Title)
	}
	if !strings.Contains(out.String(), "已更新 #1：買豆漿") {
		t.Errorf("stdout = %q", out.String())
	}
}

func TestEditErrors(t *testing.T) {
	app, _, _ := newApp(t)
	app.Run([]string{"add", "買牛奶"})
	for _, args := range [][]string{
		{"edit"},
		{"edit", "x"},
		{"edit", "42", "--pri", "low"},
		{"edit", "1", "a", "b"},
		{"edit", "1", "--due", "someday"},
	} {
		if code := app.Run(args); code == 0 {
			t.Errorf("%v 應該失敗", args)
		}
	}
}
```

- [ ] **Step 2: 執行測試確認失敗**

Run: `go test ./internal/cli/ -run TestEdit`
Expected: FAIL，`未知的指令：edit`

- [ ] **Step 3: 實作**

建立 `internal/cli/cmd_edit.go`：

```go
package cli

import (
	"errors"
	"fmt"
	"strings"

	"todo.mirumo.net/internal/datearg"
	"todo.mirumo.net/internal/task"
)

func (a *App) cmdEdit(args []string) error {
	r, err := addFlags().Parse(args)
	if err != nil {
		return err
	}
	pos := r.Args()
	if len(pos) == 0 {
		return errors.New("用法：todo edit <id> [新標題] [flags]")
	}
	if len(pos) > 2 {
		return fmt.Errorf("最多接受 <id> 與新標題兩個位置參數，收到 %d 個", len(pos))
	}
	ids, err := parseIDs(pos[:1])
	if err != nil {
		return err
	}
	t, err := a.Store.Get(ids[0])
	if err != nil {
		return fmt.Errorf("#%d：%w", ids[0], err)
	}

	// 只動「有給」的欄位。沒給 flag 與給了空值是兩回事。
	if len(pos) == 2 {
		if t.Title, err = task.ValidateTitle(pos[1]); err != nil {
			return err
		}
	}
	if p, ok, err := a.resolveProject(r); err != nil {
		return err
	} else if ok {
		t.Project = p
	}
	if r.Changed("due") {
		if strings.TrimSpace(r.String("due")) == "" {
			t.Due = nil
		} else {
			d, err := datearg.Parse(r.String("due"), a.Now())
			if err != nil {
				return err
			}
			t.Due = &d
		}
	}
	if r.Changed("pri") {
		if t.Priority, err = task.ParsePriority(r.String("pri")); err != nil {
			return err
		}
	}
	if r.Changed("tag") {
		t.Tags = task.NormalizeTags(r.Strings("tag"))
	}
	t.UpdatedAt = a.Now()

	if err := a.Store.Update(t); err != nil {
		return fmt.Errorf("#%d：%w", t.ID, err)
	}
	fmt.Fprintf(a.Out, "已更新 #%d：%s\n", t.ID, t.Title)
	return nil
}
```

在 `commands()` 加入 `"edit": a.cmdEdit,`。

- [ ] **Step 4: 執行測試確認通過**

Run: `go test ./internal/cli/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/
git commit --no-gpg-sign -m "feat(cli): todo edit"
```

---

### Task 13: `projects` 與 `tags`

**Files:**
- Create: `internal/cli/cmd_meta.go`
- Modify: `internal/cli/app.go`（`commands()` 加兩行）
- Test: `internal/cli/cmd_meta_test.go`

**Interfaces:**
- Consumes: `store.Store.Projects/Tags`、`project.Label`、`pad`
- Produces: `(*App).cmdProjects`、`(*App).cmdTags`

- [ ] **Step 1: 寫失敗的測試**

建立 `internal/cli/cmd_meta_test.go`：

```go
package cli

import (
	"strings"
	"testing"
)

func TestProjectsListsCountsAndUncategorized(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "全域的事"})
	app.Run([]string{"add", "工作 A", "-p", "/p/work"})
	app.Run([]string{"add", "工作 B", "-p", "/p/work"})
	app.Run([]string{"done", "3"})
	out.Reset()

	if code := app.Run([]string{"projects"}); code != 0 {
		t.Fatalf("離開碼 = %d", code)
	}
	s := out.String()
	if !strings.Contains(s, "（未分類）") {
		t.Errorf("應該把空專案顯示為（未分類）：%q", s)
	}
	if !strings.Contains(s, "work") || !strings.Contains(s, "1 未完成") {
		t.Errorf("應顯示 basename 與未完成數：%q", s)
	}
	if !strings.Contains(s, "/p/work") {
		t.Errorf("有專案者應附上完整路徑：%q", s)
	}
}

func TestProjectsEmpty(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"projects"})
	if !strings.Contains(out.String(), "還沒有任何待辦") {
		t.Errorf("= %q", out.String())
	}
}

func TestTags(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "x", "-t", "購物", "-t", "家務"})
	out.Reset()
	app.Run([]string{"tags"})
	s := out.String()
	if !strings.Contains(s, "@家務") || !strings.Contains(s, "@購物") {
		t.Errorf("= %q", s)
	}
}

func TestTagsEmpty(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"tags"})
	if !strings.Contains(out.String(), "還沒有任何標籤") {
		t.Errorf("= %q", out.String())
	}
}

func TestMetaCommandsRejectArgs(t *testing.T) {
	for _, cmd := range []string{"projects", "tags"} {
		app, _, errBuf := newApp(t)
		if code := app.Run([]string{cmd, "垃圾"}); code != 1 {
			t.Errorf("%s 離開碼 = %d，預期 1", cmd, code)
		}
		if !strings.Contains(errBuf.String(), "不接受參數") {
			t.Errorf("%s stderr = %q", cmd, errBuf.String())
		}
	}
}
```

- [ ] **Step 2: 執行測試確認失敗**

Run: `go test ./internal/cli/ -run 'TestProjects|TestTags|TestMeta'`
Expected: FAIL，`未知的指令：projects`

- [ ] **Step 3: 實作**

建立 `internal/cli/cmd_meta.go`：

```go
package cli

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"todo.mirumo.net/internal/project"
)

func (a *App) cmdProjects(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("projects 不接受參數，收到 %q", args[0])
	}
	ps, err := a.Store.Projects()
	if err != nil {
		return err
	}
	if len(ps) == 0 {
		fmt.Fprintln(a.Out, "還沒有任何待辦")
		return nil
	}
	labels := make([]string, len(ps))
	var w int
	for i, p := range ps {
		labels[i] = project.Label(p.Path)
		if labels[i] == "" {
			labels[i] = "（未分類）"
		}
		w = max(w, lipgloss.Width(labels[i]))
	}
	for i, p := range ps {
		fmt.Fprintf(a.Out, "%s  %d 未完成", pad(labels[i], w), p.Open)
		if p.Path != "" {
			fmt.Fprintf(a.Out, "  %s", p.Path)
		}
		fmt.Fprintln(a.Out)
	}
	return nil
}

func (a *App) cmdTags(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("tags 不接受參數，收到 %q", args[0])
	}
	tags, err := a.Store.Tags()
	if err != nil {
		return err
	}
	if len(tags) == 0 {
		fmt.Fprintln(a.Out, "還沒有任何標籤")
		return nil
	}
	for _, t := range tags {
		fmt.Fprintf(a.Out, "@%s\n", t)
	}
	return nil
}
```

在 `commands()` 加入：

```go
		"projects": a.cmdProjects,
		"tags":     a.cmdTags,
```

- [ ] **Step 4: 執行測試確認通過**

Run: `go test ./internal/cli/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/
git commit --no-gpg-sign -m "feat(cli): projects 與 tags"
```

---

### Task 14: 組裝可執行檔

**Files:**
- Create: `cmd/todo/main.go`
- Test: `cmd/todo/main_test.go`

**Interfaces:**
- Consumes: `cli.SplitGlobal`、`cli.App`、`store.OpenSQLite`
- Produces: `todo` 執行檔；內部 `resolveDBPath(envDB, flagDB, home string) string`、`isTTY(*os.File) bool`

- [ ] **Step 1: 寫失敗的測試**

建立 `cmd/todo/main_test.go`：

```go
package main

import (
	"path/filepath"
	"testing"
)

func TestResolveDBPath(t *testing.T) {
	home := "/home/me"
	def := filepath.Join(home, ".todo", "todo.db")
	cases := []struct {
		name   string
		env    string
		flag   string
		want   string
	}{
		{"預設", "", "", def},
		{"環境變數", "/tmp/env.db", "", "/tmp/env.db"},
		{"flag 覆寫環境變數", "/tmp/env.db", "/tmp/flag.db", "/tmp/flag.db"},
		{"只有 flag", "", "/tmp/flag.db", "/tmp/flag.db"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveDBPath(c.env, c.flag, home); got != c.want {
				t.Errorf("= %q，預期 %q", got, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: 執行測試確認失敗**

Run: `go test ./cmd/todo/`
Expected: FAIL，`undefined: resolveDBPath`

- [ ] **Step 3: 實作**

建立 `cmd/todo/main.go`：

```go
// Command todo 是一個本機待辦事項工具。
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"todo.mirumo.net/internal/cli"
	"todo.mirumo.net/internal/store"
	"todo.mirumo.net/internal/tui"
)

func main() { os.Exit(run()) }

// run 把整個流程包起來，讓 defer 有機會執行（os.Exit 會跳過 defer）。
func run() int {
	dbFlag, args, err := cli.SplitGlobal(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "錯誤：%s\n", err)
		return 2
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "錯誤：找不到家目錄：%s\n", err)
		return 1
	}
	dbPath := resolveDBPath(os.Getenv("TODO_DB"), dbFlag, home)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "錯誤：無法建立資料目錄 %s：%s\n", filepath.Dir(dbPath), err)
		return 1
	}
	st, err := store.OpenSQLite(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "錯誤：無法開啟資料庫 %s：%s\n", dbPath, err)
		return 1
	}
	defer st.Close()

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	app := &cli.App{
		Store: st,
		Out:   os.Stdout,
		Err:   os.Stderr,
		Now:   time.Now,
		Cwd:   cwd,
		Color: isTTY(os.Stdout),
	}
	app.RunTUI = func() error { return tui.Run(st, app.Now, cwd) }
	return app.Run(args)
}

// resolveDBPath 決定資料庫位置：--db 優先於 TODO_DB，都沒有就用 ~/.todo/todo.db。
func resolveDBPath(envDB, flagDB, home string) string {
	if flagDB != "" {
		return flagDB
	}
	if envDB != "" {
		return envDB
	}
	return filepath.Join(home, ".todo", "todo.db")
}

// isTTY 判斷輸出是不是終端機；被導向檔案或管線時要關掉顏色。
func isTTY(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
```

此時 `internal/tui` 還不存在，先建一個最小可編譯的版本 `internal/tui/tui.go`：

```go
// Package tui 提供 todo 的互動介面。
package tui

import (
	"errors"
	"time"

	"todo.mirumo.net/internal/store"
)

// Run 啟動互動介面。
func Run(s store.Store, now func() time.Time, cwd string) error {
	return errors.New("TUI 尚未實作")
}
```

- [ ] **Step 4: 執行測試與手動驗證**

Run:
```bash
go test ./... 
go build -o /tmp/todo ./cmd/todo
TODO_DB=/tmp/todo-smoke.db /tmp/todo add "買牛奶" --pri high -d tomorrow -t 購物
TODO_DB=/tmp/todo-smoke.db /tmp/todo ls
TODO_DB=/tmp/todo-smoke.db /tmp/todo done 1
TODO_DB=/tmp/todo-smoke.db /tmp/todo ls -a
TODO_DB=/tmp/todo-smoke.db /tmp/todo ls | cat
rm -f /tmp/todo-smoke.db
```
Expected: 測試 PASS；`add` 印出 `已新增 #1：買牛奶`；`ls` 顯示對齊的一行且帶 `明天`、`!高`、`@購物`；`done` 後 `ls` 為空、`ls -a` 顯示 `[x]`；`| cat` 那次輸出不含任何 ANSI 跳脫碼。

- [ ] **Step 5: Commit**

```bash
git add cmd/ internal/tui/
git commit --no-gpg-sign -m "feat(cmd): 組裝 todo 執行檔與 ~/.todo 資料路徑"
```

---

### Task 15: TUI 清單、導航與完成切換

**Files:**
- Create: `internal/tui/cmds.go`
- Create: `internal/tui/list.go`
- Modify: `internal/tui/tui.go`（取代 Task 14 的最小樁）
- Test: `internal/tui/tui_test.go`

**Interfaces:**
- Consumes: `store.Store`、`task.Filter`、`datearg.Format`、`project.Label`
- Produces: `tui.Model`、`tui.New(s store.Store, now func() time.Time, cwd string) Model`、`(Model).Init/Update/View`、`tui.Run`、內部 msg 型別 `tasksMsg`、`errMsg`、`savedMsg`、內部 `(Model).loadCmd()`、`(Model).current() (task.Task, bool)`、`mode` 常數 `modeList`

- [ ] **Step 1: 加入 Bubble Tea 依賴**

```bash
go get github.com/charmbracelet/bubbletea@v1
```

- [ ] **Step 2: 寫失敗的測試**

建立 `internal/tui/tui_test.go`：

```go
package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"todo.mirumo.net/internal/store"
	"todo.mirumo.net/internal/task"
)

func refTime() time.Time { return time.Date(2026, 8, 29, 15, 0, 0, 0, time.Local) }

func day(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	return &t
}

// newModel 建一個接上 in-memory 資料庫、已載入資料的 Model。
func newModel(t *testing.T) (Model, store.Store) {
	t.Helper()
	s, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	for _, ti := range []task.Task{
		{Title: "第一件", Due: day(2026, 8, 29), Priority: task.PriHigh, Tags: []string{"急"}},
		{Title: "第二件", Project: "/p/work"},
		{Title: "第三件"},
	} {
		ti.CreatedAt, ti.UpdatedAt = refTime(), refTime()
		if _, err := s.Add(ti); err != nil {
			t.Fatal(err)
		}
	}
	m := New(s, refTime, t.TempDir())
	m, _ = send(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, msg := run(t, m, m.Init())
	m, _ = send(t, m, msg)
	return m, s
}

// key 把按鍵字串轉成 tea.KeyMsg。
func key(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "ctrl+p":
		return tea.KeyMsg{Type: tea.KeyCtrlP}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// send 餵一個 msg，回傳新 model 與 cmd 執行後的結果 msg（沒有 cmd 時為 nil）。
func send(t *testing.T, m Model, msg tea.Msg) (Model, tea.Msg) {
	t.Helper()
	next, cmd := m.Update(msg)
	return run(t, next.(Model), cmd)
}

func run(t *testing.T, m Model, cmd tea.Cmd) (Model, tea.Msg) {
	t.Helper()
	if cmd == nil {
		return m, nil
	}
	return m, cmd()
}

// press 按一個鍵，並把它引發的 cmd 結果也餵回去（模擬 Bubble Tea 的迴圈一輪）。
func press(t *testing.T, m Model, k string) Model {
	t.Helper()
	m, msg := send(t, m, key(k))
	for i := 0; msg != nil && i < 4; i++ {
		m, msg = send(t, m, msg)
	}
	return m
}

func TestInitLoadsTasks(t *testing.T) {
	m, _ := newModel(t)
	if len(m.tasks) != 3 {
		t.Fatalf("載入 %d 筆，預期 3 筆", len(m.tasks))
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d，預期 0", m.cursor)
	}
}

func TestNavigation(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "j")
	if m.cursor != 1 {
		t.Errorf("j 之後 cursor = %d，預期 1", m.cursor)
	}
	m = press(t, m, "down")
	m = press(t, m, "down")
	if m.cursor != 2 {
		t.Errorf("到底之後 cursor = %d，預期停在 2 不越界", m.cursor)
	}
	m = press(t, m, "k")
	if m.cursor != 1 {
		t.Errorf("k 之後 cursor = %d，預期 1", m.cursor)
	}
	m = press(t, m, "g")
	if m.cursor != 0 {
		t.Errorf("g 之後 cursor = %d，預期 0", m.cursor)
	}
	m = press(t, m, "G")
	if m.cursor != 2 {
		t.Errorf("G 之後 cursor = %d，預期 2", m.cursor)
	}
	m = press(t, m, "k")
	m = press(t, m, "k")
	m = press(t, m, "k")
	if m.cursor != 0 {
		t.Errorf("到頂之後 cursor = %d，預期停在 0 不越界", m.cursor)
	}
}

func TestSpaceTogglesDone(t *testing.T) {
	m, s := newModel(t)
	id := m.tasks[0].ID
	m = press(t, m, " ")

	got, err := s.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Done() {
		t.Error("space 應該把項目標成已完成")
	}
	// 預設只看未完成，重查後該項目應消失。
	if len(m.tasks) != 2 {
		t.Errorf("重查後剩 %d 筆，預期 2 筆", len(m.tasks))
	}
}

func TestQuit(t *testing.T) {
	m, _ := newModel(t)
	_, cmd := m.Update(key("q"))
	if cmd == nil {
		t.Fatal("q 應該回傳一個 cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q 應該離開程式")
	}
}

func TestErrMsgShowsWithoutCrashing(t *testing.T) {
	m, _ := newModel(t)
	m, _ = send(t, m, errMsg{err: errFake})
	if m.err == nil {
		t.Fatal("錯誤應該被記下來")
	}
	if !strings.Contains(m.View(), "壞掉了") {
		t.Errorf("錯誤應該顯示在畫面上：%q", m.View())
	}
}

func TestViewShowsTasksAndCursor(t *testing.T) {
	m, _ := newModel(t)
	v := m.View()
	for _, want := range []string{"第一件", "第二件", "第三件", "今天", "!高", "@急", "work"} {
		if !strings.Contains(v, want) {
			t.Errorf("畫面缺少 %q：\n%s", want, v)
		}
	}
	if !strings.Contains(v, "▸") {
		t.Errorf("畫面應該有游標標記：\n%s", v)
	}
}
```

在測試檔尾加上：

```go
var errFake = fakeErr{}

type fakeErr struct{}

func (fakeErr) Error() string { return "資料庫壞掉了" }
```

- [ ] **Step 3: 執行測試確認失敗**

Run: `go test ./internal/tui/`
Expected: FAIL，`undefined: New`

- [ ] **Step 4: 實作 msg 與 cmd**

建立 `internal/tui/cmds.go`：

```go
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"todo.mirumo.net/internal/task"
)

// 所有與資料庫往來的動作都包成 tea.Cmd，結果以 msg 回到 Update。
// Update 本身不碰 IO，維持純函式，測試只需要餵 msg。
type (
	tasksMsg []task.Task
	errMsg   struct{ err error }
	savedMsg struct{ note string }
)

func (m Model) loadCmd() tea.Cmd {
	s, f, now := m.store, m.filter, m.now()
	return func() tea.Msg {
		ts, err := s.List(f, now)
		if err != nil {
			return errMsg{err}
		}
		return tasksMsg(ts)
	}
}

func (m Model) toggleCmd(t task.Task) tea.Cmd {
	s, now := m.store, m.now()
	return func() tea.Msg {
		if err := s.SetDone(t.ID, !t.Done(), now); err != nil {
			return errMsg{err}
		}
		note := "已完成「" + t.Title + "」"
		if t.Done() {
			note = "已取消完成「" + t.Title + "」"
		}
		return savedMsg{note: note}
	}
}
```

- [ ] **Step 5: 實作 Model**

改寫 `internal/tui/tui.go`：

```go
// Package tui 提供 todo 的互動介面。
package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"todo.mirumo.net/internal/store"
	"todo.mirumo.net/internal/task"
)

type mode int

const (
	modeList mode = iota
)

// Model 是根 model。所有子狀態掛在這裡，Update 依 mode 分派。
type Model struct {
	store store.Store
	now   func() time.Time
	cwd   string

	mode   mode
	tasks  []task.Task
	cursor int
	filter task.Filter

	status        string
	err           error
	width, height int
}

// New 建立一個 Model。
func New(s store.Store, now func() time.Time, cwd string) Model {
	return Model{store: s, now: now, cwd: cwd, mode: modeList, width: 80, height: 24}
}

// Run 啟動互動介面。
func Run(s store.Store, now func() time.Time, cwd string) error {
	_, err := tea.NewProgram(New(s, now, cwd), tea.WithAltScreen()).Run()
	return err
}

func (m Model) Init() tea.Cmd { return m.loadCmd() }

// current 回傳游標所指的項目。
func (m Model) current() (task.Task, bool) {
	if m.cursor < 0 || m.cursor >= len(m.tasks) {
		return task.Task{}, false
	}
	return m.tasks[m.cursor], true
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tasksMsg:
		m.tasks = []task.Task(msg)
		m.err = nil
		if m.cursor >= len(m.tasks) {
			m.cursor = max(0, len(m.tasks)-1)
		}
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil

	case savedMsg:
		m.status, m.err = msg.note, nil
		return m, m.loadCmd()

	case tea.KeyMsg:
		switch m.mode {
		case modeList:
			return m.updateList(msg)
		}
	}
	return m, nil
}

func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		if m.cursor < len(m.tasks)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "g":
		m.cursor = 0
	case "G":
		m.cursor = max(0, len(m.tasks)-1)
	case " ":
		if t, ok := m.current(); ok {
			return m, m.toggleCmd(t)
		}
	}
	return m, nil
}

func (m Model) View() string { return m.viewList() }
```

- [ ] **Step 6: 實作畫面**

建立 `internal/tui/list.go`：

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"todo.mirumo.net/internal/datearg"
	"todo.mirumo.net/internal/project"
	"todo.mirumo.net/internal/task"
)

var (
	styleCursor = lipgloss.NewStyle().Bold(true)
	styleDim    = lipgloss.NewStyle().Faint(true)
	styleErr    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleHint   = lipgloss.NewStyle().Faint(true)
)

func pad(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

// taskLine 組出一行的內容（不含游標標記）。
func (m Model) taskLine(t task.Task) string {
	status := "[ ]"
	if t.Done() {
		status = "[x]"
	}
	parts := []string{status}
	if p := t.Priority.Label(); p != "" {
		parts = append(parts, "!"+p)
	}
	if t.Due != nil {
		parts = append(parts, datearg.Format(*t.Due, m.now()))
	}
	parts = append(parts, t.Title)
	if p := project.Label(t.Project); p != "" {
		parts = append(parts, p)
	}
	if len(t.Tags) > 0 {
		parts = append(parts, "@"+strings.Join(t.Tags, " @"))
	}
	return strings.Join(parts, " ")
}

func (m Model) viewList() string {
	var b strings.Builder
	b.WriteString(m.header() + "\n\n")
	if len(m.tasks) == 0 {
		b.WriteString(styleDim.Render("沒有符合的待辦") + "\n")
	}
	for i, t := range m.tasks {
		marker := "  "
		if i == m.cursor {
			marker = "▸ "
		}
		line := marker + m.taskLine(t)
		switch {
		case i == m.cursor:
			line = styleCursor.Render(line)
		case t.Done():
			line = styleDim.Render(line)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + m.footer())
	return b.String()
}

func (m Model) header() string {
	return fmt.Sprintf("todo — %d 筆", len(m.tasks))
}

func (m Model) footer() string {
	if m.err != nil {
		return styleErr.Render("錯誤：" + m.err.Error())
	}
	if m.status != "" {
		return m.status
	}
	return styleHint.Render("j/k 移動 · space 完成 · q 離開")
}
```

- [ ] **Step 7: 執行測試確認通過**

Run: `go test ./internal/tui/ -v`
Expected: PASS

- [ ] **Step 8: 手動確認**

```bash
go build -o /tmp/todo ./cmd/todo
TODO_DB=/tmp/todo-tui.db /tmp/todo add "第一件" --pri high -d today
TODO_DB=/tmp/todo-tui.db /tmp/todo add "第二件"
TODO_DB=/tmp/todo-tui.db /tmp/todo tui
```
Expected: 進入全螢幕清單，`j`/`k` 移動、`space` 勾掉項目後它從清單消失、`q` 離開。

- [ ] **Step 9: Commit**

```bash
git add go.mod go.sum internal/tui/
git commit --no-gpg-sign -m "feat(tui): 清單、導航與完成切換"
```

---

### Task 16: 刪除與復原、搜尋、排序、顯示已完成

**Files:**
- Modify: `internal/tui/tui.go`（新增 `modeSearch`、Model 欄位、按鍵）
- Modify: `internal/tui/cmds.go`（新增 `deletedMsg`、`deleteCmd`、`restoreCmd`）
- Modify: `internal/tui/list.go`（footer 顯示提示）
- Test: `internal/tui/filter_test.go`

**Interfaces:**
- Consumes: `store.Store.Delete/Get/Restore`、`task.SortBy`
- Produces: `deletedMsg{t task.Task}`、`(Model).deleteCmd`、`(Model).restoreCmd`、Model 新欄位 `undo *task.Task`、`search textinput.Model`、`modeSearch`

- [ ] **Step 1: 加入 Bubbles 依賴**

```bash
go get github.com/charmbracelet/bubbles@v0
```

- [ ] **Step 2: 寫失敗的測試**

建立 `internal/tui/filter_test.go`：

```go
package tui

import (
	"errors"
	"strings"
	"testing"

	"todo.mirumo.net/internal/store"
	"todo.mirumo.net/internal/task"
)

func TestDeleteThenUndo(t *testing.T) {
	m, s := newModel(t)
	victim := m.tasks[0]

	m = press(t, m, "d")
	if _, err := s.Get(victim.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("d 應該刪掉項目，err = %v", err)
	}
	if len(m.tasks) != 2 {
		t.Errorf("刪除後剩 %d 筆，預期 2 筆", len(m.tasks))
	}
	if !strings.Contains(m.View(), "u 復原") {
		t.Errorf("底部應提示可以復原：\n%s", m.View())
	}

	m = press(t, m, "u")
	back, err := s.Get(victim.ID)
	if err != nil {
		t.Fatalf("u 應該以原 id 復原，err = %v", err)
	}
	if back.Title != victim.Title || len(back.Tags) != len(victim.Tags) {
		t.Errorf("復原內容不符：%+v", back)
	}
	if len(m.tasks) != 3 {
		t.Errorf("復原後 %d 筆，預期 3 筆", len(m.tasks))
	}
}

func TestUndoOnlyKeepsOneLevel(t *testing.T) {
	m, s := newModel(t)
	first := m.tasks[0]
	m = press(t, m, "d")
	second := m.tasks[0]
	m = press(t, m, "d")
	m = press(t, m, "u")

	if _, err := s.Get(second.ID); err != nil {
		t.Errorf("最後刪的那筆應該被復原：%v", err)
	}
	if _, err := s.Get(first.ID); !errors.Is(err, store.ErrNotFound) {
		t.Error("只保留一層 undo，更早刪的不該回來")
	}
	m = press(t, m, "u")
	if !strings.Contains(m.View(), "沒有可復原") {
		t.Errorf("沒有可復原時應該說一聲：\n%s", m.View())
	}
}

func TestSearchFiltersIncrementally(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "/")
	if m.mode != modeSearch {
		t.Fatal("/ 應該進入搜尋模式")
	}
	m = press(t, m, "第二")
	if len(m.tasks) != 1 || m.tasks[0].Title != "第二件" {
		t.Errorf("打字應該即時過濾，實得 %d 筆", len(m.tasks))
	}
	m = press(t, m, "enter")
	if m.mode != modeList {
		t.Error("enter 應該回到清單並保留過濾")
	}
	if len(m.tasks) != 1 {
		t.Errorf("enter 後過濾應保留，實得 %d 筆", len(m.tasks))
	}
}

func TestSearchEscCancels(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "/")
	m = press(t, m, "第二")
	m = press(t, m, "esc")
	if m.mode != modeList {
		t.Error("esc 應該回到清單")
	}
	if len(m.tasks) != 3 {
		t.Errorf("esc 應該取消過濾，實得 %d 筆", len(m.tasks))
	}
}

func TestToggleIncludeDone(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, " ") // 完成第一件
	if len(m.tasks) != 2 {
		t.Fatalf("預期剩 2 筆，實得 %d", len(m.tasks))
	}
	m = press(t, m, "A")
	if len(m.tasks) != 3 {
		t.Errorf("A 應該連已完成一起顯示，實得 %d 筆", len(m.tasks))
	}
	m = press(t, m, "A")
	if len(m.tasks) != 2 {
		t.Errorf("再按 A 應該切回只看未完成，實得 %d 筆", len(m.tasks))
	}
}

func TestSortCycles(t *testing.T) {
	m, _ := newModel(t)
	if m.filter.Sort != task.SortDue {
		t.Fatal("預設應為 due")
	}
	m = press(t, m, "s")
	if m.filter.Sort != task.SortPriority {
		t.Errorf("s 之後 = %v，預期 pri", m.filter.Sort)
	}
	m = press(t, m, "s")
	if m.filter.Sort != task.SortCreated {
		t.Errorf("再按 s = %v，預期 created", m.filter.Sort)
	}
	m = press(t, m, "s")
	if m.filter.Sort != task.SortDue {
		t.Errorf("循環回來 = %v，預期 due", m.filter.Sort)
	}
}

func TestEscClearsAllFilters(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "/")
	m = press(t, m, "第二")
	m = press(t, m, "enter")
	m = press(t, m, "A")
	m = press(t, m, "esc")
	if len(m.tasks) != 3 {
		t.Errorf("esc 應該清掉所有過濾，實得 %d 筆", len(m.tasks))
	}
	if m.filter.Search != "" || m.filter.IncludeDone {
		t.Errorf("filter 應該歸零：%+v", m.filter)
	}
}
```

- [ ] **Step 3: 執行測試確認失敗**

Run: `go test ./internal/tui/ -run 'TestDelete|TestUndo|TestSearch|TestToggle|TestSort|TestEsc'`
Expected: FAIL，`undefined: modeSearch`

- [ ] **Step 4: 實作 cmd**

在 `internal/tui/cmds.go` 的 msg 群組加入 `deletedMsg`，並追加兩個 cmd：

```go
type (
	tasksMsg   []task.Task
	errMsg     struct{ err error }
	savedMsg   struct{ note string }
	deletedMsg struct{ t task.Task }
)

// deleteCmd 先完整取回再刪除——undo 需要包含標籤的整份資料。
func (m Model) deleteCmd(t task.Task) tea.Cmd {
	s := m.store
	return func() tea.Msg {
		full, err := s.Get(t.ID)
		if err != nil {
			return errMsg{err}
		}
		if err := s.Delete(t.ID); err != nil {
			return errMsg{err}
		}
		return deletedMsg{t: full}
	}
}

func (m Model) restoreCmd(t task.Task) tea.Cmd {
	s := m.store
	return func() tea.Msg {
		if err := s.Restore(t); err != nil {
			return errMsg{err}
		}
		return savedMsg{note: "已復原「" + t.Title + "」"}
	}
}
```

- [ ] **Step 5: 實作 Model 變更**

`internal/tui/tui.go` 的 mode 常數與 Model 改成：

```go
const (
	modeList mode = iota
	modeSearch
)

type Model struct {
	store store.Store
	now   func() time.Time
	cwd   string

	mode   mode
	tasks  []task.Task
	cursor int
	filter task.Filter

	search textinput.Model
	// undo 只保留一層：最近一次刪除的完整項目，離開 TUI 即失效。
	undo *task.Task

	status        string
	err           error
	width, height int
}
```

`New` 改成：

```go
func New(s store.Store, now func() time.Time, cwd string) Model {
	ti := textinput.New()
	ti.Prompt = "/"
	ti.Placeholder = "搜尋標題"
	return Model{
		store: s, now: now, cwd: cwd,
		mode: modeList, search: ti,
		width: 80, height: 24,
	}
}
```

import 補上 `"github.com/charmbracelet/bubbles/textinput"`。

`Update` 的 `deletedMsg` 分支與 mode 分派：

```go
	case deletedMsg:
		t := msg.t
		m.undo = &t
		m.status = "已刪除「" + t.Title + "」· u 復原"
		m.err = nil
		return m, m.loadCmd()

	case tea.KeyMsg:
		switch m.mode {
		case modeList:
			return m.updateList(msg)
		case modeSearch:
			return m.updateSearch(msg)
		}
```

`updateList` 新增按鍵：

```go
	case "d":
		if t, ok := m.current(); ok {
			return m, m.deleteCmd(t)
		}
	case "u":
		if m.undo == nil {
			m.status = "沒有可復原的刪除"
			return m, nil
		}
		t := *m.undo
		m.undo = nil
		return m, m.restoreCmd(t)
	case "/":
		m.mode = modeSearch
		m.search.SetValue(m.filter.Search)
		m.search.CursorEnd()
		m.search.Focus()
		return m, nil
	case "A":
		m.filter.IncludeDone = !m.filter.IncludeDone
		return m, m.loadCmd()
	case "s":
		m.filter.Sort = (m.filter.Sort + 1) % 3
		m.status = "排序：" + sortLabel(m.filter.Sort)
		return m, m.loadCmd()
	case "esc":
		m.filter = task.Filter{}
		m.search.SetValue("")
		m.status = ""
		return m, m.loadCmd()
```

新增搜尋模式與標籤函式：

```go
// updateSearch 處理增量搜尋。每個按鍵都重查一次——清單規模小，成本可忽略，
// 換來的是畫面與資料庫永遠一致。
func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.mode = modeList
		m.search.Blur()
		return m, nil
	case "esc":
		m.mode = modeList
		m.search.Blur()
		m.search.SetValue("")
		m.filter.Search = ""
		return m, m.loadCmd()
	}
	// 刻意丟掉 textinput 回傳的 cmd：那是游標閃爍的計時器。
	// 轉傳它會讓 Update 的測試變成在等計時器，而閃爍只是裝飾。
	m.search, _ = m.search.Update(msg)
	m.filter.Search = m.search.Value()
	m.cursor = 0
	return m, m.loadCmd()
}

func sortLabel(s task.SortBy) string {
	switch s {
	case task.SortPriority:
		return "優先度"
	case task.SortCreated:
		return "建立時間"
	}
	return "截止日"
}
```

- [ ] **Step 6: footer 顯示搜尋列**

`internal/tui/list.go` 的 `footer` 改成：

```go
func (m Model) footer() string {
	if m.mode == modeSearch {
		return m.search.View()
	}
	if m.err != nil {
		return styleErr.Render("錯誤：" + m.err.Error())
	}
	if m.status != "" {
		return m.status
	}
	return styleHint.Render("j/k 移動 · space 完成 · d 刪除 · / 搜尋 · s 排序 · A 含已完成 · esc 清除 · q 離開")
}
```

`header` 加上目前的過濾狀態：

```go
func (m Model) header() string {
	h := fmt.Sprintf("todo — %d 筆", len(m.tasks))
	if m.filter.Search != "" {
		h += "  搜尋：" + m.filter.Search
	}
	if m.filter.IncludeDone {
		h += "  含已完成"
	}
	return h
}
```

- [ ] **Step 7: 執行測試確認通過**

Run: `go test ./internal/tui/ -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add go.mod go.sum internal/tui/
git commit --no-gpg-sign -m "feat(tui): 刪除與復原、增量搜尋、排序、顯示已完成"
```

---

### Task 17: 專案與標籤選單

**Files:**
- Modify: `internal/tui/tui.go`（新增 `modePicker` 與按鍵）
- Modify: `internal/tui/cmds.go`（新增 `projectsMsg`、`tagsMsg` 與對應 cmd）
- Create: `internal/tui/picker.go`
- Test: `internal/tui/picker_test.go`

**Interfaces:**
- Consumes: `store.Store.Projects/Tags`、`project.Label`
- Produces: `pickerKind`（`pickProject`/`pickTag`）、`pickerItem{label, value string; clear bool}`、`pickerState{kind pickerKind; items []pickerItem; cursor int}`、`(Model).projectsCmd()`、`(Model).tagsCmd()`、`modePicker`

- [ ] **Step 1: 寫失敗的測試**

建立 `internal/tui/picker_test.go`：

```go
package tui

import (
	"strings"
	"testing"
)

func TestProjectPickerFiltersByProject(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "P")
	if m.mode != modePicker {
		t.Fatal("P 應該開啟選單")
	}
	v := m.View()
	for _, want := range []string{"全部", "（未分類）", "work"} {
		if !strings.Contains(v, want) {
			t.Errorf("選單缺少 %q：\n%s", want, v)
		}
	}
	// 第一項是「全部」，第二項是「（未分類）」，第三項是 work。
	m = press(t, m, "j")
	m = press(t, m, "j")
	m = press(t, m, "enter")
	if m.mode != modeList {
		t.Fatal("enter 應該回到清單")
	}
	if len(m.tasks) != 1 || m.tasks[0].Title != "第二件" {
		t.Errorf("選 work 後應只剩第二件，實得 %d 筆", len(m.tasks))
	}
}

func TestProjectPickerUncategorized(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "P")
	m = press(t, m, "j")
	m = press(t, m, "enter")
	if len(m.tasks) != 2 {
		t.Errorf("（未分類）應剩 2 筆，實得 %d 筆", len(m.tasks))
	}
	if m.filter.Project == nil || *m.filter.Project != "" {
		t.Errorf("filter.Project = %v，預期指向空字串", m.filter.Project)
	}
}

func TestProjectPickerAllClearsFilter(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "P")
	m = press(t, m, "j")
	m = press(t, m, "enter")
	m = press(t, m, "P")
	m = press(t, m, "enter") // 第一項「全部」
	if m.filter.Project != nil {
		t.Errorf("選「全部」應該清掉專案過濾，實得 %v", m.filter.Project)
	}
	if len(m.tasks) != 3 {
		t.Errorf("實得 %d 筆，預期 3 筆", len(m.tasks))
	}
}

func TestTagPicker(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "T")
	if m.mode != modePicker {
		t.Fatal("T 應該開啟選單")
	}
	if !strings.Contains(m.View(), "@急") {
		t.Errorf("標籤選單應列出 @急：\n%s", m.View())
	}
	m = press(t, m, "j")
	m = press(t, m, "enter")
	if len(m.tasks) != 1 || m.tasks[0].Title != "第一件" {
		t.Errorf("選 @急 後應只剩第一件，實得 %d 筆", len(m.tasks))
	}
}

func TestPickerEscCancels(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "P")
	m = press(t, m, "esc")
	if m.mode != modeList {
		t.Error("esc 應該關掉選單")
	}
	if m.filter.Project != nil {
		t.Error("esc 不該套用任何過濾")
	}
}
```

- [ ] **Step 2: 執行測試確認失敗**

Run: `go test ./internal/tui/ -run 'TestProjectPicker|TestTagPicker|TestPickerEsc'`
Expected: FAIL，`undefined: modePicker`

- [ ] **Step 3: 實作 cmd**

在 `internal/tui/cmds.go` 追加：

```go
type (
	projectsMsg []store.ProjectCount
	tagsMsg     []string
)

func (m Model) projectsCmd() tea.Cmd {
	s := m.store
	return func() tea.Msg {
		ps, err := s.Projects()
		if err != nil {
			return errMsg{err}
		}
		return projectsMsg(ps)
	}
}

func (m Model) tagsCmd() tea.Cmd {
	s := m.store
	return func() tea.Msg {
		ts, err := s.Tags()
		if err != nil {
			return errMsg{err}
		}
		return tagsMsg(ts)
	}
}
```

import 補上 `"todo.mirumo.net/internal/store"`。

- [ ] **Step 4: 實作選單**

建立 `internal/tui/picker.go`：

```go
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"todo.mirumo.net/internal/project"
	"todo.mirumo.net/internal/store"
)

type pickerKind int

const (
	pickProject pickerKind = iota
	pickTag
)

// pickerItem 是選單的一列。clear 為 true 的那項代表「不過濾」。
type pickerItem struct {
	label string
	value string
	clear bool
}

type pickerState struct {
	kind   pickerKind
	items  []pickerItem
	cursor int
}

func projectItems(ps []store.ProjectCount) []pickerItem {
	items := []pickerItem{{label: "全部", clear: true}}
	for _, p := range ps {
		label := project.Label(p.Path)
		if label == "" {
			label = "（未分類）"
		}
		items = append(items, pickerItem{label: label, value: p.Path})
	}
	return items
}

func tagItems(tags []string) []pickerItem {
	items := []pickerItem{{label: "全部", clear: true}}
	for _, t := range tags {
		items = append(items, pickerItem{label: "@" + t, value: t})
	}
	return items
}

// updatePicker 處理選單模式的按鍵。
func (m Model) updatePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.mode = modeList
		return m, nil
	case "j", "down":
		if m.picker.cursor < len(m.picker.items)-1 {
			m.picker.cursor++
		}
	case "k", "up":
		if m.picker.cursor > 0 {
			m.picker.cursor--
		}
	case "enter":
		if m.picker.cursor >= len(m.picker.items) {
			m.mode = modeList
			return m, nil
		}
		it := m.picker.items[m.picker.cursor]
		switch m.picker.kind {
		case pickProject:
			if it.clear {
				m.filter.Project = nil
			} else {
				v := it.value
				m.filter.Project = &v
			}
		case pickTag:
			if it.clear {
				m.filter.Tags = nil
			} else {
				m.filter.Tags = []string{it.value}
			}
		}
		m.mode = modeList
		m.cursor = 0
		return m, m.loadCmd()
	}
	return m, nil
}

func (m Model) viewPicker() string {
	title := "依專案過濾"
	if m.picker.kind == pickTag {
		title = "依標籤過濾"
	}
	var b strings.Builder
	b.WriteString(title + "\n\n")
	for i, it := range m.picker.items {
		marker := "  "
		line := it.label
		if i == m.picker.cursor {
			marker = "▸ "
			line = styleCursor.Render(line)
		}
		b.WriteString(marker + line + "\n")
	}
	b.WriteString("\n" + styleHint.Render("j/k 移動 · enter 選擇 · esc 取消"))
	return b.String()
}
```

- [ ] **Step 5: 接上 Model**

`internal/tui/tui.go`：mode 加 `modePicker`，Model 加 `picker pickerState`，`Update` 加兩個 msg 分支與 mode 分派，`updateList` 加兩個按鍵，`View` 分派。

```go
const (
	modeList mode = iota
	modeSearch
	modePicker
)
```

```go
	case projectsMsg:
		m.picker = pickerState{kind: pickProject, items: projectItems(msg)}
		m.mode = modePicker
		return m, nil

	case tagsMsg:
		m.picker = pickerState{kind: pickTag, items: tagItems(msg)}
		m.mode = modePicker
		return m, nil
```

```go
		case modePicker:
			return m.updatePicker(msg)
```

`updateList` 加：

```go
	case "P":
		return m, m.projectsCmd()
	case "T":
		return m, m.tagsCmd()
```

`View` 改成：

```go
func (m Model) View() string {
	if m.mode == modePicker {
		return m.viewPicker()
	}
	return m.viewList()
}
```

footer 提示補上 `P/T 過濾`。

- [ ] **Step 6: 執行測試確認通過**

Run: `go test ./internal/tui/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/tui/
git commit --no-gpg-sign -m "feat(tui): 專案與標籤過濾選單"
```

---

### Task 18: 新增／編輯表單與說明覆蓋層

**Files:**
- Create: `internal/tui/form.go`
- Modify: `internal/tui/tui.go`（新增 `modeForm`、`modeHelp` 與按鍵）
- Modify: `internal/tui/cmds.go`（新增 `saveCmd`）
- Test: `internal/tui/form_test.go`

**Interfaces:**
- Consumes: `textinput.Model`、`task.ValidateTitle`、`task.ParsePriority`、`task.NormalizeTags`、`datearg.Parse`、`project.Current`、`store.Store.Add/Update`
- Produces: `formState`、`(Model).openForm(t task.Task, editing bool) Model`、`(Model).updateForm`、`(Model).viewForm`、`(Model).saveCmd(t task.Task, editing bool) tea.Cmd`、`modeForm`、`modeHelp`

- [ ] **Step 1: 寫失敗的測試**

建立 `internal/tui/form_test.go`：

```go
package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"todo.mirumo.net/internal/task"
)

// typeInto 依序把字串打進目前聚焦的欄位。
func typeInto(t *testing.T, m Model, s string) Model {
	t.Helper()
	return press(t, m, s)
}

func TestFormAddCreatesTask(t *testing.T) {
	m, s := newModel(t)
	m = press(t, m, "a")
	if m.mode != modeForm {
		t.Fatal("a 應該開啟表單")
	}
	m = typeInto(t, m, "第四件")
	m = press(t, m, "tab")
	m = press(t, m, "tab")
	m = typeInto(t, m, "急,雜")
	m = press(t, m, "tab")
	m = typeInto(t, m, "tomorrow")
	m = press(t, m, "tab")
	m = typeInto(t, m, "high")
	m = press(t, m, "enter")

	if m.mode != modeList {
		t.Fatalf("儲存後應回到清單，mode = %v", m.mode)
	}
	ts, err := s.List(task.Filter{Search: "第四件"}, refTime())
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) != 1 {
		t.Fatalf("應該新增了一筆，實得 %d 筆", len(ts))
	}
	got := ts[0]
	if got.Due == nil || got.Due.Format("2006-01-02") != "2026-08-30" {
		t.Errorf("due = %v", got.Due)
	}
	if got.Priority != task.PriHigh {
		t.Errorf("priority = %v", got.Priority)
	}
	if len(got.Tags) != 2 {
		t.Errorf("tags = %v，逗號分隔應拆成兩個", got.Tags)
	}
}

func TestFormEditPrefillsAndUpdates(t *testing.T) {
	m, s := newModel(t)
	id := m.tasks[0].ID
	m = press(t, m, "e")
	if m.mode != modeForm {
		t.Fatal("e 應該開啟表單")
	}
	if !strings.Contains(m.View(), "第一件") {
		t.Errorf("編輯時應預填現有值：\n%s", m.View())
	}
	// 清掉標題後重打。
	for range len([]rune("第一件")) {
		m = press(t, m, "backspace")
	}
	m = typeInto(t, m, "改過的")
	m = press(t, m, "enter")

	got, err := s.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "改過的" {
		t.Errorf("title = %q", got.Title)
	}
	if got.Due == nil {
		t.Error("沒動到的欄位應該保持原值")
	}
}

func TestFormRejectsEmptyTitle(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "a")
	m = press(t, m, "enter")
	if m.mode != modeForm {
		t.Error("標題空白時不該離開表單")
	}
	if !strings.Contains(m.View(), "標題不能是空的") {
		t.Errorf("應該說明為什麼存不了：\n%s", m.View())
	}
}

func TestFormRejectsBadDue(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "a")
	m = typeInto(t, m, "測試")
	m = press(t, m, "tab")
	m = press(t, m, "tab")
	m = press(t, m, "tab")
	m = typeInto(t, m, "someday")
	m = press(t, m, "enter")
	if m.mode != modeForm {
		t.Error("日期不合法時不該離開表單")
	}
	if !strings.Contains(m.View(), "看不懂的日期") {
		t.Errorf("應該指出是日期的問題：\n%s", m.View())
	}
}

func TestFormEscCancels(t *testing.T) {
	m, s := newModel(t)
	before, _ := s.List(task.Filter{}, refTime())
	m = press(t, m, "a")
	m = typeInto(t, m, "不要存")
	m = press(t, m, "esc")
	if m.mode != modeList {
		t.Error("esc 應該回到清單")
	}
	after, _ := s.List(task.Filter{}, refTime())
	if len(after) != len(before) {
		t.Error("esc 不該存進任何東西")
	}
}

func TestFormFillsProjectFromCwd(t *testing.T) {
	m, _ := newModel(t)
	if err := os.MkdirAll(filepath.Join(m.cwd, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	m = press(t, m, "a")
	m = press(t, m, "ctrl+p")
	if !strings.Contains(m.View(), filepath.Base(m.cwd)) {
		t.Errorf("ctrl+p 應該填入當前目錄的專案：\n%s", m.View())
	}
}

func TestHelpOverlay(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "?")
	if m.mode != modeHelp {
		t.Fatal("? 應該開啟說明")
	}
	v := m.View()
	for _, want := range []string{"space", "d", "u", "/", "P", "T"} {
		if !strings.Contains(v, want) {
			t.Errorf("說明缺少 %q：\n%s", want, v)
		}
	}
	m = press(t, m, "esc")
	if m.mode != modeList {
		t.Error("esc 應該關掉說明")
	}
}
```

- [ ] **Step 2: 執行測試確認失敗**

Run: `go test ./internal/tui/ -run 'TestForm|TestHelp'`
Expected: FAIL，`undefined: modeForm`

- [ ] **Step 3: 實作表單**

建立 `internal/tui/form.go`：

```go
package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"todo.mirumo.net/internal/datearg"
	"todo.mirumo.net/internal/project"
	"todo.mirumo.net/internal/task"
)

const (
	fieldTitle = iota
	fieldProject
	fieldTags
	fieldDue
	fieldPri
	fieldCount
)

var fieldLabels = [fieldCount]string{"標題", "專案", "標籤", "截止日", "優先度"}

// formState 是新增／編輯共用的表單。editing 為 false 時代表新增。
type formState struct {
	editing  bool
	original task.Task
	inputs   [fieldCount]textinput.Model
	focus    int
	errText  string
}

// openForm 準備一份表單。編輯時以現有值預填。
func (m Model) openForm(t task.Task, editing bool) Model {
	f := formState{editing: editing, original: t}
	values := [fieldCount]string{
		t.Title,
		t.Project,
		strings.Join(t.Tags, ","),
		"",
		t.Priority.String(),
	}
	if t.Due != nil {
		values[fieldDue] = t.Due.Format("2006-01-02")
	}
	placeholders := [fieldCount]string{
		"要做什麼",
		"留空 = 全域未分類（ctrl+p 填入當前目錄）",
		"逗號分隔",
		"tomorrow、fri、+3d、2026-09-01",
		"low、med、high",
	}
	for i := range f.inputs {
		ti := textinput.New()
		ti.Prompt = ""
		ti.SetValue(values[i])
		ti.Placeholder = placeholders[i]
		ti.CursorEnd()
		f.inputs[i] = ti
	}
	f.inputs[fieldTitle].Focus()
	m.form = f
	m.mode = modeForm
	return m
}

func (m Model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		return m, nil
	case "tab", "down":
		m.form.inputs[m.form.focus].Blur()
		m.form.focus = (m.form.focus + 1) % fieldCount
		m.form.inputs[m.form.focus].Focus()
		return m, nil
	case "shift+tab", "up":
		m.form.inputs[m.form.focus].Blur()
		m.form.focus = (m.form.focus - 1 + fieldCount) % fieldCount
		m.form.inputs[m.form.focus].Focus()
		return m, nil
	case "ctrl+p":
		p, err := project.Current(m.cwd)
		if err != nil {
			m.form.errText = err.Error()
			return m, nil
		}
		m.form.inputs[fieldProject].SetValue(p)
		m.form.inputs[fieldProject].CursorEnd()
		return m, nil
	case "enter":
		t, err := m.formTask()
		if err != nil {
			m.form.errText = err.Error()
			return m, nil
		}
		m.mode = modeList
		return m, m.saveCmd(t, m.form.editing)
	}
	// 同 updateSearch：不轉傳游標閃爍的計時器 cmd。
	m.form.inputs[m.form.focus], _ = m.form.inputs[m.form.focus].Update(msg)
	m.form.errText = ""
	return m, nil
}

// formTask 把表單內容組成一筆 Task，任何欄位不合法就回傳錯誤。
func (m Model) formTask() (task.Task, error) {
	f := m.form
	t := f.original
	now := m.now()

	title, err := task.ValidateTitle(f.inputs[fieldTitle].Value())
	if err != nil {
		return task.Task{}, err
	}
	t.Title = title
	t.Project = strings.TrimSpace(f.inputs[fieldProject].Value())
	t.Tags = task.NormalizeTags(strings.Split(f.inputs[fieldTags].Value(), ","))

	if v := strings.TrimSpace(f.inputs[fieldDue].Value()); v == "" {
		t.Due = nil
	} else {
		d, err := datearg.Parse(v, now)
		if err != nil {
			return task.Task{}, err
		}
		t.Due = &d
	}
	if t.Priority, err = task.ParsePriority(f.inputs[fieldPri].Value()); err != nil {
		return task.Task{}, err
	}

	t.UpdatedAt = now
	if !f.editing {
		t.CreatedAt = now
	}
	return t, nil
}

func (m Model) viewForm() string {
	title := "新增待辦"
	if m.form.editing {
		title = "編輯 #" + itoa(m.form.original.ID)
	}
	var b strings.Builder
	b.WriteString(title + "\n\n")
	for i, in := range m.form.inputs {
		marker := "  "
		if i == m.form.focus {
			marker = "▸ "
		}
		b.WriteString(marker + pad(fieldLabels[i], 8) + in.View() + "\n")
	}
	b.WriteString("\n")
	if m.form.errText != "" {
		b.WriteString(styleErr.Render(m.form.errText) + "\n")
	}
	b.WriteString(styleHint.Render("tab 換欄 · ctrl+p 填入當前目錄 · enter 儲存 · esc 取消"))
	return b.String()
}
```

`itoa` 放在 `internal/tui/list.go`：

```go
func itoa(n int64) string { return strconv.FormatInt(n, 10) }
```

（import 補 `"strconv"`。）

- [ ] **Step 4: 實作儲存 cmd**

在 `internal/tui/cmds.go` 追加：

```go
func (m Model) saveCmd(t task.Task, editing bool) tea.Cmd {
	s := m.store
	return func() tea.Msg {
		if editing {
			if err := s.Update(t); err != nil {
				return errMsg{err}
			}
			return savedMsg{note: "已更新「" + t.Title + "」"}
		}
		if _, err := s.Add(t); err != nil {
			return errMsg{err}
		}
		return savedMsg{note: "已新增「" + t.Title + "」"}
	}
}
```

- [ ] **Step 5: 接上 Model 與說明覆蓋層**

`internal/tui/tui.go`：

```go
const (
	modeList mode = iota
	modeSearch
	modePicker
	modeForm
	modeHelp
)
```

Model 加 `form formState`。

`Update` 的 mode 分派加：

```go
		case modeForm:
			return m.updateForm(msg)
		case modeHelp:
			m.mode = modeList
			return m, nil
```

`updateList` 加：

```go
	case "a":
		return m.openForm(task.Task{}, false), nil
	case "e":
		if t, ok := m.current(); ok {
			return m.openForm(t, true), nil
		}
	case "?":
		m.mode = modeHelp
		return m, nil
```

`View`：

```go
func (m Model) View() string {
	switch m.mode {
	case modePicker:
		return m.viewPicker()
	case modeForm:
		return m.viewForm()
	case modeHelp:
		return viewHelp()
	}
	return m.viewList()
}
```

在 `internal/tui/list.go` 加說明畫面：

```go
func viewHelp() string {
	rows := [][2]string{
		{"j / k / ↑ / ↓", "移動"},
		{"g / G", "跳到頂 / 底"},
		{"space", "切換完成"},
		{"a / e", "新增 / 編輯"},
		{"d", "刪除"},
		{"u", "復原最近一次刪除"},
		{"/", "搜尋標題"},
		{"P / T", "依專案 / 標籤過濾"},
		{"A", "切換是否顯示已完成"},
		{"s", "循環排序"},
		{"esc", "清除所有過濾"},
		{"?", "這份說明"},
		{"q", "離開"},
	}
	var b strings.Builder
	b.WriteString("按鍵說明\n\n")
	for _, r := range rows {
		b.WriteString("  " + pad(r[0], 16) + r[1] + "\n")
	}
	b.WriteString("\n" + styleHint.Render("按任意鍵返回"))
	return b.String()
}
```

`updateList` 的 `case "a"` 需要 import `"todo.mirumo.net/internal/task"`（`tui.go` 已有）。

footer 提示更新為：

```go
	return styleHint.Render("a 新增 · e 編輯 · space 完成 · d 刪除 · / 搜尋 · P/T 過濾 · ? 說明 · q 離開")
```

- [ ] **Step 6: 執行整包測試**

Run: `go test ./... -v`
Expected: 全部 PASS

- [ ] **Step 7: 手動驗收**

```bash
go build -o /tmp/todo ./cmd/todo
rm -f /tmp/todo-final.db
export TODO_DB=/tmp/todo-final.db
/tmp/todo add "買牛奶" -t 購物 -d tomorrow --pri high
/tmp/todo add "修 bug" -p
/tmp/todo ls
/tmp/todo projects
/tmp/todo tui
```
Expected：TUI 中依序驗證 `a` 新增、`e` 編輯（含 `ctrl+p` 填入目錄）、`space` 勾選、`d` 刪除後 `u` 復原、`/` 搜尋、`P`/`T` 過濾、`A` 顯示已完成、`s` 換排序、`?` 說明、`q` 離開，且離開後 `/tmp/todo ls` 反映所有變更。

- [ ] **Step 8: Commit**

```bash
git add internal/tui/
git commit --no-gpg-sign -m "feat(tui): 新增／編輯表單與說明覆蓋層"
```

---

## 收尾檢查

- [ ] `go vet ./...` 無輸出
- [ ] `gofmt -l .` 無輸出
- [ ] `go test ./...` 全綠
- [ ] `grep -rn "flag\." --include=*.go . | grep -v argparse` 無標準庫 flag 的使用
- [ ] `go list -m all | grep -E 'cobra|pflag'` 無結果
- [ ] `ls ~/.todo` 在跑過測試後仍不存在（除非手動驗收時建立過）
