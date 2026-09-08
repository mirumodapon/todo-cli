# task

A local task list with a command line and a terminal UI. Everything stays on
your machine in a SQLite file under `~/.todo`; nothing is sent anywhere.

```
$ task add "buy milk" -t shopping -d "today 17:00" --pri high
added #1: buy milk

$ task add "fix the parser" -p
added #2: fix the parser

$ task ls
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
make build            # produces ./bin/task
```

There is no cgo: the SQLite driver is pure Go, so a plain `go build` is enough
on any platform Go targets.

`task --version` reports the revision it was built from. Release builds can
stamp a version instead:

```sh
go build -ldflags "-X main.version=v1.2.3" ./cmd/task
```

## Commands

```
task add <title> [flags]        Add a task
task ls, list [flags]           List tasks
task details <id>...            Show tasks in full, description included
task done <id>...               Mark tasks as done
task undone <id>...             Mark tasks as not done
task edit <id> [new title]      Change a task
task rm <id>...                 Delete tasks
task projects                   Projects with their open counts
task tags                       Tags that are in use
task tui                        Open the interactive interface
task mcp                        Serve the task list over MCP on stdin/stdout
```

`task --help` lists them; `task <command> --help` explains one.
`task --version` reports the build.

### Fields

| Flag | Meaning |
|---|---|
| `-p`, `--project` | With no value, the current directory. With a value, that project. |
| `-t`, `--tag` | Repeatable. |
| `-d`, `--due` | `today`, `tomorrow`, `fri`, `+3d`, `+2w`, `2026-09-01`, each optionally with a time (`today 15:00`). A bare `18:00` means today. |
| `--pri` | `low`, `med`, `high`, or the marks a listing shows: `!`, `!!`, `!!!`. Quote the marks — most shells treat `!!` as history expansion: `--pri '!!!'`. |
| `--desc` | The long form of the task, over as many lines as it takes. With no value it opens `$EDITOR`; with one it takes the value. Listings show only the title; `task details` and `enter` in the TUI show it. |

`edit` touches only the fields you pass, so an omitted flag and an empty value
mean different things:

```sh
task edit 3 --pri low       # priority changes, due date untouched
task edit 3 --due ""        # due date cleared
task edit 3 "a new title"   # title changes, nothing else
task edit 3 --project=      # back to uncategorized
```

### Projects

`-p` with no value resolves the current directory: it walks up looking for
`.git` and uses the repository root when it finds one, so every directory in a
checkout maps to the same project. The absolute path is what gets stored —
directory names collide, paths do not — while listings show the basename.

```sh
task add "fix the parser" -p       # project = this repository
task ls -p                         # what is open in this repository
task ls -p work                    # a project named by hand
task ls --all-projects             # everything, whatever its project
task ls                            # uncategorized only (the default)
```

### Listing

```sh
task ls -a                  # include done tasks
task ls --done              # only done tasks
task ls -d today            # due today; also week, overdue, or a date
task ls -t urgent -t home   # tasks carrying every one of these tags
task ls -s pri              # sort by priority; also due (default) or created
task ls -c | less -R        # force colour through a pipe
task ls --dates             # calendar dates instead of time remaining
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

### Details

A listing shows one line per task. `task details` shows everything one task
carries, including the description:

```
$ task details 1
#1  renew the passport
  status    open
  due       2026-09-12  (9d)
  priority  !!! high
  tags      @admin
  created   2026-09-03 17:09

  form DS-82, two photos taken in the last six months
  the post office on the corner does them for a fiver
```

A description is a paragraph, and a paragraph does not belong on a command
line, so `--desc` with no value opens your editor on it — `$VISUAL`, then
`$EDITOR`, then `vi`, run through a shell so `EDITOR="code -w"` works:

```sh
task add "renew the passport" --desc   # opens an empty file
task edit 1 --desc                     # opens the current text
task edit 1 --desc ""                  # clears it, no editor
```

The file has no comment lines to strip, so a description may start with `#`,
and quitting the editor with an error aborts the edit rather than saving what
happens to be in the file.

Fields with nothing in them are left out, so a task with only a title prints
two lines rather than a column of blanks. The due date is written out in full
here — this is the view you come to when you want to settle what a date is,
and it has no columns to keep narrow. Several ids at once are fine:
`task details 1 2 3`.

## Terminal UI

`task tui` opens the list. With no flags it starts on uncategorized tasks, like
`task ls`, and it takes the same filters — a query worth typing once does not
have to be rebuilt with keystrokes after the interface opens:

```sh
task tui -p                    # this repository's tasks
task tui -t urgent -d week     # tagged urgent and due within the week
task tui --all-projects -a     # everything, done ones included
```

`r` rereads the database. The CLI and the MCP server write to the same file
while the interface is open, and nothing tells it when they do.

`esc` returns to whatever those flags asked for, not to the built-in default:
what you typed to open the interface is this session's default. `-c` is the one
flag `ls` has that `tui` does not — the interface always colours.

| Key | Action |
|---|---|
| `j` / `k` / `↑` / `↓` | Move |
| `ctrl+n` / `ctrl+p` | Move, including while typing |
| `g` / `G` | Jump to top / bottom |
| `enter` | Show the full task |
| `space` | Toggle done (asks first) |
| `a` / `e` | Add / edit |
| `E` | Edit the whole task in $EDITOR |
| `d` | Delete (asks first) |
| `u` | Undo the last delete |
| `/` | Search titles |
| `P` / `T` | Filter by project / tag |
| `A` | Show or hide done tasks |
| `s` | Cycle sort order |
| `D` | Switch between time remaining and dates |
| `v` | Show or hide the detail pane |
| `r` | Reread the database |
| `esc` | Back to the filter it opened on |
| `?` | This help |
| `q` | Quit |

Completing and deleting both ask before they touch anything, and only `y`
accepts, so a mistyped key cannot confirm. A delete can still be taken back
with `u` for as long as the TUI is open.

On a terminal with room to spare, the task under the cursor is detailed in a
pane that follows the cursor as it moves:

```
3 tasks · uncategorized

▶ [ ] !!! 8h first @urgent   │ #1  first
  [ ] second                 │
  [ ] third                  │ status    open
                             │ due       2026-08-29  (8h)
                             │ priority  !!! high
                             │ tags      @urgent
                             │
                             │ semi-skimmed, two litres, from the corner
                             │ shop before it shuts
```

Where the pane goes depends on the shape of the terminal: beside the list from
100 columns, underneath it from 30 rows, and nowhere at all below both — task
lines are short, so width is the room worth using first. The pane takes the
larger share of the split, around three fifths: a task line is a marker, a
status, a date and a title, while the pane holds prose. `v` hides it and gives
the room back. A description too long for the pane is cut with a marker rather
than run off the edge.

`enter` opens the task under the cursor in full, the same fields `task details`
prints, which is how to read a description the pane had to cut. Any key but `E`
closes it again.

`E` hands the whole task to `$EDITOR`, from the list or from the detail view:
the interface steps aside while the editor owns the terminal and comes back when
it exits. The file is a mail header — fields, a blank line, then the description:

```
# Fields above the blank line, description below it. # starts a comment.
# Removing a line leaves that field alone; emptying its value clears it.
title: renew the passport
project: /Users/me/Projects/home
tags: admin, paperwork
due: 2026-09-12
priority: high

form DS-82, two photos
```

Because the description lives below the blank line, it can contain anything at
all — including lines that look like fields, or start with `#`. A file that
will not parse is not thrown away: the error says where the text was left.

Lowercase `e` still opens the form over the five short fields, and the form
still leaves the description alone.

`ctrl+n` and `ctrl+p` move everywhere, including the places where `j` and `k`
are text: while searching they walk the results without leaving the field, and
in the form they step between fields.

In the add and edit form, `tab` and `shift+tab` also move between fields,
`ctrl+r` fills in the current directory's project, `enter` saves and `esc`
cancels. In the project and tag menus, `j` and `k` move, `enter` selects and
`esc` closes.

Both menus open with an entry that clears the filter, followed by the tasks
that have no value at all — `(uncategorized)` and `(untagged)`. Neither can be
named by a project path or a tag, and uncategorized is offered even when it is
empty, since that is where the list starts.

The header names what you are looking at (`uncategorized`, a project, or
`all projects`, plus any tag), so the current filter is never invisible state.
`P` lists each project with how much is open in it.

## MCP

`task mcp` speaks the [Model Context Protocol](https://modelcontextprotocol.io)
over stdin and stdout, so an MCP client can work with the same `~/.todo` the CLI
and the TUI use. It is a third interface over one database, not a copy of it.

For Claude Code:

```sh
claude mcp add task -- task mcp
```

For a client configured by file:

```json
{
  "mcpServers": {
    "task": { "command": "task", "args": ["mcp"] }
  }
}
```

Add `"env": {"TODO_DB": "/path/to/todo.db"}` to point one client at a different
database — handy for trying it out without touching your real list.

### Using it

Once connected, ask in plain language and the client picks the tools:

> **What have I got due this week?**
> → `list_tasks` with `due: "week"`

> **Add "renew the passport" for Friday, high priority, tagged admin.**
> → `add_task` with `due: "fri"`, `priority: "high"`, `tags: ["admin"]`

> **What was that passport one about?**
> → `get_task`, which is the one that carries the description

> **I did the milk one.**
> → `complete_task`

Dates go in the way they do on the command line — `tomorrow`, `fri`, `+3d`,
`2026-09-01`, optionally with a time — because both go through the same parser.
Priorities take `low`/`med`/`high` or `!`/`!!`/`!!!`.

The two prompts are worth reaching for by name rather than describing what you
want: `plan_today` and `review_project` (which takes a project path). A client
lists them wherever it lists its prompts — in Claude Code they show up among the
slash commands. They arrive with your current tasks already written into them,
so the model is not planning a day it cannot see.

`list_tasks` reads `project` in three ways, which is the same distinction the
CLI draws with `-p`:

| `project` | Means |
|---|---|
| left out | every project |
| `""` | uncategorized only |
| `"/path/to/repo"` | that project |

### Checking it by hand

The server is a pipe, so it can be driven without a client at all — which is
how to tell a broken server from a broken client:

```sh
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"sh","version":"1"}}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_tasks","arguments":{"project":""}}}' \
  | task mcp
```

```
{"jsonrpc":"2.0","id":1,"result":{"capabilities":{...},"protocolVersion":"2025-06-18","serverInfo":{"name":"task",...}}}
{"jsonrpc":"2.0","id":2,"result":{"content":[{"text":"[{\"id\":1,\"title\":\"buy milk\",...}]","type":"text"}],"isError":false}}
```

`tools/list`, `resources/list` and `prompts/list` take no parameters and are the
quickest way to see what a build offers.

### What it exposes

| Tool | Does |
|---|---|
| `list_tasks` | Filter by project, tags, due range, priority, or text. Read-only. |
| `get_task` | One task in full, description included. Read-only. |
| `add_task` | Add a task, returning its id. |
| `edit_task` | Change a task. Only the fields you pass are touched. |
| `complete_task`, `reopen_task` | Mark done, or undo that. |
| `delete_task` | Delete a task, annotated destructive so a client can ask first. |

Two resources — `task://projects` and `task://tags` — carry the project list
with open counts, and the tags in use. Two prompts write the current tasks into
the request: `plan_today` splits what is overdue, due today, and due later this
week; `review_project` walks one project's open work.

Listings leave descriptions out and set `has_desc` instead, so a long list
cannot become a wall of prose; `get_task` fetches the one you want.

A tool that fails answers with a result marked `isError` rather than a protocol
error, because the message is meant for the model to read and act on. Asking for
a tool that does not exist is a protocol error, because retrying will not help.

Requests are handled one at a time. This is one person's task list, and a queue
of one removes every question about concurrent writes.

Nothing here opens a socket: the transport is the pipe the client already
started the process on, and stdout carries JSON-RPC and nothing else.

## Data

`~/.todo/todo.db`, a SQLite database, created on first use with mode `0700`.
The directory keeps the older name, so a database written before the command
was called `task` is still the one it opens. `TODO_DB` is unchanged for the same
reason.

```sh
task --db /tmp/scratch.db ls    # somewhere else, once
TODO_DB=/tmp/scratch.db task ls # or for the whole session
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
| `internal/editor` | Hands text to `$EDITOR` and reads back what came out. |
| `internal/taskfile` | Renders a task as an editable file and parses it back. |
| `internal/cli` | Subcommands, flags, output formatting. |
| `internal/tui` | Bubble Tea model, update, view. |
| `internal/mcp` | The MCP server: JSON-RPC over stdio, tools, resources, prompts. |
| `cmd/task` | Wiring and the exit code. |

`cli`, `tui` and `mcp` depend only on the `Store` interface, and none imports
another: `cmd/task` hands the CLI the functions that start the other two. Tests
run against an in-memory database and never touch `~/.todo`.

Argument parsing is hand-written because `-p` needs an optional value — no
value means the current directory, a value names a project — and neither
`pflag` nor the standard library's `flag` supports that.

The MCP transport is hand-written for the same reason as the flag parser: it is
JSON-RPC 2.0, one object per line, which is small enough that a dependency would
cost more than it saved.

`tui.Update` performs no IO. Every database action is a `tea.Cmd` whose result
comes back as a message, which keeps the update function pure and makes the
tests a matter of feeding it messages rather than driving a terminal.

## Licence

MIT. See [LICENSE](LICENSE).
