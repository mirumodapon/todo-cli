// Package editor hands a piece of text to the user's editor and reads back
// what they left behind, the way git does for a commit message.
package editor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Command builds the command that opens path for editing. VISUAL wins over
// EDITOR, and vi is the fallback: a machine without vi is rarer than a user who
// never set either variable.
//
// The editor runs through a shell so EDITOR may carry arguments ("code -w"),
// and the file is passed as a positional argument so that a path with spaces
// in it needs no quoting of ours.
func Command(path string) *exec.Cmd {
	ed := os.Getenv("VISUAL")
	if strings.TrimSpace(ed) == "" {
		ed = os.Getenv("EDITOR")
	}
	if strings.TrimSpace(ed) == "" {
		ed = "vi"
	}
	return exec.Command("sh", "-c", ed+` "$@"`, "sh", path)
}

// WriteTemp puts content in a fresh temporary file for name. The .md suffix is
// for the editor's benefit: descriptions are prose, and most editors soft-wrap
// and highlight markdown without being asked.
func WriteTemp(name, content string) (string, error) {
	dir, err := os.MkdirTemp("", "todo-")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, name+".md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	return path, nil
}

// ReadTemp reads an edited file and removes it. The trailing newline an editor
// leaves is the file format rather than part of the text, so it is trimmed;
// nothing else is. In particular no line is treated as a comment, because a
// description may legitimately start with #.
func ReadTemp(path string) (string, error) {
	b, err := os.ReadFile(path)
	os.RemoveAll(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(b), " \t\r\n"), nil
}

// Edit runs the whole cycle. An editor that exits non-zero aborts the edit
// rather than saving what happens to be in the file, as git does.
func Edit(name, content string) (string, error) {
	path, err := WriteTemp(name, content)
	if err != nil {
		return "", err
	}
	c := Command(path)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		os.RemoveAll(filepath.Dir(path))
		return "", fmt.Errorf("editor: %w", err)
	}
	return ReadTemp(path)
}
