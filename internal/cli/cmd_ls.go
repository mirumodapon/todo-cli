package cli

import (
	"errors"
	"fmt"
	"strings"

	"todo.mirumo.net/internal/argparse"
	"todo.mirumo.net/internal/datearg"
	"todo.mirumo.net/internal/task"
)

func lsFlags() *argparse.Set {
	return argparse.New(
		argparse.Spec{Long: "project", Short: "p", Kind: argparse.OptionalString, Usage: "Only this project; uses the current directory when given no value"},
		argparse.Spec{Long: "no-project", Kind: argparse.Bool, Usage: "Only uncategorised tasks"},
		argparse.Spec{Long: "tag", Short: "t", Kind: argparse.StringSlice, Usage: "Tag; repeatable, matches tasks having all of them"},
		argparse.Spec{Long: "due", Short: "d", Kind: argparse.String, Usage: "today, week, overdue, or a date"},
		argparse.Spec{Long: "pri", Kind: argparse.String, Usage: "Priority: low, med, high"},
		argparse.Spec{Long: "all", Short: "a", Kind: argparse.Bool, Usage: "Include done tasks"},
		argparse.Spec{Long: "done", Kind: argparse.Bool, Usage: "Only done tasks"},
		argparse.Spec{Long: "sort", Short: "s", Kind: argparse.String, Usage: "Sort by: due, pri, created"},
	)
}

func (a *App) cmdLs(args []string) error {
	set := lsFlags()
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
