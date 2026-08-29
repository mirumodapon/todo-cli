package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"todo.mirumo.net/internal/datearg"
	"todo.mirumo.net/internal/task"
)

const dateLayout = "2006-01-02"

const schema = `
CREATE TABLE IF NOT EXISTS tasks (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  title      TEXT    NOT NULL,
  project    TEXT    NOT NULL DEFAULT '',
  due        TEXT    NULL,
  priority   INTEGER NOT NULL DEFAULT 0,
  done_at    TEXT    NULL,
  created_at TEXT    NOT NULL,
  updated_at TEXT    NOT NULL
);
CREATE TABLE IF NOT EXISTS tags (
  id   INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE
);
CREATE TABLE IF NOT EXISTS task_tags (
  task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  tag_id  INTEGER NOT NULL REFERENCES tags(id)  ON DELETE CASCADE,
  PRIMARY KEY (task_id, tag_id)
);
CREATE INDEX IF NOT EXISTS idx_tasks_done ON tasks(done_at);
CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project);
`

const taskCols = `id, title, project, due, priority, done_at, created_at, updated_at`

type sqlStore struct{ db *sql.DB }

// OpenSQLite 開啟（必要時建立）資料庫。path 可以是檔案路徑或 ":memory:"。
func OpenSQLite(path string) (Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// 單一使用者的 CLI 不需要連線池；限制為一條連線，
	// PRAGMA 才會確實套用在之後所有查詢上（SQLite 預設關閉外鍵）。
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &sqlStore{db: db}, nil
}

func (s *sqlStore) Close() error { return s.db.Close() }

func dueVal(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format(dateLayout)
}

func tsVal(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339)
}

func parseNull(ns sql.NullString, layout string) (*time.Time, error) {
	if !ns.Valid {
		return nil, nil
	}
	t, err := time.ParseInLocation(layout, ns.String, time.Local)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

type scanner interface{ Scan(dest ...any) error }

func scanTask(sc scanner) (task.Task, error) {
	var (
		t                task.Task
		due, doneAt      sql.NullString
		created, updated string
		pri              int
	)
	if err := sc.Scan(&t.ID, &t.Title, &t.Project, &due, &pri, &doneAt, &created, &updated); err != nil {
		return task.Task{}, err
	}
	t.Priority = task.Priority(pri)
	var err error
	if t.Due, err = parseNull(due, dateLayout); err != nil {
		return task.Task{}, err
	}
	if t.DoneAt, err = parseNull(doneAt, time.RFC3339); err != nil {
		return task.Task{}, err
	}
	if t.CreatedAt, err = time.ParseInLocation(time.RFC3339, created, time.Local); err != nil {
		return task.Task{}, err
	}
	if t.UpdatedAt, err = time.ParseInLocation(time.RFC3339, updated, time.Local); err != nil {
		return task.Task{}, err
	}
	return t, nil
}

// setTags 覆寫一筆任務的標籤關聯。
func (s *sqlStore) setTags(id int64, tags []string) error {
	if _, err := s.db.Exec(`DELETE FROM task_tags WHERE task_id = ?`, id); err != nil {
		return err
	}
	for _, name := range task.NormalizeTags(tags) {
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO tags (name) VALUES (?)`, name); err != nil {
			return err
		}
		if _, err := s.db.Exec(
			`INSERT OR IGNORE INTO task_tags (task_id, tag_id) SELECT ?, id FROM tags WHERE name = ?`,
			id, name); err != nil {
			return err
		}
	}
	return nil
}

func (s *sqlStore) loadTags(id int64) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT g.name FROM task_tags tt JOIN tags g ON g.id = tt.tag_id WHERE tt.task_id = ? ORDER BY g.name`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func (s *sqlStore) Add(t task.Task) (task.Task, error) {
	res, err := s.db.Exec(
		`INSERT INTO tasks (title, project, due, priority, done_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.Title, t.Project, dueVal(t.Due), int(t.Priority), tsVal(t.DoneAt),
		t.CreatedAt.Format(time.RFC3339), t.UpdatedAt.Format(time.RFC3339))
	if err != nil {
		return task.Task{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return task.Task{}, err
	}
	t.ID = id
	t.Tags = task.NormalizeTags(t.Tags)
	if err := s.setTags(id, t.Tags); err != nil {
		return task.Task{}, err
	}
	return t, nil
}

func (s *sqlStore) Get(id int64) (task.Task, error) {
	row := s.db.QueryRow(`SELECT `+taskCols+` FROM tasks WHERE id = ?`, id)
	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return task.Task{}, ErrNotFound
	}
	if err != nil {
		return task.Task{}, err
	}
	if t.Tags, err = s.loadTags(id); err != nil {
		return task.Task{}, err
	}
	return t, nil
}

func (s *sqlStore) List(f task.Filter, now time.Time) ([]task.Task, error) {
	var where []string
	var args []any

	switch {
	case f.OnlyDone:
		where = append(where, `done_at IS NOT NULL`)
	case !f.IncludeDone:
		where = append(where, `done_at IS NULL`)
	}
	if f.Project != nil {
		where = append(where, `project = ?`)
		args = append(args, *f.Project)
	}
	if f.Priority != nil {
		where = append(where, `priority = ?`)
		args = append(args, int(*f.Priority))
	}
	if f.Search != "" {
		where = append(where, `LOWER(title) LIKE ?`)
		args = append(args, "%"+strings.ToLower(f.Search)+"%")
	}
	today := datearg.Day(now).Format(dateLayout)
	switch f.DueRange {
	case task.DueToday:
		where = append(where, `due = ?`)
		args = append(args, today)
	case task.DueOverdue:
		where = append(where, `due IS NOT NULL AND due < ?`)
		args = append(args, today)
	case task.DueWeek:
		where = append(where, `due IS NOT NULL AND due <= ?`)
		args = append(args, datearg.Day(now).AddDate(0, 0, 7).Format(dateLayout))
	case task.DueOn:
		where = append(where, `due = ?`)
		args = append(args, datearg.Day(f.DueOn).Format(dateLayout))
	}
	if tags := task.NormalizeTags(f.Tags); len(tags) > 0 {
		ph := strings.TrimSuffix(strings.Repeat("?,", len(tags)), ",")
		where = append(where, fmt.Sprintf(
			`(SELECT COUNT(DISTINCT g.name) FROM task_tags tt JOIN tags g ON g.id = tt.tag_id
			  WHERE tt.task_id = tasks.id AND g.name IN (%s)) = ?`, ph))
		for _, tg := range tags {
			args = append(args, tg)
		}
		args = append(args, len(tags))
	}

	// 無期限者一律排在有期限者之後：SQLite 中 (due IS NULL) 為 0/1，升冪即可。
	order := `(due IS NULL), due ASC, priority DESC, id ASC`
	switch f.Sort {
	case task.SortPriority:
		order = `priority DESC, (due IS NULL), due ASC, id ASC`
	case task.SortCreated:
		order = `created_at ASC, id ASC`
	}

	q := `SELECT ` + taskCols + ` FROM tasks`
	if len(where) > 0 {
		q += ` WHERE ` + strings.Join(where, ` AND `)
	}
	q += ` ORDER BY ` + order

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	var out []task.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	// 清單規模是個人待辦，逐筆載入標籤的 N+1 成本可忽略，換來的是簡單。
	for i := range out {
		tags, err := s.loadTags(out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Tags = tags
	}
	return out, nil
}

func (s *sqlStore) Update(t task.Task) error { return errors.New("尚未實作") }

func (s *sqlStore) Delete(id int64) error {
	res, err := s.db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *sqlStore) SetDone(id int64, done bool, now time.Time) error {
	return errors.New("尚未實作")
}

func (s *sqlStore) Restore(t task.Task) error         { return errors.New("尚未實作") }
func (s *sqlStore) Tags() ([]string, error)           { return nil, errors.New("尚未實作") }
func (s *sqlStore) Projects() ([]ProjectCount, error) { return nil, errors.New("尚未實作") }
