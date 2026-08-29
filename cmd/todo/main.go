// Command todo 是一個本機待辦事項工具。
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"todo.mirumo.net/internal/cli"
	"todo.mirumo.net/internal/store"
	"todo.mirumo.net/internal/tui"
)

func main() { os.Exit(run()) }

// run 把整個流程包起來，讓 defer 有機會執行（os.Exit 會跳過 defer）。
func run() int {
	dbFlag, args, err := cli.SplitGlobal(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "錯誤：%s\n", err)
		return 2
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "錯誤：找不到家目錄：%s\n", err)
		return 1
	}
	dbPath := resolveDBPath(os.Getenv("TODO_DB"), dbFlag, home)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "錯誤：無法建立資料目錄 %s：%s\n", filepath.Dir(dbPath), err)
		return 1
	}
	st, err := store.OpenSQLite(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "錯誤：無法開啟資料庫 %s：%s\n", dbPath, err)
		return 1
	}
	defer st.Close()

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	app := &cli.App{
		Store: st,
		Out:   os.Stdout,
		Err:   os.Stderr,
		Now:   time.Now,
		Cwd:   cwd,
		Color: isTTY(os.Stdout),
	}
	app.RunTUI = func() error { return tui.Run(st, app.Now, cwd) }
	return app.Run(args)
}

// resolveDBPath 決定資料庫位置：--db 優先於 TODO_DB，都沒有就用 ~/.todo/todo.db。
func resolveDBPath(envDB, flagDB, home string) string {
	if flagDB != "" {
		return flagDB
	}
	if envDB != "" {
		return envDB
	}
	return filepath.Join(home, ".todo", "todo.db")
}

// isTTY 判斷輸出是不是終端機；被導向檔案或管線時要關掉顏色。
func isTTY(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
