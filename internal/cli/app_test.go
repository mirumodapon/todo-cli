package cli

import (
	"errors"
	"strings"
	"testing"
	"unicode"
)

func TestRunNoArgsPrintsUsage(t *testing.T) {
	app, out, _ := newApp(t)
	if code := app.Run(nil); code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Errorf("bare todo should print usage, got %q", out.String())
	}
	if !strings.Contains(out.String(), "Commands:") {
		t.Errorf("the global help should list the subcommands, got %q", out.String())
	}
	if strings.Contains(out.String(), "No matching tasks") {
		t.Error("bare todo must not enter the list or the TUI")
	}
}

func TestRunHelp(t *testing.T) {
	for _, arg := range []string{"-h", "--help", "help"} {
		app, out, _ := newApp(t)
		if code := app.Run([]string{arg}); code != 0 {
			t.Errorf("%s exit code = %d, want 0", arg, code)
		}
		if !strings.Contains(out.String(), "Usage:") {
			t.Errorf("%s did not print usage", arg)
		}
	}
}

func TestSubcommandHelpIsSpecificToThatCommand(t *testing.T) {
	// A subcommand's flag set does not know -h, so it must be caught before dispatch,
	// and it must print that subcommand's own help rather than the global usage.
	app, out, errBuf := newApp(t)
	if code := app.Run([]string{"add", "-h"}); code != 0 {
		t.Errorf("exit code = %d, want 0; stderr = %q", code, errBuf.String())
	}
	s := out.String()
	if !strings.Contains(s, "todo add <title>") {
		t.Errorf("it should print add's own usage line, got %q", s)
	}
	if !strings.Contains(s, "-p, --project") || !strings.Contains(s, "-d, --due") {
		t.Errorf("it should list add's flags, got %q", s)
	}
	if strings.Contains(s, "Commands:") {
		t.Errorf("a subcommand help must not fall back to the global usage: %q", s)
	}
}

func TestEverySubcommandHasHelp(t *testing.T) {
	for _, name := range []string{"add", "ls", "done", "undone", "edit", "rm", "projects", "tags", "tui"} {
		app, out, errBuf := newApp(t)
		if code := app.Run([]string{name, "--help"}); code != 0 {
			t.Errorf("%s --help exit code = %d; stderr = %q", name, code, errBuf.String())
			continue
		}
		s := out.String()
		if !strings.Contains(s, "Usage:") || !strings.Contains(s, "todo "+name) {
			t.Errorf("%s --help did not print its own usage: %q", name, s)
		}
	}
}

func TestHelpSubcommandTakesACommandName(t *testing.T) {
	app, out, _ := newApp(t)
	if code := app.Run([]string{"help", "ls"}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out.String(), "todo ls") || strings.Contains(out.String(), "Commands:") {
		t.Errorf("todo help ls should print ls's help: %q", out.String())
	}
}

func TestHelpTextIsEnglish(t *testing.T) {
	app, out, _ := newApp(t)
	app.Run(nil)
	for _, r := range out.String() {
		if unicode.Is(unicode.Han, r) {
			t.Fatalf("help text must be English but contains %q:\n%s", r, out.String())
		}
	}
}

func TestRunUnknownCommand(t *testing.T) {
	app, _, errBuf := newApp(t)
	if code := app.Run([]string{"frobnicate"}); code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), `unknown command "frobnicate"`) {
		t.Errorf("stderr = %q", errBuf.String())
	}
}

func TestTUIOnlyOnExplicitSubcommand(t *testing.T) {
	app, _, _ := newApp(t)
	called := false
	app.RunTUI = func() error { called = true; return nil }

	if code := app.Run(nil); code != 0 || called {
		t.Error("bare todo must not start the TUI")
	}
	if code := app.Run([]string{"tui"}); code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !called {
		t.Error("todo tui should start the TUI")
	}
}

func TestTUIErrorBecomesExitCode1(t *testing.T) {
	app, _, errBuf := newApp(t)
	app.RunTUI = func() error { return errors.New("the terminal broke") }
	if code := app.Run([]string{"tui"}); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "the terminal broke") {
		t.Errorf("stderr = %q", errBuf.String())
	}
}

func TestSplitGlobal(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantDB   string
		wantRest []string
	}{
		{"no --db", []string{"ls", "-a"}, "", []string{"ls", "-a"}},
		{"space form", []string{"--db", "/tmp/x.db", "ls"}, "/tmp/x.db", []string{"ls"}},
		{"equals form", []string{"--db=/tmp/x.db", "ls"}, "/tmp/x.db", []string{"ls"}},
		{"only --db", []string{"--db=/tmp/x.db"}, "/tmp/x.db", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db, rest, err := SplitGlobal(c.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if db != c.wantDB {
				t.Errorf("db = %q, want %q", db, c.wantDB)
			}
			if strings.Join(rest, " ") != strings.Join(c.wantRest, " ") {
				t.Errorf("rest = %v, want %v", rest, c.wantRest)
			}
		})
	}
	if _, _, err := SplitGlobal([]string{"--db"}); err == nil {
		t.Error("--db without a value should fail")
	}
}

func TestParseIDs(t *testing.T) {
	got, err := parseIDs([]string{"3", "17"})
	if err != nil || len(got) != 2 || got[0] != 3 || got[1] != 17 {
		t.Errorf("= %v, %v", got, err)
	}
	for _, bad := range [][]string{{}, {"x"}, {"0"}, {"-1"}} {
		if _, err := parseIDs(bad); err == nil {
			t.Errorf("parseIDs(%v) should fail", bad)
		}
	}
}
