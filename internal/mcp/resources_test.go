package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestResourcesListNamesBoth(t *testing.T) {
	got := serve(t, `{"jsonrpc":"2.0","id":1,"method":"resources/list"}`)
	res := got[0]["result"].(map[string]any)
	list, _ := res["resources"].([]any)
	seen := map[string]map[string]any{}
	for _, x := range list {
		m := x.(map[string]any)
		seen[m["uri"].(string)] = m
	}
	for _, uri := range []string{"task://projects", "task://tags"} {
		m, ok := seen[uri]
		if !ok {
			t.Fatalf("no resource at %s, got %v", uri, seen)
		}
		if m["mimeType"] != "application/json" {
			t.Errorf("%s mimeType = %v", uri, m["mimeType"])
		}
		if d, _ := m["description"].(string); d == "" {
			t.Errorf("%s has no description", uri)
		}
	}
}

func TestReadTheProjectsResource(t *testing.T) {
	got := serveWith(t, seeded(t), `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"task://projects"}}`)
	res := got[0]["result"].(map[string]any)
	contents, _ := res["contents"].([]any)
	if len(contents) != 1 {
		t.Fatalf("contents = %v", res["contents"])
	}
	first := contents[0].(map[string]any)
	if first["uri"] != "task://projects" {
		t.Errorf("a read should say what it read: %v", first)
	}
	var out []map[string]any
	if err := json.Unmarshal([]byte(first["text"].(string)), &out); err != nil {
		t.Fatalf("the resource should be JSON: %v", err)
	}
	byLabel := map[string]map[string]any{}
	for _, m := range out {
		byLabel[m["label"].(string)] = m
	}
	if w := byLabel["work"]; w == nil || w["open"].(float64) != 1 || w["path"] != "/p/work" {
		t.Errorf("the work project should carry its path and open count: %v", out)
	}
	if u := byLabel["(uncategorized)"]; u == nil || u["open"].(float64) != 2 {
		t.Errorf("uncategorized should be named, not blank: %v", out)
	}
}

func TestReadTheTagsResource(t *testing.T) {
	got := serveWith(t, seeded(t), `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"task://tags"}}`)
	res := got[0]["result"].(map[string]any)
	first := res["contents"].([]any)[0].(map[string]any)
	var tags []string
	if err := json.Unmarshal([]byte(first["text"].(string)), &tags); err != nil {
		t.Fatal(err)
	}
	if strings.Join(tags, ",") != "shopping,urgent" {
		t.Errorf("tags = %v, want the ones in use, sorted", tags)
	}
}

func TestReadingSomethingElseIsAnError(t *testing.T) {
	got := serve(t, `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"task://nothing"}}`)
	e := errorOf(got[0])
	if e == nil || e["code"].(float64) != -32002 {
		t.Fatalf("want a resource-not-found error, got %v", got[0])
	}
	if !strings.Contains(e["message"].(string), "task://nothing") {
		t.Errorf("the message should name the uri: %v", e["message"])
	}
}
