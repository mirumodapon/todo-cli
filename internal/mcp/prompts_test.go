package mcp

import (
	"strings"
	"testing"
)

// promptText joins every message of a prompts/get result.
func promptText(t *testing.T, res map[string]any) string {
	t.Helper()
	msgs, _ := res["messages"].([]any)
	if len(msgs) == 0 {
		t.Fatalf("the prompt carries no messages: %v", res)
	}
	var b strings.Builder
	for _, x := range msgs {
		m := x.(map[string]any)
		if m["role"] == nil {
			t.Errorf("a message with no role: %v", m)
		}
		c, _ := m["content"].(map[string]any)
		if c["type"] != "text" {
			t.Fatalf("content = %v", c)
		}
		b.WriteString(c["text"].(string) + "\n")
	}
	return b.String()
}

func TestPromptsListDescribesTheirArguments(t *testing.T) {
	got := serve(t, `{"jsonrpc":"2.0","id":1,"method":"prompts/list"}`)
	res := got[0]["result"].(map[string]any)
	list, _ := res["prompts"].([]any)
	seen := map[string]map[string]any{}
	for _, x := range list {
		m := x.(map[string]any)
		seen[m["name"].(string)] = m
	}
	for _, want := range []string{"plan_today", "review_project"} {
		if _, ok := seen[want]; !ok {
			t.Fatalf("no prompt named %q, got %v", want, seen)
		}
	}
	args, _ := seen["review_project"]["arguments"].([]any)
	if len(args) != 1 {
		t.Fatalf("review_project should take one argument, got %v", args)
	}
	a := args[0].(map[string]any)
	if a["name"] != "project" || a["required"] != true {
		t.Errorf("argument = %v", a)
	}
}

func TestPlanTodayCarriesTheTasks(t *testing.T) {
	s := seeded(t)
	serveWith(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"add_task","arguments":{"title":"call the dentist","due":"today"}}}`)
	got := serveWith(t, s, `{"jsonrpc":"2.0","id":1,"method":"prompts/get","params":{"name":"plan_today"}}`)
	res := got[0]["result"].(map[string]any)
	if d, _ := res["description"].(string); d == "" {
		t.Error("a prompt should describe itself")
	}
	text := promptText(t, res)
	if !strings.Contains(text, "call the dentist") {
		t.Errorf("the prompt should carry what is actually due:\n%s", text)
	}
	if !strings.Contains(text, "#4") {
		t.Errorf("it should carry ids, so the model can act on them:\n%s", text)
	}
	// The day is split up, because "overdue" and "due today" are not the same
	// news, and a task three days out is neither.
	for _, want := range []string{"Due today:", "Later this week:"} {
		if !strings.Contains(text, want) {
			t.Errorf("the prompt should have a %q section:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Overdue:") {
		t.Errorf("nothing is overdue here, so that heading should be absent:\n%s", text)
	}
}

func TestPlanTodaySaysWhenThereIsNothing(t *testing.T) {
	got := serve(t, `{"jsonrpc":"2.0","id":1,"method":"prompts/get","params":{"name":"plan_today"}}`)
	text := promptText(t, got[0]["result"].(map[string]any))
	if !strings.Contains(strings.ToLower(text), "nothing is due") {
		t.Errorf("an empty day should say so rather than showing an empty list:\n%s", text)
	}
}

func TestReviewProjectNeedsItsArgument(t *testing.T) {
	got := serve(t, `{"jsonrpc":"2.0","id":1,"method":"prompts/get","params":{"name":"review_project"}}`)
	e := errorOf(got[0])
	if e == nil || e["code"].(float64) != -32602 {
		t.Fatalf("a missing required argument should be an error, got %v", got[0])
	}
	if !strings.Contains(e["message"].(string), "project") {
		t.Errorf("the message should name the argument: %v", e["message"])
	}
}

func TestReviewProjectCarriesItsTasks(t *testing.T) {
	got := serveWith(t, seeded(t), `{"jsonrpc":"2.0","id":1,"method":"prompts/get","params":{"name":"review_project","arguments":{"project":"/p/work"}}}`)
	text := promptText(t, got[0]["result"].(map[string]any))
	if !strings.Contains(text, "ship the release") {
		t.Errorf("the prompt should carry the project's tasks:\n%s", text)
	}
	if strings.Contains(text, "buy milk") {
		t.Errorf("and nothing from another project:\n%s", text)
	}
}

func TestUnknownPromptIsAnError(t *testing.T) {
	got := serve(t, `{"jsonrpc":"2.0","id":1,"method":"prompts/get","params":{"name":"frobnicate"}}`)
	if e := errorOf(got[0]); e == nil || e["code"].(float64) != -32602 {
		t.Fatalf("want an invalid-params error, got %v", got[0])
	}
}
