package cli

import (
	"errors"
	"fmt"
	"strings"

	"todo.mirumo.net/internal/argparse"
	"todo.mirumo.net/internal/datearg"
	"todo.mirumo.net/internal/task"
)

// addFlags 是 add 與 edit 共用的欄位 flag。
func addFlags() *argparse.Set {
	return argparse.New(
		argparse.Spec{Long: "project", Short: "p", Kind: argparse.OptionalString, Usage: "Project; uses the current directory when given no value"},
		argparse.Spec{Long: "tag", Short: "t", Kind: argparse.StringSlice, Usage: "Tag; repeatable"},
		argparse.Spec{Long: "due", Short: "d", Kind: argparse.String, Usage: "Due date: tomorrow, fri, +3d, 2026-09-01"},
		argparse.Spec{Long: "pri", Kind: argparse.String, Usage: "Priority: low, med, high"},
	)
}

func (a *App) cmdAdd(args []string) error {
	r, err := addFlags().Parse(args)
	if err != nil {
		return err
	}
	pos := r.Args()
	if len(pos) == 0 {
		// 最常見的誤用：todo add -p "買牛奶"，標題被 -p 吃掉了。
		if v, has := r.Optional("project"); has {
			return fmt.Errorf("缺少標題；看起來 %q 被當成 --project 的值。標題請放在 flag 前面（todo add %q -p），或用 --project=%s", v, v, v)
		}
		return errors.New("用法：todo add <標題> [flags]")
	}
	if len(pos) > 1 {
		return fmt.Errorf("只能有一個標題，收到 %d 個位置參數；含空白的標題請用引號包起來", len(pos))
	}
	title, err := task.ValidateTitle(pos[0])
	if err != nil {
		return err
	}

	now := a.Now()
	t := task.Task{
		Title:     title,
		Tags:      task.NormalizeTags(r.Strings("tag")),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if p, ok, err := a.resolveProject(r); err != nil {
		return err
	} else if ok {
		t.Project = p
	}
	if r.Changed("due") && strings.TrimSpace(r.String("due")) != "" {
		d, err := datearg.Parse(r.String("due"), now)
		if err != nil {
			return err
		}
		t.Due = &d
	}
	if t.Priority, err = task.ParsePriority(r.String("pri")); err != nil {
		return err
	}

	got, err := a.Store.Add(t)
	if err != nil {
		return err
	}
	fmt.Fprintf(a.Out, "已新增 #%d：%s\n", got.ID, got.Title)
	return nil
}
