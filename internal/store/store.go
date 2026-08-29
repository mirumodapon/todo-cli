// Package store 負責待辦事項的持久化。
package store

import (
	"errors"
	"time"

	"todo.mirumo.net/internal/task"
)

// ErrNotFound 表示指定 id 的任務不存在。
var ErrNotFound = errors.New("找不到該任務")

// ProjectCount 是一個專案與它的未完成數。
type ProjectCount struct {
	Path string
	Open int
}

// Store 是待辦事項的儲存介面。CLI 與 TUI 只認這個介面，
// 測試時換成 :memory: 的實作，永不碰使用者的真實資料。
type Store interface {
	// Add 新增一筆並回傳含 ID 的結果。
	Add(t task.Task) (task.Task, error)
	// Get 依 id 取一筆，不存在時回傳 ErrNotFound。
	Get(id int64) (task.Task, error)
	// List 依 f 查詢；now 用於解析 today/week/overdue 這類相對條件。
	List(f task.Filter, now time.Time) ([]task.Task, error)
	// Update 依 t.ID 覆寫全部欄位（含標籤）。
	Update(t task.Task) error
	// Delete 刪除一筆，連帶清掉它的標籤關聯。
	Delete(id int64) error
	// SetDone 設定或取消完成狀態。
	SetDone(id int64, done bool, now time.Time) error
	// Restore 以 t.ID 原號重新插入，供 TUI 的刪除復原使用。
	Restore(t task.Task) error
	// Tags 列出至少被一個任務引用的標籤。
	Tags() ([]string, error)
	// Projects 列出所有專案與各自的未完成數。
	Projects() ([]ProjectCount, error)
	Close() error
}
