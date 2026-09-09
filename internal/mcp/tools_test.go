package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"todo.mirumo.net/internal/store"
	"todo.mirumo.net/internal/task"
)

func seeded(t *testing.T) store.Store {
	t.Helper()
	s, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	due := ref().AddDate(0, 0, 3)
	for _, ti := range []task.Task{
		{Title: "buy milk", Tags: []string{"shopping"}, Priority: task.PriHigh, Desc: "semi-skimmed"},
		{Title: "water the plants"},
		{Title: "ship the release", Project: "/p/work", Due: &due, Tags: []string{"urgent"}},
	} {
		ti.CreatedAt, ti.UpdatedAt = ref(), ref()
		if _, err := s.Add(ti); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

// call runs one tools/call and returns the result object.
func call(t *testing.T, s store.Store, name, args string) map[string]any {
	t.Helper()
	line := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":%q,"arguments":%s}}`, name, args)
	got := serveWith(t, s, line)
	if len(got) != 1 {
		t.Fatalf("want one response, got %v", got)
	}
	if e := errorOf(got[0]); e != nil {
		t.Fatalf("%s returned a protocol error: %v", name, e)
	}
	res, ok := got[0]["result"].(map[string]any)
	if !ok {
		t.Fatalf("result = %v", got[0])
	}
	return res
}

// text pulls the text out of a tool result.
func text(t *testing.T, res map[string]any) string {
	t.Helper()
	content, _ := res["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("the result carries no content: %v", res)
	}
	first, _ := content[0].(map[string]any)
	if first["type"] != "text" {
		t.Fatalf("content = %v", content)
	}
	return first["text"].(string)
}

// decode reads a tool's text back as JSON.
func decode(t *testing.T, res map[string]any, into any) {
	t.Helper()
	if err := json.Unmarshal([]byte(text(t, res)), into); err != nil {
		t.Fatalf("the tool should answer with JSON: %v\n%s", err, text(t, res))
	}
}

func TestToolsListDescribesEveryTool(t *testing.T) {
	got := serve(t, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	res := got[0]["result"].(map[string]any)
	tools, _ := res["tools"].([]any)
	seen := map[string]map[string]any{}
	for _, x := range tools {
		m := x.(map[string]any)
		seen[m["name"].(string)] = m
	}
	for _, want := range []string{"list_tasks", "get_task", "add_task", "edit_task", "complete_task", "reopen_task", "delete_task"} {
		m, ok := seen[want]
		if !ok {
			t.Fatalf("no tool named %q, got %v", want, seen)
		}
		if d, _ := m["description"].(string); d == "" {
			t.Errorf("%s has no description", want)
		}
		schema, _ := m["inputSchema"].(map[string]any)
		if schema["type"] != "object" {
			t.Errorf("%s has no object input schema: %v", want, schema)
		}
	}
	// Annotations are how a client knows what it is about to let happen.
	if a, _ := seen["list_tasks"]["annotations"].(map[string]any); a["readOnlyHint"] != true {
		t.Errorf("list_tasks should be marked read-only: %v", seen["list_tasks"]["annotations"])
	}
	if a, _ := seen["delete_task"]["annotations"].(map[string]any); a["destructiveHint"] != true {
		t.Errorf("delete_task should be marked destructive: %v", seen["delete_task"]["annotations"])
	}
}

func TestListTasksAnswersWithJSON(t *testing.T) {
	var out []map[string]any
	decode(t, call(t, seeded(t), "list_tasks", `{}`), &out)
	if len(out) != 3 {
		t.Fatalf("want every open task, got %d: %v", len(out), out)
	}
	byTitle := map[string]map[string]any{}
	for _, m := range out {
		byTitle[m["title"].(string)] = m
	}
	milk := byTitle["buy milk"]
	if milk["id"].(float64) != 1 || milk["priority"] != "high" {
		t.Errorf("buy milk = %v", milk)
	}
	if tags, _ := milk["tags"].([]any); len(tags) != 1 || tags[0] != "shopping" {
		t.Errorf("tags = %v", milk["tags"])
	}
	// A listing stays a listing: descriptions can be long, so the list says
	// which tasks have one and get_task fetches it.
	if _, ok := milk["desc"]; ok {
		t.Errorf("list_tasks should not carry descriptions: %v", milk)
	}
	if milk["has_desc"] != true {
		t.Errorf("it should say that there is one to fetch: %v", milk)
	}
	release := byTitle["ship the release"]
	if release["due"] != "2026-09-01" || release["remaining"] != "3d" {
		t.Errorf("the due date and what is left of it should both be there: %v", release)
	}
	if release["project_label"] != "work" {
		t.Errorf("a project should come with its short name: %v", release)
	}
}

func TestListTasksFilters(t *testing.T) {
	s := seeded(t)
	cases := []struct {
		name, args string
		want       int
	}{
		{"uncategorized only", `{"project":""}`, 2},
		{"one project", `{"project":"/p/work"}`, 1},
		{"by tag", `{"tags":["urgent"]}`, 1},
		{"by search", `{"search":"milk"}`, 1},
		{"by due range", `{"due":"week"}`, 1},
		{"by priority", `{"priority":"high"}`, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out []map[string]any
			decode(t, call(t, s, "list_tasks", c.args), &out)
			if len(out) != c.want {
				t.Errorf("got %d tasks, want %d: %v", len(out), c.want, out)
			}
		})
	}
}

func TestListTasksAndDoneTasks(t *testing.T) {
	s := seeded(t)
	call(t, s, "complete_task", `{"id":1}`)
	var open, all []map[string]any
	decode(t, call(t, s, "list_tasks", `{}`), &open)
	decode(t, call(t, s, "list_tasks", `{"include_done":true}`), &all)
	if len(open) != 2 || len(all) != 3 {
		t.Errorf("open = %d, all = %d", len(open), len(all))
	}
	for _, m := range all {
		if m["id"].(float64) == 1 && m["done"] != true {
			t.Errorf("the completed task should say so: %v", m)
		}
	}
}

func TestGetTaskCarriesTheDescription(t *testing.T) {
	var got map[string]any
	decode(t, call(t, seeded(t), "get_task", `{"id":1}`), &got)
	if got["desc"] != "semi-skimmed" {
		t.Errorf("get_task should carry the description: %v", got)
	}
}

func TestAddTask(t *testing.T) {
	s := seeded(t)
	res := call(t, s, "add_task", `{"title":"renew the passport","due":"tomorrow","priority":"!!","tags":["admin"],"desc":"form DS-82"}`)
	if !strings.Contains(text(t, res), "#4") {
		t.Errorf("the answer should name the new id: %q", text(t, res))
	}
	got, err := s.Get(4)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "renew the passport" || got.Priority != task.PriMed || got.Desc != "form DS-82" {
		t.Errorf("stored %+v", got)
	}
	if got.Due == nil || got.Due.Format("2006-01-02") != "2026-08-30" {
		t.Errorf("due = %v", got.Due)
	}
}

func TestEditTaskTouchesOnlyWhatItIsGiven(t *testing.T) {
	s := seeded(t)
	call(t, s, "edit_task", `{"id":1,"priority":"low"}`)
	got, _ := s.Get(1)
	if got.Priority != task.PriLow {
		t.Errorf("priority = %v", got.Priority)
	}
	if got.Title != "buy milk" || got.Desc != "semi-skimmed" || len(got.Tags) != 1 {
		t.Errorf("an absent field must not change: %+v", got)
	}
	// An empty value is the way to clear one.
	call(t, s, "edit_task", `{"id":1,"desc":"","tags":[]}`)
	got, _ = s.Get(1)
	if got.Desc != "" || len(got.Tags) != 0 {
		t.Errorf("an empty value should clear the field: %+v", got)
	}
}

func TestCompleteAndReopen(t *testing.T) {
	s := seeded(t)
	call(t, s, "complete_task", `{"id":2}`)
	if got, _ := s.Get(2); !got.Done() {
		t.Error("complete_task should mark it done")
	}
	call(t, s, "reopen_task", `{"id":2}`)
	if got, _ := s.Get(2); got.Done() {
		t.Error("reopen_task should undo that")
	}
}

func TestDeleteTask(t *testing.T) {
	s := seeded(t)
	call(t, s, "delete_task", `{"id":2}`)
	if _, err := s.Get(2); err == nil {
		t.Error("delete_task should remove it")
	}
}

// A tool that fails answers with a result marked isError, not a protocol error:
// the model is meant to read it and try something else.
func TestToolFailuresComeBackAsResults(t *testing.T) {
	s := seeded(t)
	cases := []struct{ name, args, want string }{
		{"get_task", `{"id":99}`, "99"},
		{"add_task", `{"title":""}`, "title"},
		{"add_task", `{"title":"x","due":"someday"}`, "someday"},
		{"edit_task", `{"id":99,"title":"x"}`, "99"},
		{"complete_task", `{"id":99}`, "99"},
		{"list_tasks", `{"priority":"urgent"}`, "urgent"},
	}
	for _, c := range cases {
		t.Run(c.name+" "+c.args, func(t *testing.T) {
			res := call(t, s, c.name, c.args)
			if res["isError"] != true {
				t.Fatalf("want a tool error, got %v", res)
			}
			if !strings.Contains(text(t, res), c.want) {
				t.Errorf("the message should mention %q: %q", c.want, text(t, res))
			}
		})
	}
}

// An unknown tool is a protocol error: the client asked for something that does
// not exist, which is not something the model can fix by trying again.
func TestUnknownToolIsAProtocolError(t *testing.T) {
	got := serve(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"frobnicate","arguments":{}}}`)
	e := errorOf(got[0])
	if e == nil || e["code"].(float64) != -32602 {
		t.Fatalf("want an invalid-params error, got %v", got[0])
	}
}

func TestListTasksReverse(t *testing.T) {
	s := seeded(t)
	var forward, back []map[string]any
	decode(t, call(t, s, "list_tasks", `{}`), &forward)
	decode(t, call(t, s, "list_tasks", `{"reverse":true}`), &back)
	if len(forward) != len(back) || len(forward) == 0 {
		t.Fatalf("forward %d, back %d", len(forward), len(back))
	}
	if forward[0]["id"] != back[len(back)-1]["id"] {
		t.Errorf("reverse should turn the list around: %v vs %v", forward, back)
	}
}
