// Package project 由檔案系統位置推導待辦事項所屬的專案。
package project

import (
	"os"
	"path/filepath"
)

// Current 從 dir 往上找 .git；找到就回傳該目錄，找不到則回傳 dir 本身。
// 一律回傳絕對路徑——目錄名會撞（兩個 repo 都可能有 docs/），路徑才唯一。
func Current(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for d := abs; ; {
		// git worktree 的 .git 是檔案，一般 repo 是目錄，兩種都算。
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

// Label 回傳顯示用的短名。空字串代表全域未分類，原樣回傳。
func Label(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Base(path)
}
