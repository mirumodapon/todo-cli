// Package cli 實作 todo 的命令列介面。
package cli

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"todo.mirumo.net/internal/argparse"
	"todo.mirumo.net/internal/project"
	"todo.mirumo.net/internal/store"
)

// App 持有一次執行所需的全部依賴。全部可注入，測試才能完全隔離。
type App struct {
	Store store.Store
	Out   io.Writer
	Err   io.Writer
	Now   func() time.Time
	Cwd   string
	Color bool
	// RunTUI 由 cmd/todo 注入。cli 不 import tui，兩者保持平行。
	RunTUI func() error
}

const usageText = `todo — 本機待辦事項

用法：
  todo <指令> [flags]

指令：
  add <標題>      新增一條待辦
  ls              列出待辦
  done <id>...    標記完成
  undone <id>...  取消完成
  edit <id>       修改欄位
  rm <id>...      刪除
  projects        列出專案與各自未完成數
  tags            列出標籤
  tui             進入互動介面

全域 flag：
  --db <路徑>     指定資料庫（預設 ~/.todo/todo.db，或環境變數 TODO_DB）
  -h, --help      顯示這份說明
`

// commands 是子指令表。每加一個子指令就在這裡登記一行。
func (a *App) commands() map[string]func([]string) error {
	return map[string]func([]string) error{
		"tui": a.cmdTUI,
	}
}

// Run 執行一次命令，回傳行程離開碼：0 成功、1 執行失敗、2 用法錯誤。
func (a *App) Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(a.Out, usageText)
		return 0
	}
	// -h 放在哪都算。子指令的 flag 集不認得 -h，
	// 不先攔下來的話 todo add -h 會變成「未知的 flag」，那是很差的體驗。
	for _, a2 := range args {
		if a2 == "-h" || a2 == "--help" {
			fmt.Fprint(a.Out, usageText)
			return 0
		}
	}
	name, rest := args[0], args[1:]
	if name == "help" {
		fmt.Fprint(a.Out, usageText)
		return 0
	}
	cmd, ok := a.commands()[name]
	if !ok {
		fmt.Fprintf(a.Err, "未知的指令：%s\n\n", name)
		fmt.Fprint(a.Err, usageText)
		return 2
	}
	if err := cmd(rest); err != nil {
		fmt.Fprintf(a.Err, "錯誤：%s\n", err)
		return 1
	}
	return 0
}

func (a *App) cmdTUI(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("todo tui 不接受參數，收到 %q", args[0])
	}
	if a.RunTUI == nil {
		return errors.New("這個組建沒有啟用 TUI")
	}
	return a.RunTUI()
}

// SplitGlobal 取出開頭連續的 --db，回傳剩下的參數。
// 只掃描開頭：--db 是全域 flag，位置固定才不會與子指令的 flag 混淆。
func SplitGlobal(args []string) (dbPath string, rest []string, err error) {
	for len(args) > 0 {
		switch a := args[0]; {
		case a == "--db":
			if len(args) < 2 {
				return "", nil, errors.New("flag --db 需要一個值")
			}
			dbPath, args = args[1], args[2:]
		case strings.HasPrefix(a, "--db="):
			dbPath, args = strings.TrimPrefix(a, "--db="), args[1:]
		default:
			return dbPath, args, nil
		}
	}
	return dbPath, nil, nil
}

func parseIDs(args []string) ([]int64, error) {
	if len(args) == 0 {
		return nil, errors.New("需要至少一個 id")
	}
	ids := make([]int64, 0, len(args))
	for _, a := range args {
		n, err := strconv.ParseInt(a, 10, 64)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("不是合法的 id：%q", a)
		}
		ids = append(ids, n)
	}
	return ids, nil
}

// resolveProject 判讀 -p 的三態。
// 回傳的 bool 代表「要不要動 project 欄位」，字串才是新值（可為空字串）。
func (a *App) resolveProject(r *argparse.Result) (string, bool, error) {
	if !r.Changed("project") {
		return "", false, nil
	}
	if v, has := r.Optional("project"); has {
		return v, true, nil
	}
	p, err := project.Current(a.Cwd)
	if err != nil {
		return "", false, err
	}
	return p, true, nil
}
