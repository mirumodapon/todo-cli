// Package project derives a task's project from a filesystem location.
package project

import (
	"os"
	"path/filepath"
)

// Current walks up from dir looking for .git, returning that directory when found and dir otherwise.
// It always returns an absolute path: directory names collide (two repos can both have docs/) but paths do not.
func Current(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for d := abs; ; {
		// In a git worktree .git is a file; in a normal repo it is a directory. Both count.
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return d, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return abs, nil
}

// Label returns the short display name. An empty path means uncategorised and is returned as is.
func Label(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Base(path)
}
