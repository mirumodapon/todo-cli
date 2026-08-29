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
