package cli

import (
	"errors"
	"fmt"
	"strings"

	"todo.mirumo.net/internal/argparse"
	"todo.mirumo.net/internal/datearg"
	"todo.mirumo.net/internal/task"
)

// addFlags holds the field flags shared by add and edit.
func addFlags() *argparse.Set {
	return argparse.New(
		argparse.Spec{Long: "project", Short: "p", Kind: argparse.OptionalString, Usage: "Project; uses the current directory when given no value"},
		argparse.Spec{Long: "tag", Short: "t", Kind: argparse.StringSlice, Usage: "Tag; repeatable"},
		argparse.Spec{Long: "due", Short: "d", Kind: argparse.String, Usage: "Due date: tomorrow, fri, +3d, 2026-09-01, optionally with a time (today 15:00)"},
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
		// The common mistake: todo add -p "buy milk", where -p swallowed the title.
		if v, has := r.Optional("project"); has {
			return fmt.Errorf("missing title: %q was taken as the value of --project.\n"+
				"Put the title before the flags (todo add %q -p), or write --project=%s", v, v, v)
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
		d, hasTime, err := datearg.Parse(r.String("due"), now)
		if err != nil {
			return err
		}
		t.Due, t.DueHasTime = &d, hasTime
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
