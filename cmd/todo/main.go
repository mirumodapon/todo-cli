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

// run wraps the whole flow so deferred calls still run; os.Exit skips them.
func run() int {
	dbFlag, args, err := cli.SplitGlobal(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		return 2
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot find the home directory: %s\n", err)
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

// resolveDBPath picks the database location: --db beats TODO_DB, and ~/.todo/todo.db is the fallback.
func resolveDBPath(envDB, flagDB, home string) string {
	if flagDB != "" {
		return flagDB
	}
	if envDB != "" {
		return envDB
	}
	return filepath.Join(home, ".todo", "todo.db")
}

// isTTY reports whether output is a terminal; colour is dropped when redirected to a file or pipe.
func isTTY(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
