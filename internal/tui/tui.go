// Package tui 提供 todo 的互動介面。
package tui

import (
	"errors"
	"time"

	"todo.mirumo.net/internal/store"
)

// Run 啟動互動介面。
func Run(s store.Store, now func() time.Time, cwd string) error {
	return errors.New("TUI 尚未實作")
}
