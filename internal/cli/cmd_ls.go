package cli

import (
	"errors"
	"fmt"

	"todo.mirumo.net/internal/argparse"
	"todo.mirumo.net/internal/task"
)

// listFlags are the flags ls and tui share: everything that narrows the list,
// plus --dates, which chooses how a due date reads in both.
func listFlags() []argparse.Spec {
	return []argparse.Spec{
		argparse.Spec{Long: "project", Short: "p", Kind: argparse.OptionalString, Usage: "Only this project; uses the current directory when given no value"},
		argparse.Spec{Long: "no-project", Kind: argparse.Bool, Usage: "Only uncategorized tasks (the default)"},
		argparse.Spec{Long: "all-projects", Kind: argparse.Bool, Usage: "Every task, whatever its project"},
		argparse.Spec{Long: "tag", Short: "t", Kind: argparse.StringSlice, Usage: "Tag; repeatable, matches tasks having all of them"},
		argparse.Spec{Long: "due", Short: "d", Kind: argparse.String, Usage: "today, week, overdue, or a date"},
		argparse.Spec{Long: "pri", Kind: argparse.String, Usage: "Priority: low, med, high, or !, !!, !!!"},
		argparse.Spec{Long: "all", Short: "a", Kind: argparse.Bool, Usage: "Include done tasks"},
		argparse.Spec{Long: "done", Kind: argparse.Bool, Usage: "Only done tasks"},
		argparse.Spec{Long: "deleted", Kind: argparse.Bool, Usage: "Only deleted tasks, the ones rm put aside"},
		argparse.Spec{Long: "sort", Short: "s", Kind: argparse.String, Usage: "Sort by: id (default), due, pri"},
		argparse.Spec{Long: "reverse", Short: "r", Kind: argparse.Bool, Usage: "Reverse whatever order is in force"},
		argparse.Spec{Long: "dates", Kind: argparse.Bool, Usage: "Show due dates instead of the time remaining"},
	}
}

func lsFlags() *argparse.Set {
	// Colour is the one flag tui has no use for: the interface always colours.
	return argparse.New(append(listFlags(),
		argparse.Spec{Long: "color", Short: "c", Kind: argparse.Bool, Usage: "Colour the output even when it is not a terminal"},
	)...)
}

func tuiFlags() *argparse.Set { return argparse.New(listFlags()...) }

func (a *App) cmdLs(args []string) error {
	set := lsFlags()
	r, err := set.Parse(args)
	if err != nil {
		return err
	}
	if pos := r.Args(); len(pos) > 0 {
		return fmt.Errorf("ls takes no positional arguments, got %q; press / inside task tui to search titles", pos[0])
	}

	f, err := a.filterFrom(r)
	if err != nil {
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

// filterFrom turns parsed flags into a query. ls and tui both come through
// here, so the two cannot come to disagree about what a flag means.
func (a *App) filterFrom(r *argparse.Result) (task.Filter, error) {
	f := task.Filter{
		IncludeDone: r.Bool("all"),
		OnlyDone:    r.Bool("done"),
		Tags:        r.Strings("tag"),
		Reverse:     r.Bool("reverse"),
		OnlyDeleted: r.Bool("deleted"),
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
		return f, errors.New("-p, --no-project, and --all-projects cannot be used together")
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
			return f, err
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
			return f, err
		}
		f.Priority = &p
	}
	if r.Changed("due") {
		var err error
		if f.DueRange, f.DueOn, err = task.ParseDueFilter(r.String("due"), a.Now()); err != nil {
			return f, err
		}
	}
	var err error
	if f.Sort, err = task.ParseSortBy(r.String("sort")); err != nil {
		return f, err
	}
	return f, nil
}
