# todo

A local task list with a command line and a terminal UI. Everything stays on
your machine in a SQLite file under `~/.todo`; nothing is sent anywhere.

```
$ todo add "buy milk" -t shopping -d "today 17:00" --pri high
added #1: buy milk

$ todo add "fix the parser" -p
added #2: fix the parser

$ todo ls
1 [ ] !!! 2h buy milk  @shopping
```

Tasks with no project are the default view. Tasks that belong to a project are
one `-p` away, which keeps the everyday list about what is not tied to a
directory.

## Install

Requires Go 1.26 or newer.

From a checkout:

```sh
make install          # into $(go env GOPATH)/bin
```

Or build without installing:

```sh
make build            # produces ./bin/todo
```

There is no cgo: the SQLite driver is pure Go, so a plain `go build` is enough
on any platform Go targets.

## Commands

```
todo add <title> [flags]        Add a task
todo ls, list [flags]           List tasks
todo done <id>...               Mark tasks as done
todo undone <id>...             Mark tasks as not done
todo edit <id> [new title]      Change a task
todo rm <id>...                 Delete tasks
todo projects                   Projects with their open counts
todo tags                       Tags that are in use
todo tui                        Open the interactive interface
```

`todo --help` lists them; `todo <command> --help` explains one.

### Fields

| Flag | Meaning |
|---|---|
| `-p`, `--project` | With no value, the current directory. With a value, that project. |
| `-t`, `--tag` | Repeatable. |
| `-d`, `--due` | `today`, `tomorrow`, `fri`, `+3d`, `+2w`, `2026-09-01`, each optionally with a time (`today 15:00`). A bare `18:00` means today. |
| `--pri` | `low`, `med`, `high`, or the marks a listing shows: `!`, `!!`, `!!!`. Quote the marks — most shells treat `!!` as history expansion: `--pri '!!!'`. |

`edit` touches only the fields you pass, so an omitted flag and an empty value
mean different things:

```sh
todo edit 3 --pri low       # priority changes, due date untouched
todo edit 3 --due ""        # due date cleared
todo edit 3 "a new title"   # title changes, nothing else
todo edit 3 --project=      # back to uncategorized
```

### Projects

`-p` with no value resolves the current directory: it walks up looking for
`.git` and uses the repository root when it finds one, so every directory in a
checkout maps to the same project. The absolute path is what gets stored —
directory names collide, paths do not — while listings show the basename.

```sh
todo add "fix the parser" -p       # project = this repository
todo ls -p                         # what is open in this repository
todo ls -p work                    # a project named by hand
todo ls --all-projects             # everything, whatever its project
todo ls                            # uncategorized only (the default)
```

### Listing

```sh
todo ls -a                  # include done tasks
todo ls --done              # only done tasks
todo ls -d today            # due today; also week, overdue, or a date
todo ls -t urgent -t home   # tasks carrying every one of these tags
todo ls -s pri              # sort by priority; also due (default) or created
todo ls -c | less -R        # force colour through a pipe
todo ls --dates             # calendar dates instead of time remaining
```

Colour is on when the output is a terminal and off when it is redirected, so
piping into a file does not fill it with escape codes. Two environment
variables override that: `CLICOLOR_FORCE` keeps colour on through a pipe,
`NO_COLOR` turns it off. `-c` and `CLICOLOR_FORCE` beat `NO_COLOR`, on the
principle that the more explicit request carries.

```sh
export CLICOLOR_FORCE=1   # always colour, without typing -c
```

Colour tracks how soon a task is due: green three days out, through yellow and
peach, red once twelve hours or less remain, and red for anything overdue.
Beyond three days nothing is coloured, so colour marks what is actually close
instead of decorating the whole list. A due date with no time of day counts as
the end of that day.

The shades come from [Catppuccin Macchiato](https://catppuccin.com/palette/).
The ramp blends between palette entries rather than between arbitrary values,
so every step along it belongs to the same set.

The due column shows how long is left, as a single unit: `12m`, `1h`, `40d`,
and negative once past. `--dates` swaps it for the calendar date, where today
reads as `today` or as the time of day when the task has one. A due date with
no time of day runs to the end of that day, so something due today still shows
the hours left in it.

## Terminal UI

`todo tui` opens the list. It starts on uncategorized tasks, like `todo ls`.

| Key | Action |
|---|---|
| `j` `k` `↑` `↓` `g` `G` | Move |
| `space` | Toggle done (asks first) |
| `a` / `e` | Add / edit |
| `d` | Delete (asks first) |
| `u` | Undo the last delete |
| `/` | Search titles |
| `P` / `T` | Filter by project / tag |
| `A` | Show or hide done tasks |
| `s` | Cycle sort order |
| `D` | Switch between time remaining and dates |
| `esc` | Back to the uncategorized default |
| `?` | Key help |
| `q` | Quit |

Completing and deleting both ask before they touch anything, and only `y`
accepts, so a mistyped key cannot confirm. A delete can still be taken back
with `u` for as long as the TUI is open.

The header names what you are looking at (`uncategorized`, a project, or
`all projects`), so the current filter is never invisible state. `P` lists each
project with how much is open in it.

## Data

`~/.todo/todo.db`, a SQLite database, created on first use with mode `0700`.

```sh
todo --db /tmp/scratch.db ls    # somewhere else, once
TODO_DB=/tmp/scratch.db todo ls # or for the whole session
```

`--db` wins over `TODO_DB`. Both are ordinary SQLite files, so `sqlite3` reads
them and copying one is a backup.

## Development

```sh
make            # list the targets
make check      # gofmt, go vet, go test — run this before committing
make test
make cover      # coverage report in a browser
make run ARGS="ls -a"
```

`make check` fails on unformatted code; `gofmt -l` alone exits 0 whatever it
finds, which makes it useless as a gate.

### Layout

Dependencies point inward, and the inner packages perform no IO.

| Package | Does |
|---|---|
| `internal/argparse` | Argument parsing. |
| `internal/task` | Domain types, validation, filter description. |
| `internal/theme` | The Catppuccin Macchiato colours the interface draws with. |
| `internal/urgency` | Turns time-until-due into a colour. |
| `internal/datearg` | Due date parsing and display. |
| `internal/project` | Turns a directory into a project path. |
| `internal/store` | The `Store` interface and its SQLite implementation. |
| `internal/cli` | Subcommands, flags, output formatting. |
| `internal/tui` | Bubble Tea model, update, view. |
| `cmd/todo` | Wiring and the exit code. |

`cli` and `tui` depend only on the `Store` interface, and neither imports the
other: `cmd/todo` hands the CLI a function that starts the TUI. Tests run
against an in-memory database and never touch `~/.todo`.

Argument parsing is hand-written because `-p` needs an optional value — no
value means the current directory, a value names a project — and neither
`pflag` nor the standard library's `flag` supports that.

`tui.Update` performs no IO. Every database action is a `tea.Cmd` whose result
comes back as a message, which keeps the update function pure and makes the
tests a matter of feeding it messages rather than driving a terminal.

## Licence

MIT. See [LICENSE](LICENSE).
