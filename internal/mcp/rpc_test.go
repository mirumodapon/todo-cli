package mcp

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"todo.mirumo.net/internal/store"
)

func ref() time.Time { return time.Date(2026, 8, 29, 15, 0, 0, 0, time.Local) }

// serve runs the server over a canned session and returns one decoded response
// per line it wrote.
func serve(t *testing.T, lines ...string) []map[string]any {
	t.Helper()
	s, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return serveWith(t, s, lines...)
}

func serveWith(t *testing.T, s store.Store, lines ...string) []map[string]any {
	t.Helper()
	var out strings.Builder
	srv := &Server{Store: s, Now: ref, Version: "test"}
	if err := srv.Serve(strings.NewReader(strings.Join(lines, "\n")+"\n"), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	var got []map[string]any
	for _, l := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("the server wrote something that is not JSON: %q", l)
		}
		got = append(got, m)
	}
	return got
}

// errorOf returns the error object of a response, or nil.
func errorOf(m map[string]any) map[string]any {
	e, _ := m["error"].(map[string]any)
	return e
}

func TestInitializeAnswersWithCapabilities(t *testing.T) {
	got := serve(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`)
	if len(got) != 1 {
		t.Fatalf("want one response, got %d", len(got))
	}
	r := got[0]
	if r["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc = %v", r["jsonrpc"])
	}
	res, ok := r["result"].(map[string]any)
	if !ok {
		t.Fatalf("result = %v", r["result"])
	}
	if res["protocolVersion"] != "2025-06-18" {
		t.Errorf("protocolVersion = %v, a version we speak should be echoed back", res["protocolVersion"])
	}
	caps, _ := res["capabilities"].(map[string]any)
	for _, want := range []string{"tools", "resources", "prompts"} {
		if _, ok := caps[want]; !ok {
			t.Errorf("capabilities is missing %q: %v", want, caps)
		}
	}
	info, _ := res["serverInfo"].(map[string]any)
	if info["name"] != "todo" {
		t.Errorf("serverInfo = %v", info)
	}
}

// A version we do not know is answered with one we do, which is what the client
// needs in order to decide whether to carry on.
func TestInitializeFallsBackOnAnUnknownVersion(t *testing.T) {
	got := serve(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"1999-01-01"}}`)
	res := got[0]["result"].(map[string]any)
	if res["protocolVersion"] != latestVersion {
		t.Errorf("protocolVersion = %v, want the latest we support", res["protocolVersion"])
	}
}

func TestNotificationsGetNoReply(t *testing.T) {
	got := serve(t,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`)
	if len(got) != 1 {
		t.Fatalf("a notification must not be answered, got %d responses: %v", len(got), got)
	}
	if got[0]["id"].(float64) != 2 {
		t.Errorf("the one reply should be to the ping, got %v", got[0])
	}
}

func TestUnknownMethodIsAnError(t *testing.T) {
	got := serve(t, `{"jsonrpc":"2.0","id":1,"method":"frobnicate"}`)
	e := errorOf(got[0])
	if e == nil || e["code"].(float64) != -32601 {
		t.Fatalf("want a method-not-found error, got %v", got[0])
	}
	if !strings.Contains(e["message"].(string), "frobnicate") {
		t.Errorf("the message should name the method: %v", e["message"])
	}
}

func TestBrokenJSONIsAParseError(t *testing.T) {
	got := serve(t, `{"jsonrpc":"2.0","id":1,`)
	e := errorOf(got[0])
	if e == nil || e["code"].(float64) != -32700 {
		t.Fatalf("want a parse error, got %v", got[0])
	}
	if got[0]["id"] != nil {
		t.Errorf("an unparsable request has no id to answer with, got %v", got[0]["id"])
	}
}

// Batches were dropped from the protocol; saying so is better than half-doing them.
func TestBatchesAreRejected(t *testing.T) {
	got := serve(t, `[{"jsonrpc":"2.0","id":1,"method":"ping"}]`)
	e := errorOf(got[0])
	if e == nil || e["code"].(float64) != -32600 {
		t.Fatalf("want an invalid-request error, got %v", got[0])
	}
}

// One bad line must not end the session.
func TestTheSessionSurvivesABadRequest(t *testing.T) {
	got := serve(t,
		`not json at all`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`)
	if len(got) != 2 {
		t.Fatalf("want two responses, got %d: %v", len(got), got)
	}
	if errorOf(got[1]) != nil {
		t.Errorf("the second request should have been answered normally: %v", got[1])
	}
}

// Nothing but JSON-RPC may reach stdout, so responses are single lines.
func TestEveryResponseIsOneLine(t *testing.T) {
	var out strings.Builder
	s, _ := store.OpenSQLite(":memory:")
	defer s.Close()
	srv := &Server{Store: s, Now: ref}
	if err := srv.Serve(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`+"\n"), &out); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(strings.TrimRight(out.String(), "\n"), "\n"); n != 0 {
		t.Errorf("a response spanned %d newlines:\n%s", n+1, out.String())
	}
}
