package main

import (
	"path/filepath"
	"runtime/debug"
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
		force   string
		tty     bool
		want    bool
	}{
		{"terminal", "", "", true, true},
		{"pipe", "", "", false, false},
		{"NO_COLOR wins over a terminal", "1", "", true, false},
		{"NO_COLOR set but empty is not set", "", "", true, true},
		{"CLICOLOR_FORCE colours a pipe", "", "1", false, true},
		{"CLICOLOR_FORCE beats NO_COLOR", "1", "1", true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveColor(c.noColor, c.force, c.tty); got != c.want {
				t.Errorf("= %v, want %v", got, c.want)
			}
		})
	}
}

func buildInfo(mainVersion string, settings map[string]string) *debug.BuildInfo {
	bi := &debug.BuildInfo{
		Main:      debug.Module{Version: mainVersion},
		GoVersion: "go1.26.4",
	}
	for k, v := range settings {
		bi.Settings = append(bi.Settings, debug.BuildSetting{Key: k, Value: v})
	}
	return bi
}

func TestVersionString(t *testing.T) {
	cases := []struct {
		name    string
		stamped string
		bi      *debug.BuildInfo
		ok      bool
		want    string
	}{
		{
			name:    "a linker-stamped version wins",
			stamped: "v2.0.0",
			bi:      buildInfo("v1.0.0", nil),
			ok:      true,
			want:    "v2.0.0",
		},
		{
			name: "a module version is used when installed",
			bi:   buildInfo("v1.0.0", nil),
			ok:   true,
			want: "v1.0.0",
		},
		{
			name: "a working-tree build reports its revision, not the pseudo-version",
			bi: buildInfo("v0.0.0-20260830064323-6b4db23794f0+dirty", map[string]string{
				"vcs.revision": "6b4db23794f0aaaa",
				"vcs.modified": "true",
			}),
			ok:   true,
			want: "devel (6b4db23, modified)",
		},
		{
			name: "a local build reports its revision",
			bi:   buildInfo("(devel)", map[string]string{"vcs.revision": "0123456789abcdef"}),
			ok:   true,
			want: "devel (0123456)",
		},
		{
			name: "an uncommitted local build says so",
			bi: buildInfo("(devel)", map[string]string{
				"vcs.revision": "0123456789abcdef",
				"vcs.modified": "true",
			}),
			ok:   true,
			want: "devel (0123456, modified)",
		},
		{
			name: "a build with nothing to go on",
			bi:   buildInfo("(devel)", nil),
			ok:   true,
			want: "devel",
		},
		{
			name: "no build info at all",
			ok:   false,
			want: "unknown",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := versionString(c.stamped, c.bi, c.ok); got != c.want {
				t.Errorf("= %q, want %q", got, c.want)
			}
		})
	}
}

func TestWantsVersion(t *testing.T) {
	if !wantsVersion([]string{"--version"}) {
		t.Error("--version should be recognised")
	}
	if !wantsVersion([]string{"ls", "--version"}) {
		t.Error("--version should be recognised after a subcommand, like -h")
	}
	if wantsVersion([]string{"ls"}) || wantsVersion(nil) {
		t.Error("nothing else should be taken as --version")
	}
	if wantsVersion([]string{"--", "--version"}) {
		t.Error("after -- it is an argument, not a flag")
	}
}
