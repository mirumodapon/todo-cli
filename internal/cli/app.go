// Package cli implements the todo command line interface.
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

// App holds everything one run needs. All of it is injectable so tests stay isolated.
type App struct {
	Store store.Store
	Out   io.Writer
	Err   io.Writer
	Now   func() time.Time
	Cwd   string
	Color bool
	// RunTUI is injected by cmd/todo. cli does not import tui; they stay siblings.
	RunTUI func() error
}

// command fully describes one subcommand. All help text is generated from here,
// so the global usage and each subcommand's --help cannot drift apart.
type command struct {
	name    string
	syntax  string // the form shown in the command list, e.g. "add <title>"
	summary string
	flags   func() *argparse.Set // nil for subcommands that take no flags
	run     func([]string) error
}

// usageLine is the first line of a subcommand's help.
func (c command) usageLine() string {
	u := "todo " + c.syntax
	if c.flags != nil {
		u += " [flags]"
	}
	return u
}

// help renders one subcommand's help.
func (c command) help() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Usage:\n  %s\n\n%s\n", c.usageLine(), c.summary)
	if c.flags != nil {
		fmt.Fprintf(&b, "\nFlags:\n%s", c.flags().Usage())
	}
	return b.String()
}

func (a *App) commandList() []command {
	return []command{
		{"add", "add <title>", "Add a task.", addFlags, a.cmdAdd},
		{"ls", "ls", "List tasks. Shows open tasks only unless -a or --done is given.", lsFlags, a.cmdLs},
		{"done", "done <id>...", "Mark tasks as done.", nil, a.cmdDone},
		{"undone", "undone <id>...", "Mark tasks as not done.", nil, a.cmdUndone},
		{"edit", "edit <id> [new title]", "Change a task. Only the fields you pass are touched.", addFlags, a.cmdEdit},
		{"rm", "rm <id>...", "Delete tasks.", nil, a.cmdRm},
		{"projects", "projects", "List projects with their open task counts.", nil, a.cmdProjects},
		{"tags", "tags", "List tags that are in use.", nil, a.cmdTags},
		{"tui", "tui", "Open the interactive interface.", nil, a.cmdTUI},
	}
}

func (a *App) findCommand(name string) (command, bool) {
	for _, c := range a.commandList() {
		if c.name == name {
			return c, true
		}
	}
	return command{}, false
}

// usage renders the global help. The command list comes from commandList, so adding a subcommand needs no edit here.
func (a *App) usage() string {
	cmds := a.commandList()
	var w int
	for _, c := range cmds {
		w = max(w, len(c.syntax))
	}
	var b strings.Builder
	b.WriteString("todo — a local task list\n\nUsage:\n  todo <command> [flags]\n\nCommands:\n")
	for _, c := range cmds {
		fmt.Fprintf(&b, "  %-*s  %s\n", w, c.syntax, c.summary)
	}
	b.WriteString("\nGlobal flags:\n")
	b.WriteString("  --db <path>           Database file (default ~/.todo/todo.db, or $TODO_DB)\n")
	b.WriteString("  -h, --help            Show help\n")
	b.WriteString("\nRun \"todo <command> --help\" for details on one command.\n")
	return b.String()
}

// Run executes one command and returns the process exit code: 0 success, 1 failure, 2 usage error.
func (a *App) Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(a.Out, a.usage())
		return 0
	}
	name, rest := args[0], args[1:]

	// todo -h / --help / help [<command>]
	if name == "-h" || name == "--help" || name == "help" {
		if len(rest) > 0 {
			if c, ok := a.findCommand(rest[0]); ok {
				fmt.Fprint(a.Out, c.help())
				return 0
			}
			fmt.Fprintf(a.Err, "unknown command %q\n\n", rest[0])
			fmt.Fprint(a.Err, a.usage())
			return 2
		}
		fmt.Fprint(a.Out, a.usage())
		return 0
	}

	c, ok := a.findCommand(name)
	if !ok {
		fmt.Fprintf(a.Err, "unknown command %q\n\n", name)
		fmt.Fprint(a.Err, a.usage())
		return 2
	}
	// A subcommand's flag set does not know -h, so catch it before parsing;
	// otherwise todo add -h reports an unknown flag, which is a poor experience.
	for _, x := range rest {
		if x == "-h" || x == "--help" {
			fmt.Fprint(a.Out, c.help())
			return 0
		}
	}
	if err := c.run(rest); err != nil {
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

// SplitGlobal pulls leading --db occurrences off args and returns the rest.
// Only the front is scanned: --db is global, and a fixed position keeps it distinct from subcommand flags.
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

// resolveProject reads the three states of -p.
// The bool says whether to touch the project field; the string is the new value, possibly empty.
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
