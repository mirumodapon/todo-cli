# todo：本機待辦事項 CLI + TUI 設計

日期：2026-08-29
狀態：已核可，待寫實作計畫

## 目標

一個純本機的待辦事項工具，以 CLI 為主要介面、TUI 為瀏覽與批次操作模式。資料存在使用者家目錄，不經網路。

## 非目標

- 任何形式的同步、伺服器、多裝置（`todo.mirumo.net` 只是目錄名，不是服務）
- 重複性任務、子任務、提醒通知
- shell 補全（之後要的話另外補靜態腳本）

## 技術選型

- Go 1.26
- module path：`todo.mirumo.net`，binary 名 `todo`
- TUI：Bubble Tea + Lip Gloss
- 儲存：SQLite，driver 用 `modernc.org/sqlite`（純 Go，免 cgo）
- 參數解析：**自寫**（見下）。不使用 Cobra / pflag / 標準庫 `flag`

外部依賴僅 Bubble Tea 系列與 SQLite driver。

## 架構分層

依賴方向由外向內，內層零 IO。

| 套件 | 職責 | 依賴 |
|---|---|---|
| `internal/argparse` | GNU 風格參數解析，支援可選值 flag | 無 |
| `internal/task` | 領域型別 `Task`、`Priority`、`Filter`、`SortBy`；欄位驗證 | 無 |
| `internal/datearg` | 日期字串解析與人類化顯示 | 標準庫 |
| `internal/project` | 由當前目錄推導專案路徑 | 標準庫 |
| `internal/store` | `Store` 介面 + SQLite 實作 + schema migration | `task` |
| `internal/cli` | 子指令 dispatch、flag 定義、輸出格式化 | `task` `store` `datearg` `argparse` `project` |
| `internal/tui` | Bubble Tea model / update / view | `task` `store` `datearg` `project` |
| `cmd/todo` | 組裝：開 DB、建 store、dispatch | 全部 |

`Store` 是介面（`Add / Get / List(Filter) / Update / Delete / SetDone / Tags / Projects`）。CLI 與 TUI 只認介面，測試時換成 `:memory:` SQLite 或假實作，永不碰使用者真實資料。

## 資料

### 位置

`~/.todo/todo.db`。目錄不存在時首次執行自動建立，權限 0700。可用 `--db <path>` flag 或 `TODO_DB` 環境變數覆寫；測試靠此指向暫存目錄。不採用 XDG。

### Schema

```sql
CREATE TABLE tasks (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  title      TEXT    NOT NULL,
  project    TEXT    NOT NULL DEFAULT '',
  due        TEXT    NULL,                    -- YYYY-MM-DD，無時間部分
  priority   INTEGER NOT NULL DEFAULT 0,      -- 0 none / 1 low / 2 med / 3 high
  done_at    TEXT    NULL,                    -- NULL = 未完成，兼作完成時間
  created_at TEXT    NOT NULL,
  updated_at TEXT    NOT NULL
);
CREATE TABLE tags (
  id   INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE
);
CREATE TABLE task_tags (
  task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  tag_id  INTEGER NOT NULL REFERENCES tags(id)  ON DELETE CASCADE,
  PRIMARY KEY (task_id, tag_id)
);
```

決定與理由：

- `done_at` 用 nullable 時間戳而非 bool，一個欄位同時表達「完成沒」與「何時完成」
- `priority` 存整數，可直接 `ORDER BY`
- 標籤正規化成 join table，才能便宜地列出所有標籤
- `project` 只是字串欄位，不另開表；一個任務單一歸屬，`SELECT DISTINCT project` 足夠
- id 用 `AUTOINCREMENT`，不重用已刪除的號碼，使用者直接打 `todo done 3`
- 每次開啟連線需執行 `PRAGMA foreign_keys = ON`（SQLite 預設關閉，否則 CASCADE 不生效）

### 專案欄位語意

空字串是一等狀態，代表「全域未分類」，不是缺漏。`todo add "買牛奶"` 就是一條全域待辦。清單中未分類項目不顯示專案欄，不印 `(none)` 之類佔位字。

`-p` 無值時由 `internal/project` 推導：自當前目錄往上找 `.git`，找到就用 repo 根，找不到則用 pwd。存絕對路徑（目錄名會撞，如兩個 repo 都有 `docs/`），顯示時只印 basename，完整路徑留給 `todo projects`。

## 參數解析（`internal/argparse`）

不用現成庫的原因：`-p` 要同時支援「無值 = 當前目錄」與「有值 = 指定專案名」。pflag 與標準庫 `flag` 都不支援可選值 flag，因為 `-p work` 在通則上無法與「`-p` 後接位置參數」區分。

支援語法：

```
--project work    --project=work    -p work    -p=work    -p
--all             -a                --            (結束 flag 解析)
```

Flag 三類：

- **bool**：永不吃下一個 token
- **string / string 可重複**：必須有值，缺值報錯
- **可選值 string**：下一個 token 存在且不以 `-` 開頭就吃掉，否則視為「有給但無值」

解析結果須能分辨三態——沒給 / 給了但無值 / 給了值。`edit` 的「不動它 vs 清空」與 `-p` 的「當前目錄 vs 指定名稱」都依賴這個。

不支援短 flag 疊寫（`-at`）。未知 flag 報錯並列出該子指令的合法 flag。

**已知歧義**：`todo add -p "買牛奶"` 會把標題吃成專案名。緩解：`add` 只收一個位置參數，解析後若位置參數為空而 `-p` 剛好吞了值，就報錯並提示「標題請放最前面，或用 `--project=買牛奶`」。慣用寫法是 `todo add "買牛奶" -p`。

## CLI 介面

```
todo                          印 usage
todo tui                      進 TUI
todo add <title> [-p [name]] [-t tag]... [-d due] [--pri low|med|high]
todo ls   [-p [name]] [--no-project] [-t tag] [-d today|week|overdue|<date>]
          [--pri p] [-a] [--done] [-s due|pri|created]
todo done   <id>...
todo undone <id>...
todo rm     <id>...
todo edit <id> [同 add 的 flag]
todo projects                 列出所有專案與各自未完成數
todo tags                     列出所有標籤
```

全域 flag：`--db <path>`、`-h/--help`。

`ls` 預設只列未完成，`-a` 含已完成，`--done` 只看已完成。預設排序 `due`，無期限者排最後。

`edit` 用「有沒有給這個 flag」決定是否改動：`--due ""` 清掉期限，不給 `--due` 則不動。

`ls` 的 `-t` 可重複，多個標籤取交集（AND）。`-p` 與 `--no-project` 同時出現時報錯，不猜使用者意圖。

輸出為對齊的純文字欄位。偵測到 stdout 不是 TTY（管線、重導向）時關閉顏色。

`add` 的 flag 值即時驗證：`--pri` 只收三個字串、`--due` 走 `datearg`、`--tag` 去重。錯誤訊息指出是哪個 flag 有問題。

## 日期解析（`internal/datearg`）

輸入接受：`today`、`tomorrow`、`yesterday`、星期簡稱（`mon`..`sun`，指向未來最近的該天）、相對量（`+3d`、`+2w`）、絕對日期 `YYYY-MM-DD`。

顯示端輸出人類化字串：`今天`、`明天`、`週五`、`逾期 2 天`、`09-01`。

`ls -d` 另接受區間關鍵字 `today` / `week` / `overdue`。

## TUI

根 model 持有 mode，`Update` 依 mode 分派：

```
list（預設）→ search（/ 增量搜尋）
            → form（a 新增 / e 編輯）
            → picker（P 選專案 / T 選標籤）
            → help（? 覆蓋層）
```

### list 模式鍵位

| 鍵 | 行為 |
|---|---|
| `j` `k` `↑` `↓` `g` `G` | 移動 |
| `space` | 切換完成 / 未完成 |
| `a` / `e` | 新增 / 編輯（開表單） |
| `d` | 刪除，底部顯示「已刪除『…』· u 復原」 |
| `u` | 復原最近一次刪除 |
| `/` | 搜尋標題，打字即過濾；`esc` 取消、`enter` 定住 |
| `P` `T` | 依專案 / 標籤過濾，彈選單，含「未分類」一項 |
| `A` | 切換是否顯示已完成 |
| `s` | 循環排序 due → pri → created |
| `esc` | 清除所有過濾條件 |
| `?` | 說明覆蓋層 |
| `q` | 離開 |

刪除採 undo 而非確認對話框：確認框對每次刪除都收費，undo 只在真的刪錯時才付出成本。undo 只保留一層（最近一次刪除的完整 Task 含標籤），離開 TUI 即失效。復原時以原 id 重新插入——`AUTOINCREMENT` 不重用號碼，該 id 必定仍空著。

### 表單

`a` 與 `e` 共用。五欄：標題 / 專案 / 標籤 / 截止日 / 優先度。`tab`、`shift+tab` 切換欄位，`enter` 儲存，`esc` 取消。截止日欄用 `datearg` 解析，不合法時欄位標紅且不允許儲存。專案欄留空即全域未分類，另有一鍵填入當前目錄專案（與 CLI `-p` 共用 `internal/project`）。

### 資料流

所有 store 操作走 `tea.Cmd`，結果以 msg 回到 `Update`。`Update` 不直接碰 DB，維持純函式。每次變更後發一次重查指令刷新清單，確保畫面與 DB 一致；清單規模小，重查成本可忽略。

## 錯誤處理

- TUI：store 錯誤包成 `errMsg`，於底部狀態列以紅字顯示，不崩潰、不中斷
- CLI：訊息寫 stderr，離開碼非 0
- 找不到 id：明確指出是哪個 id，不靜默略過
- 刪除任務後，`tags` 表可能留下沒有任何任務引用的標籤。不做清理（無害），但 `todo tags` 與 TUI 的標籤選單只列出至少被一個任務引用的標籤
- DB 檔損毀或無法開啟：訊息含路徑與底層錯誤

## 測試策略

| 層 | 方式 |
|---|---|
| `argparse` `task` `datearg` `project` | 表格測試，純函式 |
| `store` | 對 `:memory:` SQLite 跑真實 SQL |
| `cli` | 假 Store，驗證 flag → store 呼叫 → 輸出字串 |
| `tui` | 餵 msg 序列給 `Update`，斷言 model 狀態轉移 |

任何測試都不得碰 `~/.todo`；需要真實檔案時用 `t.TempDir()`。

## 專案結構

```
cmd/todo/main.go
internal/argparse/
internal/task/
internal/datearg/
internal/project/
internal/store/
internal/cli/
internal/tui/
docs/superpowers/specs/
```
