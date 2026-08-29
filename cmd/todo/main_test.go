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
		{"預設", "", "", def},
		{"環境變數", "/tmp/env.db", "", "/tmp/env.db"},
		{"flag 覆寫環境變數", "/tmp/env.db", "/tmp/flag.db", "/tmp/flag.db"},
		{"只有 flag", "", "/tmp/flag.db", "/tmp/flag.db"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveDBPath(c.env, c.flag, home); got != c.want {
				t.Errorf("= %q，預期 %q", got, c.want)
			}
		})
	}
}
