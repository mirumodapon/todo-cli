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
