package cli

import (
	"bytes"
	"testing"
	"time"

	"todo.mirumo.net/internal/store"
)

func refTime() time.Time { return time.Date(2026, 8, 29, 15, 0, 0, 0, time.Local) }

// newApp builds a fully isolated App: in-memory database, buffered output, fixed clock, temp dir as cwd.
func newApp(t *testing.T) (*App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	st, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite：%v", err)
	}
	t.Cleanup(func() { st.Close() })
	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	app := &App{
		Store: st, Out: out, Err: errBuf,
		Now: refTime, Cwd: t.TempDir(), Color: false,
	}
	return app, out, errBuf
}
