package main

import (
	"path/filepath"
	"testing"
)

func TestResolveDBPath(t *testing.T) {
	home := "/home/me"
	def := filepath.Join(home, ".todo", "todo.db")
	cases := []struct {
		name string
		env  string
		flag string
		want string
	}{
		{"default", "", "", def},
		{"env var", "/tmp/env.db", "", "/tmp/env.db"},
		{"flag overrides env var", "/tmp/env.db", "/tmp/flag.db", "/tmp/flag.db"},
		{"flag only", "", "/tmp/flag.db", "/tmp/flag.db"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveDBPath(c.env, c.flag, home); got != c.want {
				t.Errorf("= %q, want %q", got, c.want)
			}
		})
	}
}

func TestResolveColor(t *testing.T) {
	cases := []struct {
		name    string
		noColor string
		tty     bool
		want    bool
	}{
		{"terminal", "", true, true},
		{"pipe", "", false, false},
		{"NO_COLOR wins over a terminal", "1", true, false},
		{"NO_COLOR set but empty is not set", "", true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveColor(c.noColor, c.tty); got != c.want {
				t.Errorf("= %v, want %v", got, c.want)
			}
		})
	}
}
