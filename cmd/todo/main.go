// Command todo is a local task list.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"

	"todo.mirumo.net/internal/cli"
	"todo.mirumo.net/internal/store"
	"todo.mirumo.net/internal/tui"
)

// version is empty for ordinary builds, where the value is recovered from the
// build info instead. Release builds may stamp it:
//
//	go build -ldflags "-X main.version=v1.2.3" ./cmd/todo
var version = ""

func main() { os.Exit(run()) }

// run wraps the whole flow so deferred calls still run; os.Exit skips them.
func run() int {
	// Answered before anything touches the filesystem: asking a program its
	// version should not create a database directory.
	if wantsVersion(os.Args[1:]) {
		bi, ok := debug.ReadBuildInfo()
		fmt.Printf("todo %s\n", versionString(version, bi, ok))
		if ok {
			fmt.Printf("built with %s for %s/%s\n", bi.GoVersion, runtime.GOOS, runtime.GOARCH)
		}
		return 0
	}

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
		Color: resolveColor(os.Getenv("NO_COLOR"), os.Getenv("CLICOLOR_FORCE"), isTTY(os.Stdout)),
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

// resolveColor decides whether to colour output by default. Colour follows the
// terminal unless one of the two cross-tool conventions overrides it:
// CLICOLOR_FORCE turns it on even through a pipe, NO_COLOR turns it off
// (https://no-color.org). Forcing wins over suppressing, on the same principle
// that makes ls -c beat NO_COLOR: the more explicit request carries.
func resolveColor(noColor, forceColor string, tty bool) bool {
	if forceColor != "" {
		return true
	}
	if noColor != "" {
		return false
	}
	return tty
}

// isTTY reports whether output is a terminal; colour is dropped when redirected to a file or pipe.
func isTTY(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// wantsVersion reports whether --version appears as a flag, wherever it sits,
// matching how -h is accepted after a subcommand. Anything after -- is an
// argument rather than a flag.
func wantsVersion(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if a == "--version" {
			return true
		}
	}
	return false
}

// versionString works out what to report.
//
// A stamped version wins. Otherwise VCS information decides: its presence
// means the binary came from a working tree, so the revision is the honest
// answer — Go synthesises a pseudo-version for such builds, and reporting
// v0.0.0-20260830064323-6b4db23794f0+dirty helps nobody. Only a build with no
// VCS information, which is what "go install module@version" produces, falls
// back to the module version.
func versionString(stamped string, bi *debug.BuildInfo, ok bool) string {
	if stamped != "" {
		return stamped
	}
	if !ok {
		return "unknown"
	}

	var revision string
	var modified bool
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if revision == "" {
		if v := bi.Main.Version; v != "" && v != "(devel)" {
			return v
		}
		return "devel"
	}
	if len(revision) > 7 {
		revision = revision[:7]
	}
	if modified {
		return fmt.Sprintf("devel (%s, modified)", revision)
	}
	return fmt.Sprintf("devel (%s)", revision)
}
