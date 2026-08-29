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

	// Touch only the fields that were given. An absent flag and an empty value are different things.
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
