package cli

import (
	"fmt"

	"todo.mirumo.net/internal/argparse"
)

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

func rmFlags() *argparse.Set {
	return argparse.New(
		argparse.Spec{Long: "force", Short: "f", Kind: argparse.Bool,
			Usage: "Destroy the task instead of putting it aside"},
	)
}

// cmdRm takes tasks out of the lists. By default the row stays, because a
// delete one keystroke away from every other command should be something you
// can take back; -f is what actually destroys it.
func (a *App) cmdRm(args []string) error {
	r, err := rmFlags().Parse(args)
	if err != nil {
		return err
	}
	ids, err := parseIDs(r.Args())
	if err != nil {
		return err
	}
	force := r.Bool("force")
	for _, id := range ids {
		// Fetch first so the message can name the title and the user can confirm they deleted the right thing.
		t, err := a.Store.Get(id)
		if err != nil {
			return fmt.Errorf("#%d: %w", id, err)
		}
		if force {
			if err := a.Store.Delete(id); err != nil {
				return fmt.Errorf("#%d: %w", id, err)
			}
			fmt.Fprintf(a.Out, "destroyed #%d: %s\n", id, t.Title)
			continue
		}
		if t.Deleted() {
			return fmt.Errorf("#%d: already deleted; rm -f %d destroys it, restore %d brings it back", id, id, id)
		}
		if err := a.Store.SetDeleted(id, true, a.Now()); err != nil {
			return fmt.Errorf("#%d: %w", id, err)
		}
		fmt.Fprintf(a.Out, "deleted #%d: %s\n", id, t.Title)
	}
	return nil
}

// cmdRestore takes tasks back out of the bin.
func (a *App) cmdRestore(args []string) error {
	ids, err := parseIDs(args)
	if err != nil {
		return err
	}
	for _, id := range ids {
		t, err := a.Store.Get(id)
		if err != nil {
			return fmt.Errorf("#%d: %w", id, err)
		}
		if !t.Deleted() {
			return fmt.Errorf("#%d: not deleted", id)
		}
		if err := a.Store.SetDeleted(id, false, a.Now()); err != nil {
			return fmt.Errorf("#%d: %w", id, err)
		}
		fmt.Fprintf(a.Out, "restored #%d: %s\n", id, t.Title)
	}
	return nil
}
