package argparse

import (
	"strings"
	"testing"
)

func specs() *Set {
	return New(
		Spec{Long: "all", Short: "a", Kind: Bool, Usage: "including done"},
		Spec{Long: "due", Short: "d", Kind: String, Usage: "Due date"},
		Spec{Long: "tag", Short: "t", Kind: StringSlice, Usage: "Tag; repeatable"},
		Spec{Long: "project", Short: "p", Kind: OptionalString, Usage: "Project"},
	)
}

func TestParseLongAndShortForms(t *testing.T) {
	r, err := specs().Parse([]string{"buy milk", "--due", "2026-09-01", "-a", "-t", "shopping", "--tag=chores"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := r.Args(); len(got) != 1 || got[0] != "buy milk" {
		t.Errorf("positional args = %v, want [buy milk]", got)
	}
	if !r.Bool("all") {
		t.Error("--all should be true")
	}
	if got := r.String("due"); got != "2026-09-01" {
		t.Errorf("due = %q, want 2026-09-01", got)
	}
	if got := r.Strings("tag"); len(got) != 2 || got[0] != "shopping" || got[1] != "chores" {
		t.Errorf("tag = %v, want [shopping chores]", got)
	}
}

func TestOptionalStringThreeStates(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		changed  bool
		hasValue bool
		value    string
	}{
		{"not given", []string{"x"}, false, false, ""},
		{"given without a value", []string{"x", "-p"}, true, false, ""},
		{"no value, another flag follows", []string{"x", "-p", "-a"}, true, false, ""},
		{"space form with a value", []string{"x", "-p", "work"}, true, true, "work"},
		{"equals form with a value", []string{"x", "--project=work"}, true, true, "work"},
		{"equals form with an empty value", []string{"x", "-p="}, true, true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := specs().Parse(c.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if r.Changed("project") != c.changed {
				t.Errorf("Changed = %v, want %v", r.Changed("project"), c.changed)
			}
			v, has := r.Optional("project")
			if has != c.hasValue || v != c.value {
				t.Errorf("Optional = (%q, %v), want (%q, %v)", v, has, c.value, c.hasValue)
			}
		})
	}
}

func TestStringFlagAcceptsEmptyValue(t *testing.T) {
	r, err := specs().Parse([]string{"--due", ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.Changed("due") || r.String("due") != "" {
		t.Error("--due \"\" should count as given with an empty value")
	}
}

func TestDoubleDashEndsFlags(t *testing.T) {
	r, err := specs().Parse([]string{"--", "-a", "--due"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := r.Args(); len(got) != 2 || got[0] != "-a" || got[1] != "--due" {
		t.Errorf("positional args = %v, want [-a --due]", got)
	}
	if r.Bool("all") {
		t.Error("nothing after -- should be parsed as a flag")
	}
}

func TestErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"unknown long flag", []string{"--nope"}, "unknown flag --nope"},
		{"unknown short flag", []string{"-z"}, "unknown flag -z"},
		{"string flag missing its value", []string{"--due"}, "flag --due needs a value"},
		{"bool flag rejects a value", []string{"--all=1"}, "flag --all takes no value"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := specs().Parse(c.args)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want it to contain %q", err, c.want)
			}
		})
	}
}

func TestUsageListsFlags(t *testing.T) {
	u := specs().Usage()
	for _, want := range []string{"-a, --all", "-d, --due", "including done"} {
		if !strings.Contains(u, want) {
			t.Errorf("Usage is missing %q, got:\n%s", want, u)
		}
	}
}
