package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCurrentFindsGitRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "internal", "store")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Current(sub)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(root)
	gotResolved, _ := filepath.EvalSymlinks(got)
	if gotResolved != want {
		t.Errorf("= %q，預期 repo 根 %q", gotResolved, want)
	}
}

func TestCurrentFallsBackToDir(t *testing.T) {
	dir := t.TempDir()
	got, err := Current(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("= %q，預期絕對路徑", got)
	}
	gotResolved, _ := filepath.EvalSymlinks(got)
	want, _ := filepath.EvalSymlinks(dir)
	if gotResolved != want {
		t.Errorf("= %q，沒有 .git 時應回傳目錄本身 %q", gotResolved, want)
	}
}

func TestCurrentAcceptsGitFile(t *testing.T) {
	// In a git worktree .git is a file, not a directory.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: /elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "a")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got, _ := Current(sub)
	gotResolved, _ := filepath.EvalSymlinks(got)
	want, _ := filepath.EvalSymlinks(root)
	if gotResolved != want {
		t.Errorf("= %q，.git 是檔案時仍應視為 repo 根 %q", gotResolved, want)
	}
}

func TestLabel(t *testing.T) {
	if got := Label("/Users/me/Projects/todo.mirumo.net"); got != "todo.mirumo.net" {
		t.Errorf("= %q，預期 basename", got)
	}
	if got := Label(""); got != "" {
		t.Errorf("= %q，空字串應原樣回傳（全域未分類）", got)
	}
}
