# todo: a local task list, CLI + TUI design

Date: 2026-08-29
Status: approved, implementation plan to follow

## Goal

A purely local task tool with the CLI as the main interface and a TUI for browsing and batch work. The data lives in the user's home directory and never touches the network.

## Non-goals

- Synchronisation, servers, multiple devices in any form (`todo.mirumo.net` is a module path, not a service)
- Recurring tasks, subtasks, reminders
- Shell completion (a static script can be added later if it is wanted)

## Technology

- Go 1.26
- Module path `todo.mirumo.net`, binary named `todo`
- TUI: Bubble Tea + Lip Gloss
- Storage: SQLite through `modernc.org/sqlite` (pure Go, no cgo)
- Argument parsing: **hand-written** (see below). No Cobra, no pflag, no stdlib `flag`

The only external dependencies are the Bubble Tea family and the SQLite driver.

## Layers

Dependencies point inward, and the inner layers do no IO.

| Package | Responsibility | Depends on |
|---|---|---|
| `internal/argparse` | GNU-style argument parsing, including optional-value flags | nothing |
| `internal/task` | Domain types `Task`, `Priority`, `Filter`, `SortBy`; field validation | nothing |
| `internal/datearg` | Date parsing and human-readable display | stdlib |
| `internal/project` | Derives a project path from the current directory | stdlib |
| `internal/store` | The `Store` interface, its SQLite implementation, schema migration | `task` |
| `internal/cli` | Subcommand dispatch, flag definitions, output formatting | `task` `store` `datearg` `argparse` `project` |
| `internal/tui` | Bubble Tea model / update / view | `task` `store` `datearg` `project` |
| `cmd/todo` | Wiring: open the database, build the store, dispatch | everything |

`Store` is an interface (`Add / Get / List(Filter) / Update / Delete / SetDone / Tags / Projects`). The CLI and the TUI know only the interface, so tests swap in `:memory:` SQLite or a fake and never touch the user's real data.

## Data

### Location

`~/.todo/todo.db`. The directory is created on first run with mode 0700 if it does not exist. A `--db <path>` flag or the `TODO_DB` environment variable overrides it, which is how tests point at a temporary directory. XDG is not used.

### Schema

```sql
CREATE TABLE tasks (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  title      TEXT    NOT NULL,
  project    TEXT    NOT NULL DEFAULT '',
  due        TEXT    NULL,                    -- YYYY-MM-DD, no time part
  priority   INTEGER NOT NULL DEFAULT 0,      -- 0 none / 1 low / 2 med / 3 high
  done_at    TEXT    NULL,                    -- NULL = not done; doubles as the completion time
  created_at TEXT    NOT NULL,
  updated_at TEXT    NOT NULL
);
CREATE TABLE tags (
  id   INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE
);
CREATE TABLE task_tags (
  task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  tag_id  INTEGER NOT NULL REFERENCES tags(id)  ON DELETE CASCADE,
  PRIMARY KEY (task_id, tag_id)
);
```

Decisions and why:

- `done_at` is a nullable timestamp rather than a bool, so one column says both "is it done" and "when was it done"
- `priority` is stored as an integer so it can be used in `ORDER BY` directly
- Tags are normalised into a join table, which is what makes listing every tag cheap
- `project` is a plain column with no table of its own; a task belongs to exactly one, and `SELECT DISTINCT project` is enough
- ids use `AUTOINCREMENT` and are never reused after a delete, because the user types them: `todo done 3`
- Every connection must run `PRAGMA foreign_keys = ON` (SQLite defaults it off, and without it the CASCADE does nothing)

### What the project field means

The empty string is a first-class state meaning "uncategorized, global", not a missing value. `todo add "buy milk"` is a global task. Uncategorized rows print no project column at all, not a `(none)` placeholder.

`-p` with no value is resolved by `internal/project`: walk up from the current directory looking for `.git`, use the repository root if one is found, otherwise the working directory. The absolute path is stored (directory names collide — two repositories both have `docs/`), only the basename is displayed, and the full path is left to `todo projects`.

## Argument parsing (`internal/argparse`)

Why not an existing library: `-p` has to mean both "no value, so the current directory" and "a value, so that project". Neither pflag nor the stdlib `flag` supports optional-value flags, because in general `-p work` cannot be told apart from `-p` followed by a positional argument.

Supported syntax:

```
--project work    --project=work    -p work    -p=work    -p
--all             -a                --            (end of flags)
```

Three kinds of flag:

- **bool**: never consumes the next token
- **string / repeatable string**: requires a value, an error when it is missing
- **optional-value string**: consumes the next token if it exists and does not start with `-`, otherwise counts as "given without a value"

The result has to distinguish three states — absent, given without a value, given with a value. Both `edit`'s "leave it alone vs. clear it" and `-p`'s "current directory vs. named project" depend on it.

Bundled short flags (`-at`) are not supported. An unknown flag is an error that lists the flags that subcommand does accept.

**A known ambiguity**: `todo add -p "buy milk"` swallows the title as the project name. The mitigation: `add` takes exactly one positional argument, so if the positional is empty after parsing and `-p` happens to have consumed a value, that is an error suggesting "put the title first, or use `--project=buy milk`". The idiomatic form is `todo add "buy milk" -p`.

## CLI

```
todo                          print usage
todo tui                      enter the TUI
todo add <title> [-p [name]] [-t tag]... [-d due] [--pri low|med|high]
todo ls   [-p [name]] [--no-project] [-t tag] [-d today|week|overdue|<date>]
          [--pri p] [-a] [--done] [-s due|pri|created]
todo done   <id>...
todo undone <id>...
todo rm     <id>...
todo edit <id> [the same flags as add]
todo projects                 list every project with its open count
todo tags                     list every tag
```

Global flags: `--db <path>`, `-h/--help`.

`ls` lists only unfinished tasks by default; `-a` includes done ones and `--done` shows only those. The default sort is `due`, with undated tasks last.

`edit` decides what to change by whether a flag was given at all: `--due ""` clears the due date, leaving `--due` out leaves it alone.

`ls -t` is repeatable and several tags intersect (AND). `-p` together with `--no-project` is an error rather than a guess at what was meant.

Output is aligned plain-text columns. Colour is turned off when stdout is not a TTY (a pipe, a redirect).

`add` validates flag values as it parses: `--pri` takes only the three words, `--due` goes through `datearg`, `--tag` is deduplicated. The error message names the flag at fault.

## Date parsing (`internal/datearg`)

Input accepts: `today`, `tomorrow`, `yesterday`, weekday abbreviations (`mon`..`sun`, meaning the next such day), relative amounts (`+3d`, `+2w`), and absolute `YYYY-MM-DD`.

Display produces human-readable strings: `today`, `tomorrow`, `Fri`, `2d overdue`, `09-01`.

`ls -d` additionally accepts the range keywords `today` / `week` / `overdue`.

## TUI

The root model holds a mode and `Update` dispatches on it:

```
list (default) → search (/ incremental search)
               → form (a add / e edit)
               → picker (P projects / T tags)
               → help (? overlay)
```

### Keys in list mode

| Key | Action |
|---|---|
| `j` `k` `↑` `↓` `g` `G` | Move |
| `space` | Toggle done |
| `a` / `e` | Add / edit (opens the form) |
| `d` | Delete, with `deleted "…" · u to undo` at the bottom |
| `u` | Undo the last delete |
| `/` | Search titles, filtering as you type; `esc` cancels, `enter` keeps it |
| `P` `T` | Filter by project / tag from a menu that includes an "uncategorized" entry |
| `A` | Show or hide done tasks |
| `s` | Cycle the sort: due → pri → created |
| `esc` | Clear every filter |
| `?` | Help overlay |
| `q` | Quit |

Deleting undoes rather than confirms: a confirmation dialog charges for every delete, while undo only costs anything when the delete was actually a mistake. Undo keeps a single level (the whole Task, tags included) and is dropped when the TUI exits. Restoring reinserts under the original id — `AUTOINCREMENT` never reuses numbers, so that id is guaranteed to still be free.

### The form

Shared by `a` and `e`. Five fields: title / project / tags / due / priority. `tab` and `shift+tab` move between them, `enter` saves, `esc` cancels. The due field is parsed by `datearg`; an invalid value turns the field red and blocks saving. An empty project field means globally uncategorized, and one key fills in the current directory's project (through the same `internal/project` the CLI's `-p` uses).

### Data flow

Every store operation goes through a `tea.Cmd` and comes back to `Update` as a msg. `Update` never touches the database itself, which keeps it a pure function. Each change is followed by one reload so the screen cannot drift from the database; the list is small enough that requerying costs nothing worth counting.

## Errors

- TUI: store errors become an `errMsg` shown in red on the status line at the bottom. Nothing crashes and nothing is interrupted
- CLI: the message goes to stderr and the exit code is non-zero
- An id that does not exist: say which id, never skip it silently
- Deleting a task can leave a tag no task references any more. Nothing cleans those up (they are harmless), but `todo tags` and the TUI's tag menu list only tags at least one task references
- A corrupt or unopenable database file: the message carries both the path and the underlying error

## Testing

| Layer | How |
|---|---|
| `argparse` `task` `datearg` `project` | Table-driven tests over pure functions |
| `store` | Real SQL against `:memory:` SQLite |
| `cli` | A fake Store, asserting flags → store calls → output |
| `tui` | Feed a sequence of msgs to `Update` and assert the model's transitions |

No test may touch `~/.todo`; anything that needs real files uses `t.TempDir()`.

## Layout

```
cmd/todo/main.go
internal/argparse/
internal/task/
internal/datearg/
internal/project/
internal/store/
internal/cli/
internal/tui/
docs/superpowers/specs/
```
