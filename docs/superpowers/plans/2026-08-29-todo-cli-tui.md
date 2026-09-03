# todo CLI + TUI implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `todo`, a purely local task tool: the CLI is the main interface, `todo tui` enters a Bubble Tea browsing mode, and the data lives in `~/.todo/todo.db`.

**Architecture:** Layered from the outside in, with no IO in the inner layers. `argparse`, `task`, `datearg` and `project` are pure functions; `store` defines the `Store` interface and implements it over SQLite; `cli` and `tui` depend only on `Store` and never import each other (`cli` calls the TUI through an injected `RunTUI` function); `cmd/todo` does the wiring.

**Tech Stack:** Go 1.26, Bubble Tea + Bubbles + Lip Gloss, `modernc.org/sqlite` (pure Go, no cgo), hand-written argument parsing.

Design document: `docs/superpowers/specs/2026-08-29-todo-cli-tui-design.md`

## Global Constraints

- Go 1.26; module path `todo.mirumo.net`; binary named `todo`
- Only these four dependencies are allowed: `github.com/charmbracelet/bubbletea@v1`, `github.com/charmbracelet/bubbles@v0`, `github.com/charmbracelet/lipgloss@v1`, `modernc.org/sqlite@v1`. No Cobra, no pflag, and no stdlib `flag`
- The database defaults to `~/.todo/todo.db` with the directory at mode 0700; the `--db <path>` flag and the `TODO_DB` environment variable override it
- **No test may read or write `~/.todo`**. Use `t.TempDir()` when real files are needed; store tests use `:memory:`
- Every user-facing message, error and piece of TUI text is in Traditional Chinese
- Every task is TDD: write a failing test -> confirm it fails -> minimal implementation -> confirm it passes -> commit
- Commit with `git commit --no-gpg-sign` (pinentry cannot start without a TTY in this environment)
- Time formats: timestamps are stored as RFC3339, due dates as `YYYY-MM-DD`
- An empty `project` is a legitimate first-class state meaning "uncategorized, global", not a missing value

---

## File Structure

| File | Responsibility |
|---|---|
| `go.mod` | module `todo.mirumo.net` |
| `internal/argparse/argparse.go` | GNU-style argument parsing, including optional-value flags |
| `internal/task/task.go` | `Task`, `Priority`, `Filter`, `SortBy`, validation |
| `internal/datearg/datearg.go` | Date parsing and human-readable display |
| `internal/project/project.go` | Derives a project path from a directory (walking up for `.git`) |
| `internal/store/store.go` | The `Store` interface, `ProjectCount`, `ErrNotFound` |
| `internal/store/sqlite.go` | The SQLite implementation and its schema |
| `internal/cli/app.go` | `App`, `Run` dispatch, `SplitGlobal`, usage |
| `internal/cli/format.go` | List layout and colour |
| `internal/cli/cmd_add.go` | `add` |
| `internal/cli/cmd_ls.go` | `ls` |
| `internal/cli/cmd_mark.go` | `done` / `undone` / `rm` |
| `internal/cli/cmd_edit.go` | `edit` |
| `internal/cli/cmd_meta.go` | `projects` / `tags` |
| `internal/tui/tui.go` | The root Model, Init/Update/View, `Run` |
| `internal/tui/cmds.go` | The `tea.Cmd`s and msg types |
| `internal/tui/list.go` | List rendering |
| `internal/tui/picker.go` | The project / tag menus |
| `internal/tui/form.go` | The add / edit form |
| `cmd/todo/main.go` | Wiring and the exit code |

---

### Task 1: Project skeleton and argument parsing

**Files:**
- Create: `go.mod`
- Create: `internal/argparse/argparse.go`
- Test: `internal/argparse/argparse_test.go`

**Interfaces:**
- Consumes: nothing
- Produces: `argparse.Kind` (`Bool` / `String` / `StringSlice` / `OptionalString`), `argparse.Spec{Long, Short string; Kind Kind; Usage string}`, `argparse.New(...Spec) *Set`, `(*Set).Parse([]string) (*Result, error)`, `(*Result).Changed(long string) bool`, `.Bool(long string) bool`, `.String(long string) string`, `.Strings(long string) []string`, `.Optional(long string) (string, bool)`, `.Args() []string`, `(*Set).Usage() string`

- [ ] **Step 1: Create the module skeleton**

```bash
go mod init todo.mirumo.net
mkdir -p internal/argparse internal/task internal/datearg internal/project internal/store internal/cli internal/tui cmd/todo
```

- [ ] **Step 2: Write the failing test**

Create `internal/argparse/argparse_test.go`:

```go
package argparse

import (
	"strings"
	"testing"
)

func specs() *Set {
	return New(
		Spec{Long: "all", Short: "a", Kind: Bool, Usage: "including done"},
		Spec{Long: "due", Short: "d", Kind: String, Usage: "Due date"},
		Spec{Long: "tag", Short: "t", Kind: StringSlice, Usage: "Tag; repeatable"},
		Spec{Long: "project", Short: "p", Kind: OptionalString, Usage: "Project"},
	)
}

func TestParseLongAndShortForms(t *testing.T) {
	r, err := specs().Parse([]string{"buy milk", "--due", "2026-09-01", "-a", "-t", "shopping", "--tag=chores"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := r.Args(); len(got) != 1 || got[0] != "buy milk" {
		t.Errorf("positional args = %v, want [buy milk]", got)
	}
	if !r.Bool("all") {
		t.Error("--all should be true")
	}
	if got := r.String("due"); got != "2026-09-01" {
		t.Errorf("due = %q, want 2026-09-01", got)
	}
	if got := r.Strings("tag"); len(got) != 2 || got[0] != "shopping" || got[1] != "chores" {
		t.Errorf("tag = %v, want [shopping chores]", got)
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
		{"absent", []string{"x"}, false, false, ""},
		{"given without a value", []string{"x", "-p"}, true, false, ""},
		{"no value, another flag next", []string{"x", "-p", "-a"}, true, false, ""},
		{"value after a space", []string{"x", "-p", "work"}, true, true, "work"},
		{"value after an equals sign", []string{"x", "--project=work"}, true, true, "work"},
		{"empty value after an equals sign", []string{"x", "-p="}, true, true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := specs().Parse(c.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if r.Changed("project") != c.changed {
				t.Errorf("Changed = %v, want %v", r.Changed("project"), c.changed)
			}
			v, has := r.Optional("project")
			if has != c.hasValue || v != c.value {
				t.Errorf("Optional = (%q, %v), want (%q, %v)", v, has, c.value, c.hasValue)
			}
		})
	}
}

func TestStringFlagAcceptsEmptyValue(t *testing.T) {
	r, err := specs().Parse([]string{"--due", ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.Changed("due") || r.String("due") != "" {
		t.Error("--due \"\" should count as given with an empty value")
	}
}

func TestDoubleDashEndsFlags(t *testing.T) {
	r, err := specs().Parse([]string{"--", "-a", "--due"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := r.Args(); len(got) != 2 || got[0] != "-a" || got[1] != "--due" {
		t.Errorf("positional args = %v, want [-a --due]", got)
	}
	if r.Bool("all") {
		t.Error("nothing after -- should be parsed as a flag")
	}
}

func TestErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"unknown long flag", []string{"--nope"}, "unknown flag --nope"},
		{"unknown short flag", []string{"-z"}, "unknown flag -z"},
		{"string flag with no value", []string{"--due"}, "flag --due needs a value"},
		{"bool flag given a value", []string{"--all=1"}, "flag --all takes no value"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := specs().Parse(c.args)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want it to contain %q", err, c.want)
			}
		})
	}
}

func TestUsageListsFlags(t *testing.T) {
	u := specs().Usage()
	for _, want := range []string{"-a, --all", "-d, --due", "including done"} {
		if !strings.Contains(u, want) {
			t.Errorf("Usage is missing %q, got:\n%s", want, u)
		}
	}
}
```

- [ ] **Step 3: Run the test and confirm it fails**

Run: `go test ./internal/argparse/`
Expected: FAIL, a compile error `undefined: New`

- [ ] **Step 4: Implement**

Create `internal/argparse/argparse.go`:

```go
// Package argparse provides GNU-style command line argument parsing.
//
// Neither pflag nor the stdlib flag is used: -p/--project needs an optional value —
// no value means the current directory, a value names a project. Neither library has it.
package argparse

import (
	"fmt"
	"strings"
)

// Kind decides how a flag consumes the tokens after it.
type Kind int

const (
	// Bool never consumes the next token.
	Bool Kind = iota
	// String requires a value; a missing one is an error.
	String
	// StringSlice is String, but may repeat and accumulates a list.
	StringSlice
	// OptionalString consumes the next token if it exists and does not start with "-", otherwise it counts as given without a value.
	OptionalString
)

// Spec describes one flag. Short carries no leading "-" and may be empty.
type Spec struct {
	Long  string
	Short string
	Kind  Kind
	Usage string
}

// Set is one subcommand's flag definitions.
type Set struct{ specs []Spec }

// New builds a Set.
func New(specs ...Spec) *Set { return &Set{specs: specs} }

type value struct {
	set      bool
	hasValue bool
	str      string
	strs     []string
}

// Result is the outcome of one parse.
type Result struct {
	vals map[string]*value
	args []string
}

// Changed reports whether the flag appeared on the command line.
func (r *Result) Changed(long string) bool {
	v, ok := r.vals[long]
	return ok && v.set
}

// Bool reports whether a bool flag was given.
func (r *Result) Bool(long string) bool { return r.Changed(long) }

// String returns a string flag's value, empty when it was not given (Changed tells the two apart).
func (r *Result) String(long string) string {
	if v, ok := r.vals[long]; ok {
		return v.str
	}
	return ""
}

// Strings returns the values a repeatable flag accumulated.
func (r *Result) Strings(long string) []string {
	if v, ok := r.vals[long]; ok {
		return v.strs
	}
	return nil
}

// Optional returns an optional-value flag's value and whether it carried one.
// The three states: Changed false is absent; Changed true with hasValue false is given without a value.
func (r *Result) Optional(long string) (string, bool) {
	v, ok := r.vals[long]
	if !ok || !v.set {
		return "", false
	}
	return v.str, v.hasValue
}

// Args returns the positional arguments.
func (r *Result) Args() []string { return r.args }

// Usage renders the flag help text.
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

// Parse parses args (without the program name or the subcommand name).
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
				return nil, fmt.Errorf("unknown flag --%s", name)
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
				return nil, fmt.Errorf("unknown flag -%s", name)
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

// assign applies one flag and reports how many tokens it consumed.
func (s *Set) assign(r *Result, sp Spec, inline string, hasInline bool, args []string, i int) (int, error) {
	v := r.vals[sp.Long]
	v.set = true
	switch sp.Kind {
	case Bool:
		if hasInline {
			return 0, fmt.Errorf("flag --%s takes no value", sp.Long)
		}
		return 1, nil
	case String, StringSlice:
		val, used := inline, 1
		if !hasInline {
			if i+1 >= len(args) {
				return 0, fmt.Errorf("flag --%s needs a value", sp.Long)
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

- [ ] **Step 5: Run the test and confirm it passes**

Run: `go test ./internal/argparse/ -v`
Expected: PASS, every case green

- [ ] **Step 6: Commit**

```bash
git add go.mod internal/argparse/
git commit --no-gpg-sign -m "feat(argparse): GNU-style argument parsing with optional-value flags"
```

---

### Task 2: Domain types

**Files:**
- Create: `internal/task/task.go`
- Test: `internal/task/task_test.go`

**Interfaces:**
- Consumes: nothing
- Produces: `task.Priority` (`PriNone`/`PriLow`/`PriMed`/`PriHigh`), `task.ParsePriority(string) (Priority, error)`, `(Priority).String() string`, `(Priority).Label() string`, `task.Task{ID int64; Title, Project string; Due *time.Time; Priority Priority; DoneAt *time.Time; Tags []string; CreatedAt, UpdatedAt time.Time}`, `(Task).Done() bool`, `task.ValidateTitle(string) (string, error)`, `task.SortBy` (`SortDue`/`SortPriority`/`SortCreated`), `task.ParseSortBy(string) (SortBy, error)`, `task.DueRange` (`DueAny`/`DueToday`/`DueWeek`/`DueOverdue`/`DueOn`), `task.Filter`, `task.NormalizeTags([]string) []string`

- [ ] **Step 1: Write the failing test**

Create `internal/task/task_test.go`:

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
			t.Errorf("ParsePriority(%q) err = %v, want an error = %v", c.in, err, c.wantErr)
			continue
		}
		if err == nil && got != c.want {
			t.Errorf("ParsePriority(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestPriorityOrderingIsAscending(t *testing.T) {
	if !(PriNone < PriLow && PriLow < PriMed && PriMed < PriHigh) {
		t.Error("Priority must increase from low to high, so SQL can ORDER BY priority DESC")
	}
}

func TestValidateTitle(t *testing.T) {
	got, err := ValidateTitle("  buy milk  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "buy milk" {
		t.Errorf("= %q, want buy milk with the surrounding spaces trimmed", got)
	}
	if _, err := ValidateTitle("   "); err == nil {
		t.Error("a title of nothing but spaces should be an error")
	}
}

func TestDone(t *testing.T) {
	if (Task{}).Done() {
		t.Error("Done() should be false while DoneAt is nil")
	}
	now := time.Now()
	if !(Task{DoneAt: &now}).Done() {
		t.Error("Done() should be true once DoneAt is set")
	}
}

func TestNormalizeTags(t *testing.T) {
	got := NormalizeTags([]string{" shopping ", "chores", "shopping", ""})
	if len(got) != 2 || got[0] != "shopping" || got[1] != "chores" {
		t.Errorf("= %v, want [shopping chores]: trimmed, deduplicated, empties dropped, first-seen order kept", got)
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
		t.Error("an unknown sort field should be an error")
	}
}
```

- [ ] **Step 2: Run the test and confirm it fails**

Run: `go test ./internal/task/`
Expected: FAIL, `undefined: ParsePriority`

- [ ] **Step 3: Implement**

Create `internal/task/task.go`:

```go
// Package task defines the domain types of a task. It does no IO.
package task

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Priority is how urgent a task is. The values increase from low to high, so SQL can ORDER BY them directly.
type Priority int

const (
	PriNone Priority = iota
	PriLow
	PriMed
	PriHigh
)

// ParsePriority parses what the user typed. An empty string means unset.
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
	return PriNone, fmt.Errorf("unrecognised priority %q (use low, med or high)", s)
}

// String returns the code the CLI uses.
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

// Label returns the label shown to the reader.
func (p Priority) Label() string {
	switch p {
	case PriLow:
		return "Low"
	case PriMed:
		return "Med"
	case PriHigh:
		return "High"
	}
	return ""
}

// Task is one task. An empty Project means globally uncategorized.
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

// Done reports whether the task is finished.
func (t Task) Done() bool { return t.DoneAt != nil }

// ValidateTitle trims the surrounding spaces and rejects what is left empty.
func ValidateTitle(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("a title cannot be empty")
	}
	return s, nil
}

// NormalizeTags trims, drops empties and duplicates, and keeps first-seen order.
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

// SortBy is how a list is ordered.
type SortBy int

const (
	SortDue SortBy = iota
	SortPriority
	SortCreated
)

// ParseSortBy parses the value of -s.
func ParseSortBy(s string) (SortBy, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "due":
		return SortDue, nil
	case "pri":
		return SortPriority, nil
	case "created":
		return SortCreated, nil
	}
	return SortDue, fmt.Errorf("unrecognised sort %q (use due, pri or created)", s)
}

// DueRange is the due-date range a query narrows to.
type DueRange int

const (
	DueAny DueRange = iota
	DueToday
	DueWeek
	DueOverdue
	DueOn
)

// Filter describes one query. A nil Project means no project filter;
// a pointer to an empty string means only the globally uncategorized ones.
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

- [ ] **Step 4: Run the test and confirm it passes**

Run: `go test ./internal/task/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/task/
git commit --no-gpg-sign -m "feat(task): task domain types and validation"
```

---

### Task 3: Date parsing and display

**Files:**
- Create: `internal/datearg/datearg.go`
- Test: `internal/datearg/datearg_test.go`

**Interfaces:**
- Consumes: nothing
- Produces: `datearg.Parse(s string, now time.Time) (time.Time, error)`, `datearg.Format(due, now time.Time) string`, `datearg.Day(t time.Time) time.Time`

- [ ] **Step 1: Write the failing test**

Create `internal/datearg/datearg_test.go`:

```go
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
		{"sat", "2026-08-29"},  // today is Saturday, so it means today
		{"mon", "2026-08-31"},  // the first Monday within the next seven days
		{"+3d", "2026-09-01"},
		{"+2w", "2026-09-12"},
		{"2026-12-25", "2026-12-25"},
		{"  TOMORROW  ", "2026-08-30"},
	}
	for _, c := range cases {
		got, err := Parse(c.in, ref())
		if err != nil {
			t.Errorf("Parse(%q) unexpected error: %v", c.in, err)
			continue
		}
		if got.Format("2006-01-02") != c.want {
			t.Errorf("Parse(%q) = %s, want %s", c.in, got.Format("2006-01-02"), c.want)
		}
	}
}

func TestParseReturnsMidnight(t *testing.T) {
	got, err := Parse("today", ref())
	if err != nil {
		t.Fatal(err)
	}
	if got.Hour() != 0 || got.Minute() != 0 || got.Second() != 0 {
		t.Errorf("= %v, want midnight that day", got)
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	for _, in := range []string{"someday", "2026-13-45", "+3x", "", "+d"} {
		if _, err := Parse(in, ref()); err == nil {
			t.Errorf("Parse(%q) should be an error", in)
		}
	}
}

func TestFormat(t *testing.T) {
	cases := []struct {
		due  string
		want string
	}{
		{"2026-08-27", "2d overdue"},
		{"2026-08-28", "1d overdue"},
		{"2026-08-29", "today"},
		{"2026-08-30", "tomorrow"},
		{"2026-08-31", "Mon"},
		{"2026-09-04", "Fri"},
		{"2026-09-05", "09-05"},
		{"2027-01-02", "2027-01-02"},
	}
	for _, c := range cases {
		due, err := time.ParseInLocation("2006-01-02", c.due, time.Local)
		if err != nil {
			t.Fatal(err)
		}
		if got := Format(due, ref()); got != c.want {
			t.Errorf("Format(%s) = %q, want %q", c.due, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run the test and confirm it fails**

Run: `go test ./internal/datearg/`
Expected: FAIL, `undefined: Parse`

- [ ] **Step 3: Implement**

Create `internal/datearg/datearg.go`:

```go
// Package datearg parses due dates and renders them for people to read.
package datearg

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Day truncates a time to midnight that day in the local zone.
func Day(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

var weekdays = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday,
	"wed": time.Wednesday, "thu": time.Thursday,
	"fri": time.Friday, "sat": time.Saturday,
}

var weekdayName = [7]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

// Parse parses a due date the user typed and returns midnight that day, local time.
// It accepts today / tomorrow / yesterday, weekday abbreviations, +3d, +2w and YYYY-MM-DD.
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
		// The first matching day within the next seven, today included.
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
	return time.Time{}, fmt.Errorf("unrecognised date %q (try today, tomorrow, fri, +3d, 2026-09-01)", s)
}

// Format renders a due date: Nd overdue / today / tomorrow / Fri / 09-05 / 2027-01-02.
func Format(due, now time.Time) string {
	d, base := Day(due), Day(now)
	diff := int(math.Round(d.Sub(base).Hours() / 24))
	switch {
	case diff < 0:
		return fmt.Sprintf("%dd overdue", -diff)
	case diff == 0:
		return "today"
	case diff == 1:
		return "tomorrow"
	case diff < 7:
		return weekdayName[int(d.Weekday())]
	case d.Year() == base.Year():
		return d.Format("01-02")
	default:
		return d.Format("2006-01-02")
	}
}
```

- [ ] **Step 4: Run the test and confirm it passes**

Run: `go test ./internal/datearg/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/datearg/
git commit --no-gpg-sign -m "feat(datearg): due date parsing and human-readable display"
```

---

### Task 4: Deriving the project path

**Files:**
- Create: `internal/project/project.go`
- Test: `internal/project/project_test.go`

**Interfaces:**
- Consumes: nothing
- Produces: `project.Current(dir string) (string, error)`, `project.Label(path string) string`

- [ ] **Step 1: Write the failing test**

Create `internal/project/project_test.go`:

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
		t.Errorf("= %q, want the repository root %q", gotResolved, want)
	}
}

func TestCurrentFallsBackToDir(t *testing.T) {
	dir := t.TempDir()
	got, err := Current(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("= %q, want an absolute path", got)
	}
	gotResolved, _ := filepath.EvalSymlinks(got)
	want, _ := filepath.EvalSymlinks(dir)
	if gotResolved != want {
		t.Errorf("= %q, with no .git it should return the directory itself %q", gotResolved, want)
	}
}

func TestCurrentAcceptsGitFile(t *testing.T) {
	// A git worktree's .git is a file, not a directory.
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
		t.Errorf("= %q, a .git file should still count as the repository root %q", gotResolved, want)
	}
}

func TestLabel(t *testing.T) {
	if got := Label("/Users/me/Projects/todo.mirumo.net"); got != "todo.mirumo.net" {
		t.Errorf("= %q, want the basename", got)
	}
	if got := Label(""); got != "" {
		t.Errorf("= %q, an empty string should come back unchanged (globally uncategorized)", got)
	}
}
```

- [ ] **Step 2: Run the test and confirm it fails**

Run: `go test ./internal/project/`
Expected: FAIL, `undefined: Current`

- [ ] **Step 3: Implement**

Create `internal/project/project.go`:

```go
// Package project derives the project a task belongs to from a filesystem location.
package project

import (
	"os"
	"path/filepath"
)

// Current walks up from dir looking for .git; it returns that directory when one is found, and dir itself otherwise.
// The path is always absolute — directory names collide (two repositories can both have docs/), only paths are unique.
func Current(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for d := abs; ; {
		// A git worktree's .git is a file and an ordinary repository's is a directory; both count.
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

// Label returns the short name to display. An empty string means globally uncategorized and comes back unchanged.
func Label(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Base(path)
}
```

- [ ] **Step 4: Run the test and confirm it passes**

Run: `go test ./internal/project/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/project/
git commit --no-gpg-sign -m "feat(project): derive the project path from the current directory"
```

---

### Task 5: The Store interface and the SQLite basics (schema, Add, Get, Close)

**Files:**
- Create: `internal/store/store.go`
- Create: `internal/store/sqlite.go`
- Test: `internal/store/sqlite_test.go`

**Interfaces:**
- Consumes: `task.Task`, `task.Priority`, `task.Filter`
- Produces: the `store.Store` interface, `store.ProjectCount{Path string; Open int}`, `store.ErrNotFound`, `store.OpenSQLite(path string) (Store, error)`

The whole `Store` interface (defined by this task, implemented by the two that follow):

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

- [ ] **Step 1: Add the SQLite dependency**

```bash
go get modernc.org/sqlite@v1
```

- [ ] **Step 2: Write the failing test**

Create `internal/store/sqlite_test.go`:

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

// newStore opens an in-memory store, so tests never touch ~/.todo.
func newStore(t *testing.T) Store {
	t.Helper()
	s, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func sample() task.Task {
	return task.Task{
		Title:     "buy milk",
		Project:   "/Users/me/Projects/home",
		Due:       day(2026, 9, 1),
		Priority:  task.PriHigh,
		Tags:      []string{"shopping", "chores"},
		CreatedAt: ref(),
		UpdatedAt: ref(),
	}
}

func TestAddAssignsIDAndRoundTrips(t *testing.T) {
	s := newStore(t)
	got, err := s.Add(sample())
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if got.ID == 0 {
		t.Fatal("Add should fill in the ID")
	}
	back, err := s.Get(got.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if back.Title != "buy milk" || back.Project != "/Users/me/Projects/home" {
		t.Errorf("the title or project was not stored: %+v", back)
	}
	if back.Priority != task.PriHigh {
		t.Errorf("priority = %v, want high", back.Priority)
	}
	if back.Due == nil || back.Due.Format("2006-01-02") != "2026-09-01" {
		t.Errorf("due = %v, want 2026-09-01", back.Due)
	}
	if back.Done() {
		t.Error("a new task should not be done")
	}
	if len(back.Tags) != 2 {
		t.Errorf("tags = %v, want two", back.Tags)
	}
}

func TestAddAcceptsEmptyProjectAndNilDue(t *testing.T) {
	s := newStore(t)
	in := task.Task{Title: "pay rent", CreatedAt: ref(), UpdatedAt: ref()}
	got, err := s.Add(in)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	back, err := s.Get(got.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.Project != "" {
		t.Errorf("project = %q, globally uncategorized should be an empty string", back.Project)
	}
	if back.Due != nil {
		t.Errorf("due = %v, want nil", back.Due)
	}
	if len(back.Tags) != 0 {
		t.Errorf("tags = %v, want none", back.Tags)
	}
}

func TestGetMissingReturnsErrNotFound(t *testing.T) {
	s := newStore(t)
	_, err := s.Get(999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestIDsAreNotReused(t *testing.T) {
	s := newStore(t)
	a, _ := s.Add(sample())
	if err := s.Delete(a.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	b, _ := s.Add(sample())
	if b.ID == a.ID {
		t.Errorf("the id was reused (%d); AUTOINCREMENT should guarantee it is not", b.ID)
	}
}
```

(`TestIDsAreNotReused` depends on `Delete` from Task 7; this task gets the other tests passing, and until Task 7 lands that case will not compile — so this task's `sqlite.go` first puts a skeleton of every interface method in place, the unimplemented ones returning `errors.New("not implemented yet")`.)

- [ ] **Step 3: Run the test and confirm it fails**

Run: `go test ./internal/store/`
Expected: FAIL, `undefined: OpenSQLite`

- [ ] **Step 4: Implement the interface**

Create `internal/store/store.go`:

```go
// Package store persists tasks.
package store

import (
	"errors"
	"time"

	"todo.mirumo.net/internal/task"
)

// ErrNotFound means no task carries that id.
var ErrNotFound = errors.New("task not found")

// ProjectCount is one project and how much is open in it.
type ProjectCount struct {
	Path string
	Open int
}

// Store is the storage interface for tasks. The CLI and the TUI know only this
// interface, so tests swap in a :memory: implementation and never touch the user's real data.
type Store interface {
	// Add inserts one task and returns it with its ID.
	Add(t task.Task) (task.Task, error)
	// Get fetches one task by id, returning ErrNotFound when there is none.
	Get(id int64) (task.Task, error)
	// List queries by f; now resolves relative conditions such as today/week/overdue.
	List(f task.Filter, now time.Time) ([]task.Task, error)
	// Update overwrites every field of t.ID, tags included.
	Update(t task.Task) error
	// Delete removes one task along with its tag links.
	Delete(id int64) error
	// SetDone marks a task done or reopens it.
	SetDone(id int64, done bool, now time.Time) error
	// Restore reinserts a task under its original t.ID, for the TUI's undo.
	Restore(t task.Task) error
	// Tags lists the tags at least one task references.
	Tags() ([]string, error)
	// Projects lists every project with its open count.
	Projects() ([]ProjectCount, error)
	Close() error
}
```

Create `internal/store/sqlite.go`:

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

// OpenSQLite opens the database, creating it when needed. path is a file path or ":memory:".
func OpenSQLite(path string) (Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// A single-user CLI needs no connection pool; one connection is the limit, which is
	// what makes the PRAGMA apply to every later query (SQLite defaults foreign keys off).
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

// setTags overwrites one task's tag links.
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

// The four methods below are implemented in Task 6 and Task 7.
func (s *sqlStore) List(f task.Filter, now time.Time) ([]task.Task, error) {
	return nil, errors.New("not implemented yet")
}
func (s *sqlStore) Update(t task.Task) error { return errors.New("not implemented yet") }
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
	return errors.New("not implemented yet")
}
func (s *sqlStore) Restore(t task.Task) error         { return errors.New("not implemented yet") }
func (s *sqlStore) Tags() ([]string, error)           { return nil, errors.New("not implemented yet") }
func (s *sqlStore) Projects() ([]ProjectCount, error) { return nil, errors.New("not implemented yet") }
```

- [ ] **Step 5: Run the test and confirm it passes**

Run: `go test ./internal/store/ -v`
Expected: PASS (all five cases green)

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/store/
git commit --no-gpg-sign -m "feat(store): the Store interface, the SQLite schema, Add/Get/Delete"
```

---

### Task 6: Queries and filtering

**Files:**
- Modify: `internal/store/sqlite.go` (replacing the `List` skeleton)
- Test: `internal/store/list_test.go`

**Interfaces:**
- Consumes: `task.Filter`, `task.SortBy`, `task.DueRange`, and Task 5's `sqlStore`, `scanTask`, `loadTags`
- Produces: a working `(*sqlStore).List(f task.Filter, now time.Time) ([]task.Task, error)`

- [ ] **Step 1: Write the failing test**

Create `internal/store/list_test.go`:

```go
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
	// Dated tasks run from soonest to furthest, undated ones last.
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
		t.Errorf("= %v, want high first, then med", titles(got))
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
		t.Errorf("the default should list only unfinished tasks, got %v", titles(open))
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
```

- [ ] **Step 2: Run the test and confirm it fails**

Run: `go test ./internal/store/ -run TestList`
Expected: FAIL, `not implemented yet`

- [ ] **Step 3: Implement**

In `internal/store/sqlite.go`, replace the `List` skeleton with the following and add `"fmt"` and `"strings"` to the imports:

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

	// Undated tasks always sort after dated ones: in SQLite (due IS NULL) is 0 or 1, so ascending works.
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

	// The list is one person's tasks, so the N+1 cost of loading tags row by row is negligible, and simplicity is what it buys.
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

The import block becomes:

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

- [ ] **Step 4: Run the test and confirm it passes**

Run: `go test ./internal/store/ -v`
Expected: everything PASSes except `TestListDoneVisibility`, which needs `SetDone` from Task 7

If `TestListDoneVisibility` fails with `not implemented yet`, skip it for now: `go test ./internal/store/ -run 'TestList' -skip TestListDoneVisibility`, and run the lot once Task 7 lands.

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit --no-gpg-sign -m "feat(store): list queries, filtering and sorting"
```

---

### Task 7: Mutations and metadata queries

**Files:**
- Modify: `internal/store/sqlite.go` (replacing the `Update`/`SetDone`/`Restore`/`Tags`/`Projects` skeletons)
- Test: `internal/store/mutate_test.go`

**Interfaces:**
- Consumes: everything from Task 5 and Task 6
- Produces: a complete, working `Store` implementation

- [ ] **Step 1: Write the failing test**

Create `internal/store/mutate_test.go`:

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
		t.Fatalf("SetDone: %v", err)
	}
	back, _ := s.Get(got.ID)
	if !back.Done() {
		t.Fatal("it should be done")
	}
	if back.DoneAt.Format("2006-01-02") != "2026-08-29" {
		t.Errorf("done_at = %v, want the completion time recorded", back.DoneAt)
	}
	if err := s.SetDone(got.ID, false, ref()); err != nil {
		t.Fatal(err)
	}
	back, _ = s.Get(got.ID)
	if back.Done() {
		t.Error("DoneAt should be nil once the task is reopened")
	}
}

func TestSetDoneMissingID(t *testing.T) {
	s := newStore(t)
	if err := s.SetDone(42, true, ref()); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateOverwritesFieldsAndTags(t *testing.T) {
	s := newStore(t)
	got, _ := s.Add(sample())
	got.Title = "buy soy milk"
	got.Project = ""
	got.Due = nil
	got.Priority = task.PriLow
	got.Tags = []string{"breakfast"}
	if err := s.Update(got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	back, _ := s.Get(got.ID)
	if back.Title != "buy soy milk" || back.Project != "" || back.Due != nil || back.Priority != task.PriLow {
		t.Errorf("the fields were not updated: %+v", back)
	}
	if len(back.Tags) != 1 || back.Tags[0] != "breakfast" {
		t.Errorf("tags = %v, want the whole set replaced by [breakfast]", back.Tags)
	}
}

func TestUpdateMissingID(t *testing.T) {
	s := newStore(t)
	if err := s.Update(task.Task{ID: 7, Title: "x", CreatedAt: ref(), UpdatedAt: ref()}); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDeleteRemovesTagLinks(t *testing.T) {
	s := newStore(t)
	got, _ := s.Add(sample())
	if err := s.Delete(got.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(got.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	// The tag row itself may stay, but nothing should reference it any more.
	tags, err := s.Tags()
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 0 {
		t.Errorf("Tags() = %v, want none: only referenced tags are listed", tags)
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
		t.Fatalf("Restore: %v", err)
	}
	back, err := s.Get(original.ID)
	if err != nil {
		t.Fatalf("the original id should fetch it back after a restore: %v", err)
	}
	if back.Title != original.Title || len(back.Tags) != len(original.Tags) {
		t.Errorf("the restored content does not match: %+v", back)
	}
}

func TestTagsListsOnlyReferenced(t *testing.T) {
	s := newStore(t)
	seed(t, s)
	tags, err := s.Tags()
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 || tags[0] != "misc" || tags[1] != "urgent" {
		t.Errorf("= %v, want [misc urgent] sorted by name", tags)
	}
}

func TestProjectsCountsOpenTasks(t *testing.T) {
	s := newStore(t)
	ids := seed(t, s)
	if err := s.SetDone(ids["work one"], true, ref()); err != nil {
		t.Fatal(err)
	}
	ps, err := s.Projects()
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 2 {
		t.Fatalf("= %+v, want two projects (the empty-string uncategorized one included)", ps)
	}
	if ps[0].Path != "" || ps[0].Open != 3 {
		t.Errorf("ps[0] = %+v, want uncategorized with 3 open", ps[0])
	}
	if ps[1].Path != "/p/work" || ps[1].Open != 1 {
		t.Errorf("ps[1] = %+v, want /p/work with 1 open (the other one is done)", ps[1])
	}
}
```

- [ ] **Step 2: Run the test and confirm it fails**

Run: `go test ./internal/store/ -run 'TestSetDone|TestUpdate|TestRestore|TestTags|TestProjects'`
Expected: FAIL, `not implemented yet`

- [ ] **Step 3: Implement**

In `internal/store/sqlite.go`, replace the five skeleton methods with:

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

// Restore reinserts under the original id. AUTOINCREMENT never reuses numbers, so that id is guaranteed to still be free.
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

// Tags lists only the tags at least one task references; orphans left behind by a delete are neither cleaned up nor shown.
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

- [ ] **Step 4: Run the whole package and confirm it passes**

Run: `go test ./... -v`
Expected: PASS, every case in `internal/store` green (including Task 5's `TestIDsAreNotReused` and Task 6's `TestListDoneVisibility`)

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit --no-gpg-sign -m "feat(store): updates, done toggling, restore and metadata queries"
```

---

### Task 8: The CLI skeleton, dispatch and `todo tui`

**Files:**
- Create: `internal/cli/app.go`
- Test: `internal/cli/app_test.go`
- Test: `internal/cli/helper_test.go`

**Interfaces:**
- Consumes: `store.Store`, `argparse.Result`, `project.Current`
- Produces: `cli.App{Store store.Store; Out, Err io.Writer; Now func() time.Time; Cwd string; Color bool; RunTUI func() error}`, `(*App).Run(args []string) int`, `cli.SplitGlobal(args []string) (dbPath string, rest []string, err error)`, and the internal helpers `(*App).commands()`, `parseIDs([]string) ([]int64, error)`, `(*App).resolveProject(*argparse.Result) (string, bool, error)`

**Choosing the Store for tests**: the spec says "a fake Store", but the implementation uses `store.OpenSQLite(":memory:")` instead. It is shorter, it runs real SQL, it is still completely isolated (nothing touches the filesystem), and it costs less to maintain than a hand-written fake. This is a deliberate narrowing of the spec, not an oversight.

- [ ] **Step 1: Write the failing test**

Create `internal/cli/helper_test.go`:

```go
package cli

import (
	"bytes"
	"testing"
	"time"

	"todo.mirumo.net/internal/store"
)

func refTime() time.Time { return time.Date(2026, 8, 29, 15, 0, 0, 0, time.Local) }

// newApp builds a completely isolated App: an in-memory database, buffered output, a fixed clock and a temporary directory as the cwd.
func newApp(t *testing.T) (*App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	st, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
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

Create `internal/cli/app_test.go`:

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
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Errorf("a bare todo should print the usage, got: %q", out.String())
	}
	if strings.Contains(out.String(), "No matching tasks") {
		t.Error("a bare todo should enter neither the list nor the TUI")
	}
}

func TestRunHelp(t *testing.T) {
	for _, arg := range []string{"-h", "--help", "help"} {
		app, out, _ := newApp(t)
		if code := app.Run([]string{arg}); code != 0 {
			t.Errorf("%s exit code = %d, want 0", arg, code)
		}
		if !strings.Contains(out.String(), "Usage:") {
			t.Errorf("%s printed no usage", arg)
		}
	}
}

func TestHelpFlagWorksAfterSubcommand(t *testing.T) {
	// A subcommand's flag set does not know -h, so it has to be caught before dispatch.
	app, out, errBuf := newApp(t)
	if code := app.Run([]string{"add", "-h"}); code != 0 {
		t.Errorf("exit code = %d, want 0; stderr = %q", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Errorf("todo add -h should print the usage, got: %q", out.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	app, _, errBuf := newApp(t)
	if code := app.Run([]string{"frobnicate"}); code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), `unknown command "frobnicate"`) {
		t.Errorf("stderr = %q", errBuf.String())
	}
}

func TestTUIOnlyOnExplicitSubcommand(t *testing.T) {
	app, _, _ := newApp(t)
	called := false
	app.RunTUI = func() error { called = true; return nil }

	if code := app.Run(nil); code != 0 || called {
		t.Error("a bare todo should not start the TUI")
	}
	if code := app.Run([]string{"tui"}); code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !called {
		t.Error("todo tui should start the TUI")
	}
}

func TestTUIErrorBecomesExitCode1(t *testing.T) {
	app, _, errBuf := newApp(t)
	app.RunTUI = func() error { return errors.New("the terminal is broken") }
	if code := app.Run([]string{"tui"}); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "the terminal is broken") {
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
		{"no --db", []string{"ls", "-a"}, "", []string{"ls", "-a"}},
		{"value after a space", []string{"--db", "/tmp/x.db", "ls"}, "/tmp/x.db", []string{"ls"}},
		{"value after an equals sign", []string{"--db=/tmp/x.db", "ls"}, "/tmp/x.db", []string{"ls"}},
		{"nothing but --db", []string{"--db=/tmp/x.db"}, "/tmp/x.db", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db, rest, err := SplitGlobal(c.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if db != c.wantDB {
				t.Errorf("db = %q, want %q", db, c.wantDB)
			}
			if strings.Join(rest, " ") != strings.Join(c.wantRest, " ") {
				t.Errorf("rest = %v, want %v", rest, c.wantRest)
			}
		})
	}
	if _, _, err := SplitGlobal([]string{"--db"}); err == nil {
		t.Error("--db with no value should be an error")
	}
}

func TestParseIDs(t *testing.T) {
	got, err := parseIDs([]string{"3", "17"})
	if err != nil || len(got) != 2 || got[0] != 3 || got[1] != 17 {
		t.Errorf("= %v, %v", got, err)
	}
	for _, bad := range [][]string{{}, {"x"}, {"0"}, {"-1"}} {
		if _, err := parseIDs(bad); err == nil {
			t.Errorf("parseIDs(%v) should be an error", bad)
		}
	}
}
```

- [ ] **Step 2: Run the test and confirm it fails**

Run: `go test ./internal/cli/`
Expected: FAIL, `undefined: App`

- [ ] **Step 3: Implement**

Create `internal/cli/app.go`:

```go
// Package cli implements todo's command line interface.
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

// App holds everything one run depends on. All of it is injectable, which is what makes tests isolated.
type App struct {
	Store store.Store
	Out   io.Writer
	Err   io.Writer
	Now   func() time.Time
	Cwd   string
	Color bool
	// RunTUI is injected by cmd/todo. cli does not import tui; the two stay parallel.
	RunTUI func() error
}

const usageText = `todo — a local task list

Usage:
  todo <command> [flags]

Commands:
  add <title>     Add a task
  ls              List tasks
  done <id>...    Mark tasks as done
  undone <id>...  Mark tasks as not done
  edit <id>       Change fields
  rm <id>...      Delete tasks
  projects        List projects with their open counts
  tags            List tags
  tui             Open the interactive interface

Global flags:
  --db <path>     Database file (default ~/.todo/todo.db, or $TODO_DB)
  -h, --help      Show this help
`

// commands is the subcommand table. Every new subcommand is one line here.
func (a *App) commands() map[string]func([]string) error {
	return map[string]func([]string) error{
		"tui": a.cmdTUI,
	}
}

// Run executes one command and returns the process exit code: 0 success, 1 a failed run, 2 a usage error.
func (a *App) Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(a.Out, usageText)
		return 0
	}
	name, rest := args[0], args[1:]
	// -h counts wherever it appears. A subcommand's flag set does not know it,
	// so without catching it first todo add -h becomes "unknown flag", which is a poor experience.
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
		fmt.Fprintf(a.Err, "unknown command %q\n\n", name)
		fmt.Fprint(a.Err, usageText)
		return 2
	}
	if err := cmd(rest); err != nil {
		fmt.Fprintf(a.Err, "error: %s\n", err)
		return 1
	}
	return 0
}

func (a *App) cmdTUI(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("todo tui takes no arguments, got %q", args[0])
	}
	if a.RunTUI == nil {
		return errors.New("this build has no TUI")
	}
	return a.RunTUI()
}

// SplitGlobal takes the leading run of --db and returns the rest of the arguments.
// Only the start is scanned: --db is a global flag, and a fixed position is what keeps it from being confused with a subcommand's flags.
func SplitGlobal(args []string) (dbPath string, rest []string, err error) {
	for len(args) > 0 {
		switch a := args[0]; {
		case a == "--db":
			if len(args) < 2 {
				return "", nil, errors.New("flag --db needs a value")
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
		return nil, errors.New("at least one id is needed")
	}
	ids := make([]int64, 0, len(args))
	for _, a := range args {
		n, err := strconv.ParseInt(a, 10, 64)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("not a valid id: %q", a)
		}
		ids = append(ids, n)
	}
	return ids, nil
}

// resolveProject reads the three states of -p.
// The bool it returns means "should the project field change at all"; the string is the new value, which may be empty.
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

- [ ] **Step 4: Run the test and confirm it passes**

Run: `go test ./internal/cli/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/
git commit --no-gpg-sign -m "feat(cli): the App skeleton, command dispatch and todo tui"
```

---

### Task 9: `todo add`

**Files:**
- Create: `internal/cli/cmd_add.go`
- Modify: `internal/cli/app.go` (one more line in `commands()`)
- Test: `internal/cli/cmd_add_test.go`

**Interfaces:**
- Consumes: `(*App).resolveProject`, `argparse`, `task`, `datearg`
- Produces: `(*App).cmdAdd([]string) error`, `addFlags() *argparse.Set` (reused by `edit`)

- [ ] **Step 1: Write the failing test**

Create `internal/cli/cmd_add_test.go`:

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
	if code := app.Run([]string{"add", "  buy milk  "}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out.String(), "added #1: buy milk") {
		t.Errorf("stdout = %q", out.String())
	}
	got, err := app.Store.Get(1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "buy milk" {
		t.Errorf("title = %q, want the surrounding spaces trimmed", got.Title)
	}
	if got.Project != "" {
		t.Errorf("project = %q, without -p it should be globally uncategorized", got.Project)
	}
}

func TestAddAllFlags(t *testing.T) {
	app, _, _ := newApp(t)
	code := app.Run([]string{"add", "buy milk", "-t", "shopping", "--tag=chores", "-d", "tomorrow", "--pri", "high"})
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.Due == nil || got.Due.Format("2006-01-02") != "2026-08-30" {
		t.Errorf("due = %v, want 2026-08-30", got.Due)
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
	if code := app.Run([]string{"add", "fix a bug", "-p"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got, _ := app.Store.Get(1)
	want, _ := filepath.EvalSymlinks(root)
	gotResolved, _ := filepath.EvalSymlinks(got.Project)
	if gotResolved != want {
		t.Errorf("project = %q, want the current repository root %q", gotResolved, want)
	}
}

func TestAddProjectExplicitName(t *testing.T) {
	app, _, _ := newApp(t)
	if code := app.Run([]string{"add", "fix a bug", "-p", "work"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.Project != "work" {
		t.Errorf("project = %q, want work", got.Project)
	}
}

func TestAddMissingTitleExplainsTheFootgun(t *testing.T) {
	app, _, errBuf := newApp(t)
	// -p swallowed the argument that should have been the title.
	if code := app.Run([]string{"add", "-p", "buy milk"}); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	msg := errBuf.String()
	if !strings.Contains(msg, "missing title") || !strings.Contains(msg, "--project=buy milk") {
		t.Errorf("the error should say -p swallowed it and show the fix, got: %q", msg)
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
			t.Errorf("%v should fail", args)
		}
		if errBuf.Len() == 0 {
			t.Errorf("%v should print an error", args)
		}
	}
}
```

- [ ] **Step 2: Run the test and confirm it fails**

Run: `go test ./internal/cli/ -run TestAdd`
Expected: FAIL, `unknown command "add"`

- [ ] **Step 3: Implement**

Create `internal/cli/cmd_add.go`:

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

// addFlags is the field flag set add and edit share.
func addFlags() *argparse.Set {
	return argparse.New(
		argparse.Spec{Long: "project", Short: "p", Kind: argparse.OptionalString, Usage: "Project; with no value, the current directory"},
		argparse.Spec{Long: "tag", Short: "t", Kind: argparse.StringSlice, Usage: "Tag; repeatable"},
		argparse.Spec{Long: "due", Short: "d", Kind: argparse.String, Usage: "Due date: tomorrow, fri, +3d, 2026-09-01"},
		argparse.Spec{Long: "pri", Kind: argparse.String, Usage: "Priority: low, med, high"},
	)
}

func (a *App) cmdAdd(args []string) error {
	r, err := addFlags().Parse(args)
	if err != nil {
		return err
	}
	pos := r.Args()
	if len(pos) == 0 {
		// The most common mistake: todo add -p "buy milk", where -p swallows the title.
		if v, has := r.Optional("project"); has {
			return fmt.Errorf("missing title: %q was taken as the value of --project. Put the title before the flags (todo add %q -p), or write --project=%s", v, v, v)
		}
		return errors.New("usage: todo add <title> [flags]")
	}
	if len(pos) > 1 {
		return fmt.Errorf("only one title is allowed, got %d positional arguments; quote a title that contains spaces", len(pos))
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
	fmt.Fprintf(a.Out, "added #%d: %s\n", got.ID, got.Title)
	return nil
}
```

Add `add` to `commands()` in `internal/cli/app.go`:

```go
	return map[string]func([]string) error{
		"add": a.cmdAdd,
		"tui": a.cmdTUI,
	}
```

- [ ] **Step 4: Run the test and confirm it passes**

Run: `go test ./internal/cli/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/
git commit --no-gpg-sign -m "feat(cli): todo add"
```

---

### Task 10: `todo ls` and the list layout

**Files:**
- Create: `internal/cli/format.go`
- Create: `internal/cli/cmd_ls.go`
- Modify: `internal/cli/app.go` (one more line in `commands()`)
- Test: `internal/cli/format_test.go`
- Test: `internal/cli/cmd_ls_test.go`

**Interfaces:**
- Consumes: `store.Store.List`, `task.Filter`, `datearg.Format`, `project.Label`
- Produces: `cli.WriteList(w io.Writer, ts []task.Task, now time.Time, color bool)`, the internal `pad(string, int) string`, `(*App).cmdLs([]string) error`

- [ ] **Step 1: Add the Lip Gloss dependency**

```bash
go get github.com/charmbracelet/lipgloss@v1
```

Only `lipgloss.Width` is used: a Chinese character takes two cells in a terminal, and both `len()` and `text/tabwriter` get that wrong, which throws the columns off.

- [ ] **Step 2: Write the failing test**

Create `internal/cli/format_test.go`:

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
	// The Chinese title is 3 runes, 9 bytes and 6 display cells wide.
	if got := pad("買牛奶", 8); got != "買牛奶  " {
		t.Errorf("= %q, want it padded to 8 cells (two spaces)", got)
	}
	if got := pad("abc", 2); got != "abc" {
		t.Errorf("= %q, anything wider than the column comes back unchanged", got)
	}
}

func TestWriteListAlignsColumns(t *testing.T) {
	now := time.Date(2026, 8, 29, 15, 0, 0, 0, time.Local)
	ts := []task.Task{
		{ID: 1, Title: "買牛奶", Due: day(2026, 8, 29), Priority: task.PriHigh, Tags: []string{"shopping"}},
		{ID: 12, Title: "繳房租", Project: "/p/home"},
	}
	var buf bytes.Buffer
	WriteList(&buf, ts, now, false)
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want two lines, got %d: %q", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[0], "1  [ ] !High today 買牛奶 ") {
		t.Errorf("line one = %q", lines[0])
	}
	if !strings.Contains(lines[0], "@shopping") {
		t.Errorf("line one is missing the tag: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "12 [ ] ") {
		t.Errorf("line two = %q, the id column should align to the widest id", lines[1])
	}
	if !strings.HasSuffix(lines[1], "home") {
		t.Errorf("line two should end with the project basename: %q", lines[1])
	}
	for _, l := range lines {
		if strings.HasSuffix(l, " ") {
			t.Errorf("there should be no trailing whitespace: %q", l)
		}
		if strings.Contains(l, "\x1b[") {
			t.Errorf("color=false must emit no ANSI codes: %q", l)
		}
	}
}

func TestWriteListEmpty(t *testing.T) {
	var buf bytes.Buffer
	WriteList(&buf, nil, time.Now(), false)
	if !strings.Contains(buf.String(), "No matching tasks") {
		t.Errorf("= %q", buf.String())
	}
}

func TestWriteListColorMarksOverdueAndDone(t *testing.T) {
	now := time.Date(2026, 8, 29, 15, 0, 0, 0, time.Local)
	done := now
	ts := []task.Task{
		{ID: 1, Title: "overdue", Due: day(2026, 8, 1)},
		{ID: 2, Title: "done", DoneAt: &done},
	}
	var buf bytes.Buffer
	WriteList(&buf, ts, now, true)
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if !strings.HasPrefix(lines[0], "\x1b[31m") {
		t.Errorf("an overdue task should be red: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "\x1b[2m") {
		t.Errorf("a finished task should be dim: %q", lines[1])
	}
}
```

Create `internal/cli/cmd_ls_test.go`:

```go
package cli

import (
	"strings"
	"testing"
)

// Insert a few tasks before testing ls.
func seedCLI(t *testing.T, app *App) {
	t.Helper()
	cases := [][]string{
		{"add", "overdue one", "-d", "2026-08-20"},
		{"add", "today one", "-d", "today", "--pri", "high", "-t", "urgent"},
		{"add", "work one", "-p", "work", "-t", "misc"},
		{"add", "undated one"},
	}
	for _, args := range cases {
		if code := app.Run(args); code != 0 {
			t.Fatalf("%v failed", args)
		}
	}
}

func TestLsDefaultsToOpenOnly(t *testing.T) {
	app, out, _ := newApp(t)
	seedCLI(t, app)
	if code := app.Run([]string{"done", "1"}); code != 0 {
		t.Skip("done is not implemented yet; run this again after Task 11")
	}
	out.Reset()
	app.Run([]string{"ls"})
	if strings.Contains(out.String(), "overdue one") {
		t.Errorf("the default should not list done tasks: %q", out.String())
	}
	out.Reset()
	app.Run([]string{"ls", "-a"})
	if !strings.Contains(out.String(), "overdue one") {
		t.Errorf("-a should include done tasks: %q", out.String())
	}
}

func TestLsFilterByProjectAndNoProject(t *testing.T) {
	app, out, _ := newApp(t)
	seedCLI(t, app)

	out.Reset()
	app.Run([]string{"ls", "-p", "work"})
	if !strings.Contains(out.String(), "work one") || strings.Contains(out.String(), "today one") {
		t.Errorf("-p work = %q", out.String())
	}

	out.Reset()
	app.Run([]string{"ls", "--no-project"})
	if strings.Contains(out.String(), "work one") {
		t.Errorf("--no-project should exclude tasks that have one: %q", out.String())
	}
	if !strings.Contains(out.String(), "today one") {
		t.Errorf("--no-project should include uncategorized tasks: %q", out.String())
	}
}

func TestLsRejectsConflictingProjectFlags(t *testing.T) {
	app, _, errBuf := newApp(t)
	if code := app.Run([]string{"ls", "-p", "work", "--no-project"}); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "cannot be used together") {
		t.Errorf("stderr = %q", errBuf.String())
	}
}

func TestLsDueKeywordsAndTags(t *testing.T) {
	app, out, _ := newApp(t)
	seedCLI(t, app)

	out.Reset()
	app.Run([]string{"ls", "-d", "today"})
	if !strings.Contains(out.String(), "today one") || strings.Contains(out.String(), "overdue one") {
		t.Errorf("-d today = %q", out.String())
	}

	out.Reset()
	app.Run([]string{"ls", "-d", "overdue"})
	if !strings.Contains(out.String(), "overdue one") {
		t.Errorf("-d overdue = %q", out.String())
	}

	out.Reset()
	app.Run([]string{"ls", "-t", "urgent"})
	if !strings.Contains(out.String(), "today one") || strings.Contains(out.String(), "work one") {
		t.Errorf("-t urgent = %q", out.String())
	}
}

func TestLsRejectsPositionalArgs(t *testing.T) {
	app, _, errBuf := newApp(t)
	if code := app.Run([]string{"ls", "junk"}); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "takes no positional arguments") {
		t.Errorf("stderr = %q", errBuf.String())
	}
}

func TestLsBadSortAndPriority(t *testing.T) {
	app, _, _ := newApp(t)
	if code := app.Run([]string{"ls", "-s", "title"}); code == 0 {
		t.Error("an unknown sort should fail")
	}
	if code := app.Run([]string{"ls", "--pri", "urgent"}); code == 0 {
		t.Error("an unknown priority should fail")
	}
}
```

- [ ] **Step 3: Run the test and confirm it fails**

Run: `go test ./internal/cli/ -run 'TestPad|TestWriteList|TestLs'`
Expected: FAIL, `undefined: pad`

- [ ] **Step 4: Implement the layout**

Create `internal/cli/format.go`:

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

// pad pads to a terminal display width. A Chinese character takes two cells, which both len() and text/tabwriter get wrong.
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

// WriteList prints an aligned task list. With color false it emits no ANSI codes at all.
func WriteList(w io.Writer, ts []task.Task, now time.Time, color bool) {
	if len(ts) == 0 {
		fmt.Fprintln(w, "No matching tasks")
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

- [ ] **Step 5: Implement ls**

Create `internal/cli/cmd_ls.go`:

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
		argparse.Spec{Long: "project", Short: "p", Kind: argparse.OptionalString, Usage: "Only this project; with no value, the current directory"},
		argparse.Spec{Long: "no-project", Kind: argparse.Bool, Usage: "Only globally uncategorized tasks"},
		argparse.Spec{Long: "tag", Short: "t", Kind: argparse.StringSlice, Usage: "Tag; repeatable, intersected"},
		argparse.Spec{Long: "due", Short: "d", Kind: argparse.String, Usage: "today, week, overdue or a date"},
		argparse.Spec{Long: "pri", Kind: argparse.String, Usage: "Priority: low, med, high"},
		argparse.Spec{Long: "all", Short: "a", Kind: argparse.Bool, Usage: "Include done tasks"},
		argparse.Spec{Long: "done", Kind: argparse.Bool, Usage: "Only done tasks"},
		argparse.Spec{Long: "sort", Short: "s", Kind: argparse.String, Usage: "Sort: due, pri, created"},
	)
	r, err := set.Parse(args)
	if err != nil {
		return err
	}
	if pos := r.Args(); len(pos) > 0 {
		return fmt.Errorf("ls takes no positional arguments, got %q; to search titles use the / key in todo tui", pos[0])
	}

	f := task.Filter{
		IncludeDone: r.Bool("all"),
		OnlyDone:    r.Bool("done"),
		Tags:        r.Strings("tag"),
	}
	if r.Bool("no-project") && r.Changed("project") {
		return errors.New("-p and --no-project cannot be used together")
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

Add `"ls": a.cmdLs,` to `commands()`.

- [ ] **Step 6: Run the test and confirm it passes**

Run: `go test ./internal/cli/ -v`
Expected: PASS (`TestLsDefaultsToOpenOnly` SKIPs, and turns green after Task 11)

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/cli/
git commit --no-gpg-sign -m "feat(cli): todo ls and a list layout with the right widths"
```

---

### Task 11: `done` / `undone` / `rm`

**Files:**
- Create: `internal/cli/cmd_mark.go`
- Modify: `internal/cli/app.go` (three more lines in `commands()`)
- Test: `internal/cli/cmd_mark_test.go`

**Interfaces:**
- Consumes: `parseIDs`, `store.Store.Get/SetDone/Delete`, `store.ErrNotFound`
- Produces: `(*App).cmdDone`, `(*App).cmdUndone`, `(*App).cmdRm`

- [ ] **Step 1: Write the failing test**

Create `internal/cli/cmd_mark_test.go`:

```go
package cli

import (
	"strings"
	"testing"
)

func TestDoneAndUndone(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "buy milk"})
	out.Reset()

	if code := app.Run([]string{"done", "1"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out.String(), "done #1: buy milk") {
		t.Errorf("stdout = %q", out.String())
	}
	got, _ := app.Store.Get(1)
	if !got.Done() {
		t.Error("it should be done")
	}

	out.Reset()
	app.Run([]string{"undone", "1"})
	if !strings.Contains(out.String(), "reopened #1") {
		t.Errorf("stdout = %q", out.String())
	}
	got, _ = app.Store.Get(1)
	if got.Done() {
		t.Error("it should not be done")
	}
}

func TestDoneAcceptsMultipleIDs(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "a"})
	app.Run([]string{"add", "b"})
	out.Reset()
	if code := app.Run([]string{"done", "1", "2"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if strings.Count(out.String(), "done") != 2 {
		t.Errorf("stdout = %q, want two lines", out.String())
	}
}

func TestMarkMissingIDNamesTheID(t *testing.T) {
	app, _, errBuf := newApp(t)
	if code := app.Run([]string{"done", "42"}); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "#42") {
		t.Errorf("the error should name the id at fault: %q", errBuf.String())
	}
}

func TestRm(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "buy milk"})
	out.Reset()
	if code := app.Run([]string{"rm", "1"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out.String(), "deleted #1: buy milk") {
		t.Errorf("stdout = %q", out.String())
	}
	if _, err := app.Store.Get(1); err == nil {
		t.Error("it should have been deleted")
	}
}

func TestMarkRequiresID(t *testing.T) {
	for _, cmd := range []string{"done", "undone", "rm"} {
		app, _, errBuf := newApp(t)
		if code := app.Run([]string{cmd}); code != 1 {
			t.Errorf("%s with no id: exit code = %d, want 1", cmd, code)
		}
		if errBuf.Len() == 0 {
			t.Errorf("%s should print an error", cmd)
		}
	}
}
```

- [ ] **Step 2: Run the test and confirm it fails**

Run: `go test ./internal/cli/ -run 'TestDone|TestRm|TestMark'`
Expected: FAIL, `unknown command "done"`

- [ ] **Step 3: Implement**

Create `internal/cli/cmd_mark.go`:

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
	verb := "done"
	if !done {
		verb = "reopened"
	}
	for _, id := range ids {
		t, err := a.Store.Get(id)
		if err != nil {
			return fmt.Errorf("#%d: %w", id, err)
		}
		if err := a.Store.SetDone(id, done, a.Now()); err != nil {
			return fmt.Errorf("#%d: %w", id, err)
		}
		fmt.Fprintf(a.Out, "%s #%d: %s\n", verb, id, t.Title)
	}
	return nil
}

func (a *App) cmdRm(args []string) error {
	ids, err := parseIDs(args)
	if err != nil {
		return err
	}
	for _, id := range ids {
		// Fetch it first, so the message can carry the title and the user can see they deleted the right thing.
		t, err := a.Store.Get(id)
		if err != nil {
			return fmt.Errorf("#%d: %w", id, err)
		}
		if err := a.Store.Delete(id); err != nil {
			return fmt.Errorf("#%d: %w", id, err)
		}
		fmt.Fprintf(a.Out, "deleted #%d: %s\n", id, t.Title)
	}
	return nil
}
```

Add to `commands()`:

```go
		"done":   a.cmdDone,
		"undone": a.cmdUndone,
		"rm":     a.cmdRm,
```

- [ ] **Step 4: Run the test and confirm it passes**

Run: `go test ./internal/cli/ -v`
Expected: PASS, and `TestLsDefaultsToOpenOnly` no longer SKIPs

- [ ] **Step 5: Commit**

```bash
git add internal/cli/
git commit --no-gpg-sign -m "feat(cli): done, undone, rm"
```

---

### Task 12: `todo edit`

**Files:**
- Create: `internal/cli/cmd_edit.go`
- Modify: `internal/cli/app.go` (one more line in `commands()`)
- Test: `internal/cli/cmd_edit_test.go`

**Interfaces:**
- Consumes: `addFlags()`, `(*App).resolveProject`, `store.Store.Get/Update`
- Produces: `(*App).cmdEdit([]string) error`

**An addition to the spec**: the spec's `edit` lists only flags. The implementation also takes an optional second positional argument as the new title (`todo edit 3 "new title"`), because otherwise the CLI cannot change a title at all and the TUI is the only way — that is a gap, not a design.

- [ ] **Step 1: Write the failing test**

Create `internal/cli/cmd_edit_test.go`:

```go
package cli

import (
	"strings"
	"testing"

	"todo.mirumo.net/internal/task"
)

func TestEditOnlyTouchesGivenFlags(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "buy milk", "-d", "tomorrow", "--pri", "high", "-t", "shopping", "-p", "work"})
	out.Reset()

	if code := app.Run([]string{"edit", "1", "--pri", "low"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.Priority != task.PriLow {
		t.Errorf("priority = %v, want low", got.Priority)
	}
	if got.Due == nil {
		t.Error("without --due the due date must not move")
	}
	if got.Project != "work" {
		t.Errorf("project = %q, without -p it must not move", got.Project)
	}
	if len(got.Tags) != 1 {
		t.Errorf("tags = %v, without -t they must not move", got.Tags)
	}
}

func TestEditEmptyDueClearsIt(t *testing.T) {
	app, _, _ := newApp(t)
	app.Run([]string{"add", "buy milk", "-d", "tomorrow"})
	if code := app.Run([]string{"edit", "1", "--due", ""}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.Due != nil {
		t.Errorf("due = %v, --due \"\" should clear the due date", got.Due)
	}
}

func TestEditEmptyProjectMakesItGlobal(t *testing.T) {
	app, _, _ := newApp(t)
	app.Run([]string{"add", "buy milk", "-p", "work"})
	if code := app.Run([]string{"edit", "1", "--project="}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.Project != "" {
		t.Errorf("project = %q, --project= should put it back to globally uncategorized", got.Project)
	}
}

func TestEditReplacesTagsWholesale(t *testing.T) {
	app, _, _ := newApp(t)
	app.Run([]string{"add", "buy milk", "-t", "shopping", "-t", "chores"})
	app.Run([]string{"edit", "1", "-t", "breakfast"})
	got, _ := app.Store.Get(1)
	if len(got.Tags) != 1 || got.Tags[0] != "breakfast" {
		t.Errorf("tags = %v, -t should replace the whole set rather than add to it", got.Tags)
	}
}

func TestEditTitleViaSecondPositional(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "buy milk"})
	out.Reset()
	if code := app.Run([]string{"edit", "1", "buy soy milk"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got, _ := app.Store.Get(1)
	if got.Title != "buy soy milk" {
		t.Errorf("title = %q", got.Title)
	}
	if !strings.Contains(out.String(), "updated #1: buy soy milk") {
		t.Errorf("stdout = %q", out.String())
	}
}

func TestEditErrors(t *testing.T) {
	app, _, _ := newApp(t)
	app.Run([]string{"add", "buy milk"})
	for _, args := range [][]string{
		{"edit"},
		{"edit", "x"},
		{"edit", "42", "--pri", "low"},
		{"edit", "1", "a", "b"},
		{"edit", "1", "--due", "someday"},
	} {
		if code := app.Run(args); code == 0 {
			t.Errorf("%v should fail", args)
		}
	}
}
```

- [ ] **Step 2: Run the test and confirm it fails**

Run: `go test ./internal/cli/ -run TestEdit`
Expected: FAIL, `unknown command "edit"`

- [ ] **Step 3: Implement**

Create `internal/cli/cmd_edit.go`:

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
		return errors.New("usage: todo edit <id> [new title] [flags]")
	}
	if len(pos) > 2 {
		return fmt.Errorf("at most two positional arguments are accepted, <id> and a new title, got %d", len(pos))
	}
	ids, err := parseIDs(pos[:1])
	if err != nil {
		return err
	}
	t, err := a.Store.Get(ids[0])
	if err != nil {
		return fmt.Errorf("#%d: %w", ids[0], err)
	}

	// Only the fields that were given change. Leaving a flag out and giving it an empty value are two different things.
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
		return fmt.Errorf("#%d: %w", t.ID, err)
	}
	fmt.Fprintf(a.Out, "updated #%d: %s\n", t.ID, t.Title)
	return nil
}
```

Add `"edit": a.cmdEdit,` to `commands()`.

- [ ] **Step 4: Run the test and confirm it passes**

Run: `go test ./internal/cli/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/
git commit --no-gpg-sign -m "feat(cli): todo edit"
```

---

### Task 13: `projects` and `tags`

**Files:**
- Create: `internal/cli/cmd_meta.go`
- Modify: `internal/cli/app.go` (two more lines in `commands()`)
- Test: `internal/cli/cmd_meta_test.go`

**Interfaces:**
- Consumes: `store.Store.Projects/Tags`, `project.Label`, `pad`
- Produces: `(*App).cmdProjects`, `(*App).cmdTags`

- [ ] **Step 1: Write the failing test**

Create `internal/cli/cmd_meta_test.go`:

```go
package cli

import (
	"strings"
	"testing"
)

func TestProjectsListsCountsAndUncategorized(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "a global one"})
	app.Run([]string{"add", "work A", "-p", "/p/work"})
	app.Run([]string{"add", "work B", "-p", "/p/work"})
	app.Run([]string{"done", "3"})
	out.Reset()

	if code := app.Run([]string{"projects"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	s := out.String()
	if !strings.Contains(s, "(uncategorized)") {
		t.Errorf("an empty project should show as (uncategorized): %q", s)
	}
	if !strings.Contains(s, "work") || !strings.Contains(s, "1 open") {
		t.Errorf("the basename and the open count should both show: %q", s)
	}
	if !strings.Contains(s, "/p/work") {
		t.Errorf("a task with a project should carry its full path: %q", s)
	}
}

func TestProjectsEmpty(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"projects"})
	if !strings.Contains(out.String(), "No tasks yet") {
		t.Errorf("= %q", out.String())
	}
}

func TestTags(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"add", "x", "-t", "shopping", "-t", "chores"})
	out.Reset()
	app.Run([]string{"tags"})
	s := out.String()
	if !strings.Contains(s, "@chores") || !strings.Contains(s, "@shopping") {
		t.Errorf("= %q", s)
	}
}

func TestTagsEmpty(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run([]string{"tags"})
	if !strings.Contains(out.String(), "No tags yet") {
		t.Errorf("= %q", out.String())
	}
}

func TestMetaCommandsRejectArgs(t *testing.T) {
	for _, cmd := range []string{"projects", "tags"} {
		app, _, errBuf := newApp(t)
		if code := app.Run([]string{cmd, "junk"}); code != 1 {
			t.Errorf("%s exit code = %d, want 1", cmd, code)
		}
		if !strings.Contains(errBuf.String(), "takes no arguments") {
			t.Errorf("%s stderr = %q", cmd, errBuf.String())
		}
	}
}
```

- [ ] **Step 2: Run the test and confirm it fails**

Run: `go test ./internal/cli/ -run 'TestProjects|TestTags|TestMeta'`
Expected: FAIL, `unknown command "projects"`

- [ ] **Step 3: Implement**

Create `internal/cli/cmd_meta.go`:

```go
package cli

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"todo.mirumo.net/internal/project"
)

func (a *App) cmdProjects(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("projects takes no arguments, got %q", args[0])
	}
	ps, err := a.Store.Projects()
	if err != nil {
		return err
	}
	if len(ps) == 0 {
		fmt.Fprintln(a.Out, "No tasks yet")
		return nil
	}
	labels := make([]string, len(ps))
	var w int
	for i, p := range ps {
		labels[i] = project.Label(p.Path)
		if labels[i] == "" {
			labels[i] = "(uncategorized)"
		}
		w = max(w, lipgloss.Width(labels[i]))
	}
	for i, p := range ps {
		fmt.Fprintf(a.Out, "%s  %d open", pad(labels[i], w), p.Open)
		if p.Path != "" {
			fmt.Fprintf(a.Out, "  %s", p.Path)
		}
		fmt.Fprintln(a.Out)
	}
	return nil
}

func (a *App) cmdTags(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("tags takes no arguments, got %q", args[0])
	}
	tags, err := a.Store.Tags()
	if err != nil {
		return err
	}
	if len(tags) == 0 {
		fmt.Fprintln(a.Out, "No tags yet")
		return nil
	}
	for _, t := range tags {
		fmt.Fprintf(a.Out, "@%s\n", t)
	}
	return nil
}
```

Add to `commands()`:

```go
		"projects": a.cmdProjects,
		"tags":     a.cmdTags,
```

- [ ] **Step 4: Run the test and confirm it passes**

Run: `go test ./internal/cli/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/
git commit --no-gpg-sign -m "feat(cli): projects and tags"
```

---

### Task 14: Wiring the executable

**Files:**
- Create: `cmd/todo/main.go`
- Test: `cmd/todo/main_test.go`

**Interfaces:**
- Consumes: `cli.SplitGlobal`, `cli.App`, `store.OpenSQLite`
- Produces: the `todo` executable, plus the internal `resolveDBPath(envDB, flagDB, home string) string` and `isTTY(*os.File) bool`

- [ ] **Step 1: Write the failing test**

Create `cmd/todo/main_test.go`:

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
		{"default", "", "", def},
		{"environment variable", "/tmp/env.db", "", "/tmp/env.db"},
		{"the flag overrides the environment", "/tmp/env.db", "/tmp/flag.db", "/tmp/flag.db"},
		{"nothing but the flag", "", "/tmp/flag.db", "/tmp/flag.db"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveDBPath(c.env, c.flag, home); got != c.want {
				t.Errorf("= %q, want %q", got, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test and confirm it fails**

Run: `go test ./cmd/todo/`
Expected: FAIL, `undefined: resolveDBPath`

- [ ] **Step 3: Implement**

Create `cmd/todo/main.go`:

```go
// Command todo is a local task list.
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

// run wraps the whole flow so deferred calls get to run (os.Exit skips them).
func run() int {
	dbFlag, args, err := cli.SplitGlobal(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		return 2
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: no home directory: %s\n", err)
		return 1
	}
	dbPath := resolveDBPath(os.Getenv("TODO_DB"), dbFlag, home)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot create the data directory %s: %s\n", filepath.Dir(dbPath), err)
		return 1
	}
	st, err := store.OpenSQLite(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot open the database %s: %s\n", dbPath, err)
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

// resolveDBPath decides where the database lives: --db beats TODO_DB, and ~/.todo/todo.db is the fallback.
func resolveDBPath(envDB, flagDB, home string) string {
	if flagDB != "" {
		return flagDB
	}
	if envDB != "" {
		return envDB
	}
	return filepath.Join(home, ".todo", "todo.db")
}

// isTTY reports whether the output is a terminal; colour is turned off for a file or a pipe.
func isTTY(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
```

`internal/tui` does not exist yet, so start with the smallest version that compiles, `internal/tui/tui.go`:

```go
// Package tui provides todo's interactive interface.
package tui

import (
	"errors"
	"time"

	"todo.mirumo.net/internal/store"
)

// Run starts the interactive interface.
func Run(s store.Store, now func() time.Time, cwd string) error {
	return errors.New("the TUI is not implemented yet")
}
```

- [ ] **Step 4: Run the tests and check it by hand**

Run:
```bash
go test ./... 
go build -o /tmp/todo ./cmd/todo
TODO_DB=/tmp/todo-smoke.db /tmp/todo add "buy milk" --pri high -d tomorrow -t shopping
TODO_DB=/tmp/todo-smoke.db /tmp/todo ls
TODO_DB=/tmp/todo-smoke.db /tmp/todo done 1
TODO_DB=/tmp/todo-smoke.db /tmp/todo ls -a
TODO_DB=/tmp/todo-smoke.db /tmp/todo ls | cat
rm -f /tmp/todo-smoke.db
```
Expected: the tests PASS; `add` prints `added #1: buy milk`; `ls` shows one aligned line carrying `tomorrow`, `!High` and `@shopping`; after `done`, `ls` is empty and `ls -a` shows `[x]`; the `| cat` run contains no ANSI escapes at all.

- [ ] **Step 5: Commit**

```bash
git add cmd/ internal/tui/
git commit --no-gpg-sign -m "feat(cmd): wire up the todo executable and the ~/.todo data path"
```

---

### Task 15: The TUI list, navigation and toggling done

**Files:**
- Create: `internal/tui/cmds.go`
- Create: `internal/tui/list.go`
- Modify: `internal/tui/tui.go` (replacing Task 14's minimal stub)
- Test: `internal/tui/tui_test.go`

**Interfaces:**
- Consumes: `store.Store`, `task.Filter`, `datearg.Format`, `project.Label`
- Produces: `tui.Model`, `tui.New(s store.Store, now func() time.Time, cwd string) Model`, `(Model).Init/Update/View`, `tui.Run`, the internal msg types `tasksMsg`, `errMsg`, `savedMsg`, and the internal `(Model).loadCmd()`, `(Model).current() (task.Task, bool)`, and the `mode` constant `modeList`

- [ ] **Step 1: Add the Bubble Tea dependency**

```bash
go get github.com/charmbracelet/bubbletea@v1
```

- [ ] **Step 2: Write the failing test**

Create `internal/tui/tui_test.go`:

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

// newModel builds a Model backed by an in-memory database with its tasks already loaded.
func newModel(t *testing.T) (Model, store.Store) {
	t.Helper()
	s, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	for _, ti := range []task.Task{
		{Title: "first", Due: day(2026, 8, 29), Priority: task.PriHigh, Tags: []string{"urgent"}},
		{Title: "second", Project: "/p/work"},
		{Title: "third"},
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

// key turns a key name into a tea.KeyMsg.
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

// send feeds one msg and returns the new model plus the msg its cmd produced, or nil when there is no cmd.
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

// press sends one key and feeds back whatever its cmd produced, mimicking one turn of the Bubble Tea loop.
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
		t.Fatalf("loaded %d tasks, want 3", len(m.tasks))
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
}

func TestNavigation(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "j")
	if m.cursor != 1 {
		t.Errorf("cursor after j = %d, want 1", m.cursor)
	}
	m = press(t, m, "down")
	m = press(t, m, "down")
	if m.cursor != 2 {
		t.Errorf("cursor at the bottom = %d, want it to stop at 2 rather than run past", m.cursor)
	}
	m = press(t, m, "k")
	if m.cursor != 1 {
		t.Errorf("cursor after k = %d, want 1", m.cursor)
	}
	m = press(t, m, "g")
	if m.cursor != 0 {
		t.Errorf("cursor after g = %d, want 0", m.cursor)
	}
	m = press(t, m, "G")
	if m.cursor != 2 {
		t.Errorf("cursor after G = %d, want 2", m.cursor)
	}
	m = press(t, m, "k")
	m = press(t, m, "k")
	m = press(t, m, "k")
	if m.cursor != 0 {
		t.Errorf("cursor at the top = %d, want it to stop at 0 rather than run past", m.cursor)
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
		t.Error("space should mark the task done")
	}
	// The default shows only unfinished tasks, so it should be gone after the reload.
	if len(m.tasks) != 2 {
		t.Errorf("%d tasks after the reload, want 2", len(m.tasks))
	}
}

func TestQuit(t *testing.T) {
	m, _ := newModel(t)
	_, cmd := m.Update(key("q"))
	if cmd == nil {
		t.Fatal("q should return a cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q should quit")
	}
}

func TestErrMsgShowsWithoutCrashing(t *testing.T) {
	m, _ := newModel(t)
	m, _ = send(t, m, errMsg{err: errFake})
	if m.err == nil {
		t.Fatal("the error should have been recorded")
	}
	if !strings.Contains(m.View(), "broken") {
		t.Errorf("the error should be shown on screen: %q", m.View())
	}
}

func TestViewShowsTasksAndCursor(t *testing.T) {
	m, _ := newModel(t)
	v := m.View()
	for _, want := range []string{"first", "second", "third", "today", "!High", "@urgent", "work"} {
		if !strings.Contains(v, want) {
			t.Errorf("the screen is missing %q:\n%s", want, v)
		}
	}
	if !strings.Contains(v, "▸") {
		t.Errorf("the screen should carry a cursor marker:\n%s", v)
	}
}
```

Add at the end of the test file:

```go
var errFake = fakeErr{}

type fakeErr struct{}

func (fakeErr) Error() string { return "the database is broken" }
```

- [ ] **Step 3: Run the test and confirm it fails**

Run: `go test ./internal/tui/`
Expected: FAIL, `undefined: New`

- [ ] **Step 4: Implement the msgs and cmds**

Create `internal/tui/cmds.go`:

```go
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"todo.mirumo.net/internal/task"
)

// Everything that talks to the database is wrapped in a tea.Cmd whose result comes back to Update as a msg.
// Update itself does no IO and stays a pure function, so tests only have to feed it msgs.
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
		note := "completed \"" + t.Title + "\""
		if t.Done() {
			note = "reopened \"" + t.Title + "\""
		}
		return savedMsg{note: note}
	}
}
```

- [ ] **Step 5: Implement the Model**

Rewrite `internal/tui/tui.go`:

```go
// Package tui provides todo's interactive interface.
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

// Model is the root model. Every substate hangs off it and Update dispatches on mode.
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

// New builds a Model.
func New(s store.Store, now func() time.Time, cwd string) Model {
	return Model{store: s, now: now, cwd: cwd, mode: modeList, width: 80, height: 24}
}

// Run starts the interactive interface.
func Run(s store.Store, now func() time.Time, cwd string) error {
	_, err := tea.NewProgram(New(s, now, cwd), tea.WithAltScreen()).Run()
	return err
}

func (m Model) Init() tea.Cmd { return m.loadCmd() }

// current returns the item under the cursor.
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

- [ ] **Step 6: Implement the view**

Create `internal/tui/list.go`:

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

// taskLine builds one row's text, without the cursor marker.
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
		b.WriteString(styleDim.Render("No matching tasks") + "\n")
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
	return fmt.Sprintf("todo — %d tasks", len(m.tasks))
}

func (m Model) footer() string {
	if m.err != nil {
		return styleErr.Render("error: " + m.err.Error())
	}
	if m.status != "" {
		return m.status
	}
	return styleHint.Render("j/k move · space toggle · q quit")
}
```

- [ ] **Step 7: Run the test and confirm it passes**

Run: `go test ./internal/tui/ -v`
Expected: PASS

- [ ] **Step 8: Check it by hand**

```bash
go build -o /tmp/todo ./cmd/todo
TODO_DB=/tmp/todo-tui.db /tmp/todo add "first" --pri high -d today
TODO_DB=/tmp/todo-tui.db /tmp/todo add "second"
TODO_DB=/tmp/todo-tui.db /tmp/todo tui
```
Expected: a full-screen list opens, `j`/`k` move, `space` ticks a task off and it leaves the list, `q` quits.

- [ ] **Step 9: Commit**

```bash
git add go.mod go.sum internal/tui/
git commit --no-gpg-sign -m "feat(tui): the list, navigation and toggling done"
```

---

### Task 16: Delete and undo, search, sorting, showing done tasks

**Files:**
- Modify: `internal/tui/tui.go` (adding `modeSearch`, Model fields and keys)
- Modify: `internal/tui/cmds.go` (adding `deletedMsg`, `deleteCmd`, `restoreCmd`)
- Modify: `internal/tui/list.go` (the footer shows the hint)
- Test: `internal/tui/filter_test.go`

**Interfaces:**
- Consumes: `store.Store.Delete/Get/Restore`, `task.SortBy`
- Produces: `deletedMsg{t task.Task}`, `(Model).deleteCmd`, `(Model).restoreCmd`, and the new Model fields `undo *task.Task`, `search textinput.Model`, `modeSearch`

- [ ] **Step 1: Add the Bubbles dependency**

```bash
go get github.com/charmbracelet/bubbles@v0
```

- [ ] **Step 2: Write the failing test**

Create `internal/tui/filter_test.go`:

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
		t.Fatalf("d should delete the task, err = %v", err)
	}
	if len(m.tasks) != 2 {
		t.Errorf("%d tasks after the delete, want 2", len(m.tasks))
	}
	if !strings.Contains(m.View(), "u to undo") {
		t.Errorf("the bottom should offer the undo:\n%s", m.View())
	}

	m = press(t, m, "u")
	back, err := s.Get(victim.ID)
	if err != nil {
		t.Fatalf("u should restore under the original id, err = %v", err)
	}
	if back.Title != victim.Title || len(back.Tags) != len(victim.Tags) {
		t.Errorf("the restored content does not match: %+v", back)
	}
	if len(m.tasks) != 3 {
		t.Errorf("%d tasks after the undo, want 3", len(m.tasks))
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
		t.Errorf("the last delete should be the one undone: %v", err)
	}
	if _, err := s.Get(first.ID); !errors.Is(err, store.ErrNotFound) {
		t.Error("undo keeps one level; anything deleted earlier must not come back")
	}
	m = press(t, m, "u")
	if !strings.Contains(m.View(), "nothing to undo") {
		t.Errorf("with nothing to undo it should say so:\n%s", m.View())
	}
}

func TestSearchFiltersIncrementally(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "/")
	if m.mode != modeSearch {
		t.Fatal("/ should enter search mode")
	}
	m = press(t, m, "second")
	if len(m.tasks) != 1 || m.tasks[0].Title != "second" {
		t.Errorf("typing should filter as it goes, got %d tasks", len(m.tasks))
	}
	m = press(t, m, "enter")
	if m.mode != modeList {
		t.Error("enter should return to the list and keep the filter")
	}
	if len(m.tasks) != 1 {
		t.Errorf("the filter should survive enter, got %d tasks", len(m.tasks))
	}
}

func TestSearchEscCancels(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "/")
	m = press(t, m, "second")
	m = press(t, m, "esc")
	if m.mode != modeList {
		t.Error("esc should return to the list")
	}
	if len(m.tasks) != 3 {
		t.Errorf("esc should drop the filter, got %d tasks", len(m.tasks))
	}
}

func TestToggleIncludeDone(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, " ") // complete the first one
	if len(m.tasks) != 2 {
		t.Fatalf("want 2 tasks left, got %d", len(m.tasks))
	}
	m = press(t, m, "A")
	if len(m.tasks) != 3 {
		t.Errorf("A should show the done ones too, got %d tasks", len(m.tasks))
	}
	m = press(t, m, "A")
	if len(m.tasks) != 2 {
		t.Errorf("A again should go back to unfinished only, got %d tasks", len(m.tasks))
	}
}

func TestSortCycles(t *testing.T) {
	m, _ := newModel(t)
	if m.filter.Sort != task.SortDue {
		t.Fatal("the default should be due")
	}
	m = press(t, m, "s")
	if m.filter.Sort != task.SortPriority {
		t.Errorf("after s = %v, want pri", m.filter.Sort)
	}
	m = press(t, m, "s")
	if m.filter.Sort != task.SortCreated {
		t.Errorf("s again = %v, want created", m.filter.Sort)
	}
	m = press(t, m, "s")
	if m.filter.Sort != task.SortDue {
		t.Errorf("back around = %v, want due", m.filter.Sort)
	}
}

func TestEscClearsAllFilters(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "/")
	m = press(t, m, "second")
	m = press(t, m, "enter")
	m = press(t, m, "A")
	m = press(t, m, "esc")
	if len(m.tasks) != 3 {
		t.Errorf("esc should clear every filter, got %d tasks", len(m.tasks))
	}
	if m.filter.Search != "" || m.filter.IncludeDone {
		t.Errorf("the filter should be back to zero: %+v", m.filter)
	}
}
```

- [ ] **Step 3: Run the test and confirm it fails**

Run: `go test ./internal/tui/ -run 'TestDelete|TestUndo|TestSearch|TestToggle|TestSort|TestEsc'`
Expected: FAIL, `undefined: modeSearch`

- [ ] **Step 4: Implement the cmds**

Add `deletedMsg` to the msg group in `internal/tui/cmds.go` and append two cmds:

```go
type (
	tasksMsg   []task.Task
	errMsg     struct{ err error }
	savedMsg   struct{ note string }
	deletedMsg struct{ t task.Task }
)

// deleteCmd fetches the whole task before deleting it — undo needs everything, tags included.
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
		return savedMsg{note: "restored \"" + t.Title + "\""}
	}
}
```

- [ ] **Step 5: Implement the Model changes**

The mode constants and the Model in `internal/tui/tui.go` become:

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
	// undo keeps a single level: the last deleted item, discarded when the TUI exits.
	undo *task.Task

	status        string
	err           error
	width, height int
}
```

`New` becomes:

```go
func New(s store.Store, now func() time.Time, cwd string) Model {
	ti := textinput.New()
	ti.Prompt = "/"
	ti.Placeholder = "search titles"
	return Model{
		store: s, now: now, cwd: cwd,
		mode: modeList, search: ti,
		width: 80, height: 24,
	}
}
```

Add `"github.com/charmbracelet/bubbles/textinput"` to the imports.

The `deletedMsg` branch of `Update` and the mode dispatch:

```go
	case deletedMsg:
		t := msg.t
		m.undo = &t
		m.status = "deleted \"" + t.Title + "\" · u to undo"
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

New keys in `updateList`:

```go
	case "d":
		if t, ok := m.current(); ok {
			return m, m.deleteCmd(t)
		}
	case "u":
		if m.undo == nil {
			m.status = "nothing to undo"
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
		m.status = "sort: " + sortLabel(m.filter.Sort)
		return m, m.loadCmd()
	case "esc":
		m.filter = task.Filter{}
		m.search.SetValue("")
		m.status = ""
		return m, m.loadCmd()
```

The search mode and the label helper:

```go
// updateSearch drives incremental search, requerying on every keystroke. The list is small,
// so the cost is negligible and the view never drifts from the database.
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
	// The cmd textinput returns is dropped on purpose: it is the cursor blink timer.
	// Forwarding it would make Update tests wait on a timer, and blinking is decoration.
	m.search, _ = m.search.Update(msg)
	m.filter.Search = m.search.Value()
	m.cursor = 0
	return m, m.loadCmd()
}

func sortLabel(s task.SortBy) string {
	switch s {
	case task.SortPriority:
		return "priority"
	case task.SortCreated:
		return "created"
	}
	return "due date"
}
```

- [ ] **Step 6: The footer shows the search line**

`footer` in `internal/tui/list.go` becomes:

```go
func (m Model) footer() string {
	if m.mode == modeSearch {
		return m.search.View()
	}
	if m.err != nil {
		return styleErr.Render("error: " + m.err.Error())
	}
	if m.status != "" {
		return m.status
	}
	return styleHint.Render("j/k move · space toggle · d delete · / search · s sort · A include done · esc clear · q quit")
}
```

`header` gains the current filter state:

```go
func (m Model) header() string {
	h := fmt.Sprintf("todo — %d tasks", len(m.tasks))
	if m.filter.Search != "" {
		h += "  search: " + m.filter.Search
	}
	if m.filter.IncludeDone {
		h += "  including done"
	}
	return h
}
```

- [ ] **Step 7: Run the test and confirm it passes**

Run: `go test ./internal/tui/ -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add go.mod go.sum internal/tui/
git commit --no-gpg-sign -m "feat(tui): delete and undo, incremental search, sorting, showing done tasks"
```

---

### Task 17: The project and tag menus

**Files:**
- Modify: `internal/tui/tui.go` (adding `modePicker` and its keys)
- Modify: `internal/tui/cmds.go` (adding `projectsMsg`, `tagsMsg` and their cmds)
- Create: `internal/tui/picker.go`
- Test: `internal/tui/picker_test.go`

**Interfaces:**
- Consumes: `store.Store.Projects/Tags`, `project.Label`
- Produces: `pickerKind` (`pickProject`/`pickTag`), `pickerItem{label, value string; clear bool}`, `pickerState{kind pickerKind; items []pickerItem; cursor int}`, `(Model).projectsCmd()`, `(Model).tagsCmd()`, `modePicker`

- [ ] **Step 1: Write the failing test**

Create `internal/tui/picker_test.go`:

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
		t.Fatal("P should open the menu")
	}
	v := m.View()
	for _, want := range []string{"All", "(uncategorized)", "work"} {
		if !strings.Contains(v, want) {
			t.Errorf("the menu is missing %q:\n%s", want, v)
		}
	}
	// Row one is "All", row two is "(uncategorized)", row three is work.
	m = press(t, m, "j")
	m = press(t, m, "j")
	m = press(t, m, "enter")
	if m.mode != modeList {
		t.Fatal("enter should return to the list")
	}
	if len(m.tasks) != 1 || m.tasks[0].Title != "second" {
		t.Errorf("picking work should leave only second, got %d", len(m.tasks))
	}
}

func TestProjectPickerUncategorized(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "P")
	m = press(t, m, "j")
	m = press(t, m, "enter")
	if len(m.tasks) != 2 {
		t.Errorf("(uncategorized) should leave 2, got %d", len(m.tasks))
	}
	if m.filter.Project == nil || *m.filter.Project != "" {
		t.Errorf("filter.Project = %v, want a pointer to an empty string", m.filter.Project)
	}
}

func TestProjectPickerAllClearsFilter(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "P")
	m = press(t, m, "j")
	m = press(t, m, "enter")
	m = press(t, m, "P")
	m = press(t, m, "enter") // row one, "All"
	if m.filter.Project != nil {
		t.Errorf("picking All should clear the project filter, got %v", m.filter.Project)
	}
	if len(m.tasks) != 3 {
		t.Errorf("got %d tasks, want 3", len(m.tasks))
	}
}

func TestTagPicker(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "T")
	if m.mode != modePicker {
		t.Fatal("T should open the menu")
	}
	if !strings.Contains(m.View(), "@urgent") {
		t.Errorf("the tag menu should list @urgent:\n%s", m.View())
	}
	m = press(t, m, "j")
	m = press(t, m, "enter")
	if len(m.tasks) != 1 || m.tasks[0].Title != "first" {
		t.Errorf("picking @urgent should leave only first, got %d", len(m.tasks))
	}
}

func TestPickerEscCancels(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "P")
	m = press(t, m, "esc")
	if m.mode != modeList {
		t.Error("esc should close the menu")
	}
	if m.filter.Project != nil {
		t.Error("esc should apply no filter at all")
	}
}
```

- [ ] **Step 2: Run the test and confirm it fails**

Run: `go test ./internal/tui/ -run 'TestProjectPicker|TestTagPicker|TestPickerEsc'`
Expected: FAIL, `undefined: modePicker`

- [ ] **Step 3: Implement the cmds**

Append to `internal/tui/cmds.go`:

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

Add `"todo.mirumo.net/internal/store"` to the imports.

- [ ] **Step 4: Implement the menu**

Create `internal/tui/picker.go`:

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

// pickerItem is one row of the menu. The entry with clear set means no filtering.
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
	items := []pickerItem{{label: "All", clear: true}}
	for _, p := range ps {
		label := project.Label(p.Path)
		if label == "" {
			label = "(uncategorized)"
		}
		items = append(items, pickerItem{label: label, value: p.Path})
	}
	return items
}

func tagItems(tags []string) []pickerItem {
	items := []pickerItem{{label: "All", clear: true}}
	for _, t := range tags {
		items = append(items, pickerItem{label: "@" + t, value: t})
	}
	return items
}

// updatePicker handles keys in menu mode.
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
	title := "Filter by project"
	if m.picker.kind == pickTag {
		title = "Filter by tag"
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
	b.WriteString("\n" + styleHint.Render("j/k move · enter select · esc cancel"))
	return b.String()
}
```

- [ ] **Step 5: Wire it into the Model**

`internal/tui/tui.go`: add `modePicker` to the modes, `picker pickerState` to the Model, two msg branches and the mode dispatch to `Update`, two keys to `updateList`, and the dispatch in `View`.

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

Add to `updateList`:

```go
	case "P":
		return m, m.projectsCmd()
	case "T":
		return m, m.tagsCmd()
```

`View` becomes:

```go
func (m Model) View() string {
	if m.mode == modePicker {
		return m.viewPicker()
	}
	return m.viewList()
}
```

Add `P/T filter` to the footer hint.

- [ ] **Step 6: Run the test and confirm it passes**

Run: `go test ./internal/tui/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/tui/
git commit --no-gpg-sign -m "feat(tui): the project and tag filter menus"
```

---

### Task 18: The add/edit form and the help overlay

**Files:**
- Create: `internal/tui/form.go`
- Modify: `internal/tui/tui.go` (adding `modeForm`, `modeHelp` and their keys)
- Modify: `internal/tui/cmds.go` (adding `saveCmd`)
- Test: `internal/tui/form_test.go`

**Interfaces:**
- Consumes: `textinput.Model`, `task.ValidateTitle`, `task.ParsePriority`, `task.NormalizeTags`, `datearg.Parse`, `project.Current`, `store.Store.Add/Update`
- Produces: `formState`, `(Model).openForm(t task.Task, editing bool) Model`, `(Model).updateForm`, `(Model).viewForm`, `(Model).saveCmd(t task.Task, editing bool) tea.Cmd`, `modeForm`, `modeHelp`

- [ ] **Step 1: Write the failing test**

Create `internal/tui/form_test.go`:

```go
package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"todo.mirumo.net/internal/task"
)

// typeInto types a string into the focused field.
func typeInto(t *testing.T, m Model, s string) Model {
	t.Helper()
	return press(t, m, s)
}

func TestFormAddCreatesTask(t *testing.T) {
	m, s := newModel(t)
	m = press(t, m, "a")
	if m.mode != modeForm {
		t.Fatal("a should open the form")
	}
	m = typeInto(t, m, "fourth")
	m = press(t, m, "tab")
	m = press(t, m, "tab")
	m = typeInto(t, m, "urgent,misc")
	m = press(t, m, "tab")
	m = typeInto(t, m, "tomorrow")
	m = press(t, m, "tab")
	m = typeInto(t, m, "high")
	m = press(t, m, "enter")

	if m.mode != modeList {
		t.Fatalf("saving should return to the list, mode = %v", m.mode)
	}
	ts, err := s.List(task.Filter{Search: "fourth"}, refTime())
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) != 1 {
		t.Fatalf("one task should have been added, got %d", len(ts))
	}
	got := ts[0]
	if got.Due == nil || got.Due.Format("2006-01-02") != "2026-08-30" {
		t.Errorf("due = %v", got.Due)
	}
	if got.Priority != task.PriHigh {
		t.Errorf("priority = %v", got.Priority)
	}
	if len(got.Tags) != 2 {
		t.Errorf("tags = %v, the comma-separated value should split into two", got.Tags)
	}
}

func TestFormEditPrefillsAndUpdates(t *testing.T) {
	m, s := newModel(t)
	id := m.tasks[0].ID
	m = press(t, m, "e")
	if m.mode != modeForm {
		t.Fatal("e should open the form")
	}
	if !strings.Contains(m.View(), "first") {
		t.Errorf("editing should prefill the current values:\n%s", m.View())
	}
	// Clear the title and type a new one.
	for range len([]rune("first")) {
		m = press(t, m, "backspace")
	}
	m = typeInto(t, m, "changed")
	m = press(t, m, "enter")

	got, err := s.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "changed" {
		t.Errorf("title = %q", got.Title)
	}
	if got.Due == nil {
		t.Error("fields left alone should keep their value")
	}
}

func TestFormRejectsEmptyTitle(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "a")
	m = press(t, m, "enter")
	if m.mode != modeForm {
		t.Error("an empty title should not leave the form")
	}
	if !strings.Contains(m.View(), "a title cannot be empty") {
		t.Errorf("it should say why it cannot save:\n%s", m.View())
	}
}

func TestFormRejectsBadDue(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "a")
	m = typeInto(t, m, "test")
	m = press(t, m, "tab")
	m = press(t, m, "tab")
	m = press(t, m, "tab")
	m = typeInto(t, m, "someday")
	m = press(t, m, "enter")
	if m.mode != modeForm {
		t.Error("an invalid date should not leave the form")
	}
	if !strings.Contains(m.View(), "unrecognised date") {
		t.Errorf("it should point at the date:\n%s", m.View())
	}
}

func TestFormEscCancels(t *testing.T) {
	m, s := newModel(t)
	before, _ := s.List(task.Filter{}, refTime())
	m = press(t, m, "a")
	m = typeInto(t, m, "do not save")
	m = press(t, m, "esc")
	if m.mode != modeList {
		t.Error("esc should return to the list")
	}
	after, _ := s.List(task.Filter{}, refTime())
	if len(after) != len(before) {
		t.Error("esc must not save anything")
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
		t.Errorf("ctrl+p should fill in the current directory's project:\n%s", m.View())
	}
}

func TestHelpOverlay(t *testing.T) {
	m, _ := newModel(t)
	m = press(t, m, "?")
	if m.mode != modeHelp {
		t.Fatal("? should open the help")
	}
	v := m.View()
	for _, want := range []string{"space", "d", "u", "/", "P", "T"} {
		if !strings.Contains(v, want) {
			t.Errorf("the help is missing %q:\n%s", want, v)
		}
	}
	m = press(t, m, "esc")
	if m.mode != modeList {
		t.Error("esc should close the help")
	}
}
```

- [ ] **Step 2: Run the test and confirm it fails**

Run: `go test ./internal/tui/ -run 'TestForm|TestHelp'`
Expected: FAIL, `undefined: modeForm`

- [ ] **Step 3: Implement the form**

Create `internal/tui/form.go`:

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

var fieldLabels = [fieldCount]string{"Title", "Project", "Tags", "Due", "Priority"}

// formState is the form shared by add and edit. editing false means add.
type formState struct {
	editing  bool
	original task.Task
	inputs   [fieldCount]textinput.Model
	focus    int
	errText  string
}

// openForm prepares a form, prefilling current values when editing.
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
		"what needs doing",
		"empty = uncategorized (ctrl+p fills the current directory)",
		"comma separated",
		"tomorrow, fri, +3d, 2026-09-01",
		"low, med, high",
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
	// As in updateSearch, the cursor blink timer cmd is not forwarded.
	m.form.inputs[m.form.focus], _ = m.form.inputs[m.form.focus].Update(msg)
	m.form.errText = ""
	return m, nil
}

// formTask assembles a Task from the form, returning an error if any field is invalid.
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
	title := "New task"
	if m.form.editing {
		title = "Edit #" + itoa(m.form.original.ID)
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
	b.WriteString(styleHint.Render("tab next field · ctrl+p fill current directory · enter save · esc cancel"))
	return b.String()
}
```

`itoa` lives in `internal/tui/list.go`:

```go
func itoa(n int64) string { return strconv.FormatInt(n, 10) }
```

(Add `"strconv"` to the imports.)

- [ ] **Step 4: Implement the save cmd**

Append to `internal/tui/cmds.go`:

```go
func (m Model) saveCmd(t task.Task, editing bool) tea.Cmd {
	s := m.store
	return func() tea.Msg {
		if editing {
			if err := s.Update(t); err != nil {
				return errMsg{err}
			}
			return savedMsg{note: "updated \"" + t.Title + "\""}
		}
		if _, err := s.Add(t); err != nil {
			return errMsg{err}
		}
		return savedMsg{note: "added \"" + t.Title + "\""}
	}
}
```

- [ ] **Step 5: Wire in the Model and the help overlay**

`internal/tui/tui.go`:

```go
const (
	modeList mode = iota
	modeSearch
	modePicker
	modeForm
	modeHelp
)
```

Add `form formState` to the Model.

The mode dispatch in `Update` gains:

```go
		case modeForm:
			return m.updateForm(msg)
		case modeHelp:
			m.mode = modeList
			return m, nil
```

`updateList` gains:

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

`View`:

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

Add the help screen to `internal/tui/list.go`:

```go
func viewHelp() string {
	rows := [][2]string{
		{"j / k / ↑ / ↓", "Move"},
		{"g / G", "Jump to top / bottom"},
		{"space", "Toggle done"},
		{"a / e", "Add / edit"},
		{"d", "Delete"},
		{"u", "Undo the last delete"},
		{"/", "Search titles"},
		{"P / T", "Filter by project / tag"},
		{"A", "Show or hide done tasks"},
		{"s", "Cycle sort order"},
		{"esc", "Clear every filter"},
		{"?", "This help"},
		{"q", "Quit"},
	}
	var b strings.Builder
	b.WriteString("Keys\n\n")
	for _, r := range rows {
		b.WriteString("  " + pad(r[0], 16) + r[1] + "\n")
	}
	b.WriteString("\n" + styleHint.Render("Press any key to go back"))
	return b.String()
}
```

`case "a"` in `updateList` needs `"todo.mirumo.net/internal/task"` imported (`tui.go` already has it).

The footer hint becomes:

```go
	return styleHint.Render("a add · e edit · space toggle · d delete · / search · P/T filter · ? help · q quit")
```

- [ ] **Step 6: Run the whole test suite**

Run: `go test ./... -v`
Expected: everything PASSes

- [ ] **Step 7: Check it by hand**

```bash
go build -o /tmp/todo ./cmd/todo
rm -f /tmp/todo-final.db
export TODO_DB=/tmp/todo-final.db
/tmp/todo add "buy milk" -t shopping -d tomorrow --pri high
/tmp/todo add "fix a bug" -p
/tmp/todo ls
/tmp/todo projects
/tmp/todo tui
```
Expected: in the TUI, check in order that `a` adds, `e` edits (including `ctrl+p` filling the directory), `space` ticks off, `d` then `u` deletes and restores, `/` searches, `P`/`T` filter, `A` shows done tasks, `s` changes the sort, `?` shows the help and `q` quits — and that `/tmp/todo ls` reflects every change afterwards.

- [ ] **Step 8: Commit**

```bash
git add internal/tui/
git commit --no-gpg-sign -m "feat(tui): the add/edit form and the help overlay"
```

---

## Closing checks

- [ ] `go vet ./...` prints nothing
- [ ] `gofmt -l .` prints nothing
- [ ] `go test ./...` is green
- [ ] `grep -rn "flag\." --include=*.go . | grep -v argparse` finds no use of the stdlib flag
- [ ] `go list -m all | grep -E 'cobra|pflag'` finds nothing
- [ ] `ls ~/.todo` still does not exist after the tests have run (unless a manual check created it)
