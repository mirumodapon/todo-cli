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
		argparse.Spec{Long: "no-project", Kind: argparse.Bool, Usage: "Only uncategorized tasks (the default)"},
		argparse.Spec{Long: "all-projects", Kind: argparse.Bool, Usage: "Every task, whatever its project"},
		argparse.Spec{Long: "tag", Short: "t", Kind: argparse.StringSlice, Usage: "Tag; repeatable, matches tasks having all of them"},
		argparse.Spec{Long: "due", Short: "d", Kind: argparse.String, Usage: "today, week, overdue, or a date"},
		argparse.Spec{Long: "pri", Kind: argparse.String, Usage: "Priority: low, med, high, or !, !!, !!!"},
		argparse.Spec{Long: "all", Short: "a", Kind: argparse.Bool, Usage: "Include done tasks"},
		argparse.Spec{Long: "done", Kind: argparse.Bool, Usage: "Only done tasks"},
		argparse.Spec{Long: "sort", Short: "s", Kind: argparse.String, Usage: "Sort by: due, pri, created"},
		argparse.Spec{Long: "color", Short: "c", Kind: argparse.Bool, Usage: "Colour the output even when it is not a terminal"},
		argparse.Spec{Long: "dates", Kind: argparse.Bool, Usage: "Show due dates instead of the time remaining"},
	)
}

func (a *App) cmdLs(args []string) error {
	set := lsFlags()
	r, err := set.Parse(args)
	if err != nil {
		return err
	}
	if pos := r.Args(); len(pos) > 0 {
		return fmt.Errorf("ls takes no positional arguments, got %q; press / inside todo tui to search titles", pos[0])
	}

	f := task.Filter{
		IncludeDone: r.Bool("all"),
		OnlyDone:    r.Bool("done"),
		Tags:        r.Strings("tag"),
	}
	// The three project selectors are mutually exclusive; honouring one silently
	// would hide the fact that the others were ignored.
	given := 0
	for _, on := range []bool{r.Changed("project"), r.Bool("no-project"), r.Bool("all-projects")} {
		if on {
			given++
		}
	}
	if given > 1 {
		return errors.New("-p, --no-project, and --all-projects cannot be used together")
	}
	switch {
	case r.Bool("all-projects"):
		f.Project = nil
	case r.Bool("no-project"):
		empty := ""
		f.Project = &empty
	default:
		p, ok, err := a.resolveProject(r)
		if err != nil {
			return err
		}
		if !ok {
			// Default to uncategorized tasks. Reaching a project's tasks needs
			// -p, so the plain list stays about what is not tied to a directory.
			p = ""
		}
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
			d, _, err := datearg.Parse(v, a.Now())
			if err != nil {
				return err
			}
			// Filtering is by day; a time of day in -d narrows nothing.
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
	WriteList(a.Out, ts, ListOptions{
		Now: a.Now(),
		// -c is an override, never a downgrade: it turns colour on where the
		// terminal check said no, and changes nothing where it already said yes.
		Color: a.Color || r.Bool("color"),
		Dates: r.Bool("dates"),
	})
	return nil
}
