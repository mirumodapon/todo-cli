package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"todo.mirumo.net/internal/datearg"
	"todo.mirumo.net/internal/project"
	"todo.mirumo.net/internal/task"
)

// tool is one callable. run returns the text to send back; the error it returns
// is a tool failure, reported as a result the model can read and act on rather
// than as a protocol error.
type tool struct {
	name        string
	title       string
	description string
	schema      map[string]any
	annotations map[string]any
	run         func(s *Server, args json.RawMessage) (string, error)
}

func prop(typ, description string) map[string]any {
	return map[string]any{"type": typ, "description": description}
}

func arrayOf(typ, description string) map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": typ}, "description": description}
}

func object(props map[string]any, required ...string) map[string]any {
	schema := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// idSchema is shared by the tools that name a single task.
func idSchema() map[string]any {
	return object(map[string]any{"id": prop("integer", "The task's id, as shown in a listing.")}, "id")
}

const dueHelp = "today, week, overdue, or a date: tomorrow, fri, +3d, 2026-09-01."
const priHelp = "low, med, high, or the marks a listing shows: !, !!, !!!."

func (s *Server) tools() []tool {
	return []tool{
		{
			name: "list_tasks", title: "List tasks",
			description: "List tasks, with optional filters. Descriptions are left out; use get_task for one task in full.",
			schema: object(map[string]any{
				"project":      prop("string", `Leave this out for every project; pass "" for uncategorized tasks only; pass a path for one project.`),
				"tags":         arrayOf("string", "Only tasks carrying all of these tags."),
				"untagged":     prop("boolean", "Only tasks with no tags at all."),
				"due":          prop("string", "Only tasks due in this range: "+dueHelp),
				"priority":     prop("string", "Only tasks at this priority: "+priHelp),
				"search":       prop("string", "Only tasks whose title contains this text."),
				"include_done": prop("boolean", "Include tasks that are done. False by default."),
				"only_done":    prop("boolean", "Only tasks that are done."),
				"sort":         prop("string", "id (the default), due, pri, or created."),
				"reverse":      prop("boolean", "Reverse whatever order sort chose."),
			}),
			annotations: map[string]any{"readOnlyHint": true},
			run:         (*Server).listTasks,
		},
		{
			name: "get_task", title: "Get a task",
			description: "One task in full, including its description.",
			schema:      idSchema(),
			annotations: map[string]any{"readOnlyHint": true},
			run:         (*Server).getTask,
		},
		{
			name: "add_task", title: "Add a task",
			description: "Add a task and return its id.",
			schema: object(map[string]any{
				"title":    prop("string", "What needs doing."),
				"desc":     prop("string", "The long form, over as many lines as it takes."),
				"project":  prop("string", `A project path. Leave it out for an uncategorized task.`),
				"tags":     arrayOf("string", "Tags to carry."),
				"due":      prop("string", "When it is due: "+dueHelp),
				"priority": prop("string", priHelp),
			}, "title"),
			annotations: map[string]any{"idempotentHint": false},
			run:         (*Server).addTask,
		},
		{
			name: "edit_task", title: "Edit a task",
			description: "Change a task. Only the fields you pass are touched; pass an empty value to clear one.",
			schema: object(map[string]any{
				"id":       prop("integer", "The task's id."),
				"title":    prop("string", "A new title."),
				"desc":     prop("string", `A new description; "" clears it.`),
				"project":  prop("string", `A new project path; "" makes it uncategorized.`),
				"tags":     arrayOf("string", "The complete new set of tags; [] clears them."),
				"due":      prop("string", `A new due date; "" clears it. `+dueHelp),
				"priority": prop("string", `A new priority; "" clears it. `+priHelp),
			}, "id"),
			annotations: map[string]any{"idempotentHint": true},
			run:         (*Server).editTask,
		},
		{
			name: "complete_task", title: "Complete a task",
			description: "Mark a task as done.",
			schema:      idSchema(),
			annotations: map[string]any{"idempotentHint": true},
			run:         func(s *Server, a json.RawMessage) (string, error) { return s.setDone(a, true) },
		},
		{
			name: "reopen_task", title: "Reopen a task",
			description: "Mark a task as not done.",
			schema:      idSchema(),
			annotations: map[string]any{"idempotentHint": true},
			run:         func(s *Server, a json.RawMessage) (string, error) { return s.setDone(a, false) },
		},
		{
			name: "delete_task", title: "Delete a task",
			description: "Delete a task. There is no undo outside the interactive interface.",
			schema:      idSchema(),
			annotations: map[string]any{"destructiveHint": true, "idempotentHint": true},
			run:         (*Server).deleteTask,
		},
	}
}

func (s *Server) listTools() (any, *rpcError) {
	out := make([]map[string]any, 0, len(s.tools()))
	for _, t := range s.tools() {
		out = append(out, map[string]any{
			"name":        t.name,
			"title":       t.title,
			"description": t.description,
			"inputSchema": t.schema,
			"annotations": t.annotations,
		})
	}
	return map[string]any{"tools": out}, nil
}

func (s *Server) callTool(params json.RawMessage) (any, *rpcError) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, errf(codeInvalidParams, "cannot read the call: %s", err)
	}
	for _, t := range s.tools() {
		if t.name != p.Name {
			continue
		}
		text, err := t.run(s, p.Arguments)
		if err != nil {
			// A tool that fails is a result, not a protocol error: the model is
			// meant to read the message and try something else.
			return toolResult(err.Error(), true), nil
		}
		return toolResult(text, false), nil
	}
	return nil, errf(codeInvalidParams, "unknown tool %q", p.Name)
}

func toolResult(text string, isError bool) map[string]any {
	return map[string]any{
		"content": []any{map[string]any{"type": "text", "text": text}},
		"isError": isError,
	}
}

// unmarshalArgs reads a tool's arguments, treating an absent object as an empty one.
func unmarshalArgs(raw json.RawMessage, into any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return fmt.Errorf("cannot read the arguments: %w", err)
	}
	return nil
}

// taskView is what a task looks like on the wire. Empty fields are left out, so
// what arrives says only what is true.
type taskView struct {
	ID           int64    `json:"id"`
	Title        string   `json:"title"`
	Done         bool     `json:"done"`
	Desc         string   `json:"desc,omitempty"`
	HasDesc      bool     `json:"has_desc,omitempty"`
	Project      string   `json:"project,omitempty"`
	ProjectLabel string   `json:"project_label,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	Due          string   `json:"due,omitempty"`
	DueHasTime   bool     `json:"due_has_time,omitempty"`
	Remaining    string   `json:"remaining,omitempty"`
	Priority     string   `json:"priority,omitempty"`
	DoneAt       string   `json:"done_at,omitempty"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

// view renders a task. full carries the description; a listing says only that
// there is one, because descriptions are prose and a list of them is a wall.
func view(t task.Task, now time.Time, full bool) taskView {
	v := taskView{
		ID: t.ID, Title: t.Title, Done: t.Done(),
		Project: t.Project, ProjectLabel: project.Label(t.Project),
		Tags: t.Tags, Priority: t.Priority.String(),
		CreatedAt: t.CreatedAt.Format(time.RFC3339),
		UpdatedAt: t.UpdatedAt.Format(time.RFC3339),
	}
	if full {
		v.Desc = t.Desc
	} else {
		v.HasDesc = t.Desc != ""
	}
	if t.Due != nil {
		layout := "2006-01-02"
		if t.DueHasTime {
			layout = "2006-01-02 15:04"
		}
		v.Due = t.Due.Format(layout)
		v.DueHasTime = t.DueHasTime
		v.Remaining = datearg.Remaining(*t.Due, t.DueHasTime, now)
	}
	if t.DoneAt != nil {
		v.DoneAt = t.DoneAt.Format(time.RFC3339)
	}
	return v
}

func asJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *Server) listTasks(raw json.RawMessage) (string, error) {
	var a struct {
		Project     *string  `json:"project"`
		Tags        []string `json:"tags"`
		Untagged    bool     `json:"untagged"`
		Due         string   `json:"due"`
		Priority    string   `json:"priority"`
		Search      string   `json:"search"`
		IncludeDone bool     `json:"include_done"`
		OnlyDone    bool     `json:"only_done"`
		Sort        string   `json:"sort"`
		Reverse     bool     `json:"reverse"`
	}
	if err := unmarshalArgs(raw, &a); err != nil {
		return "", err
	}
	now := s.Now()
	f := task.Filter{
		Project: a.Project, Tags: a.Tags, Untagged: a.Untagged,
		Search: a.Search, IncludeDone: a.IncludeDone, OnlyDone: a.OnlyDone,
		Reverse: a.Reverse,
	}
	var err error
	if f.DueRange, f.DueOn, err = task.ParseDueFilter(a.Due, now); err != nil {
		return "", err
	}
	if strings.TrimSpace(a.Priority) != "" {
		p, err := task.ParsePriority(a.Priority)
		if err != nil {
			return "", err
		}
		f.Priority = &p
	}
	if f.Sort, err = task.ParseSortBy(a.Sort); err != nil {
		return "", err
	}
	ts, err := s.Store.List(f, now)
	if err != nil {
		return "", err
	}
	out := make([]taskView, 0, len(ts))
	for _, t := range ts {
		out = append(out, view(t, now, false))
	}
	return asJSON(out)
}

// withID reads an {"id": n} argument object.
func withID(raw json.RawMessage) (int64, error) {
	var a struct {
		ID int64 `json:"id"`
	}
	if err := unmarshalArgs(raw, &a); err != nil {
		return 0, err
	}
	if a.ID <= 0 {
		return 0, fmt.Errorf("id must be a positive number, got %d", a.ID)
	}
	return a.ID, nil
}

func (s *Server) getTask(raw json.RawMessage) (string, error) {
	id, err := withID(raw)
	if err != nil {
		return "", err
	}
	t, err := s.Store.Get(id)
	if err != nil {
		return "", fmt.Errorf("#%d: %w", id, err)
	}
	return asJSON(view(t, s.Now(), true))
}

func (s *Server) addTask(raw json.RawMessage) (string, error) {
	var a struct {
		Title    string   `json:"title"`
		Desc     string   `json:"desc"`
		Project  string   `json:"project"`
		Tags     []string `json:"tags"`
		Due      string   `json:"due"`
		Priority string   `json:"priority"`
	}
	if err := unmarshalArgs(raw, &a); err != nil {
		return "", err
	}
	now := s.Now()
	t := task.Task{
		Desc: a.Desc, Project: a.Project, Tags: task.NormalizeTags(a.Tags),
		CreatedAt: now, UpdatedAt: now,
	}
	var err error
	if t.Title, err = task.ValidateTitle(a.Title); err != nil {
		return "", err
	}
	if t.Priority, err = task.ParsePriority(a.Priority); err != nil {
		return "", err
	}
	if strings.TrimSpace(a.Due) != "" {
		d, hasTime, err := datearg.Parse(a.Due, now)
		if err != nil {
			return "", err
		}
		t.Due, t.DueHasTime = &d, hasTime
	}
	got, err := s.Store.Add(t)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("added #%d: %s", got.ID, got.Title), nil
}

func (s *Server) editTask(raw json.RawMessage) (string, error) {
	var a struct {
		ID       int64     `json:"id"`
		Title    *string   `json:"title"`
		Desc     *string   `json:"desc"`
		Project  *string   `json:"project"`
		Tags     *[]string `json:"tags"`
		Due      *string   `json:"due"`
		Priority *string   `json:"priority"`
	}
	if err := unmarshalArgs(raw, &a); err != nil {
		return "", err
	}
	if a.ID <= 0 {
		return "", fmt.Errorf("id must be a positive number, got %d", a.ID)
	}
	t, err := s.Store.Get(a.ID)
	if err != nil {
		return "", fmt.Errorf("#%d: %w", a.ID, err)
	}
	now := s.Now()
	// Absent means leave it alone; present means set it, empty included. JSON
	// gives that for nothing, where the command line needed a whole flag kind.
	if a.Title != nil {
		if t.Title, err = task.ValidateTitle(*a.Title); err != nil {
			return "", err
		}
	}
	if a.Desc != nil {
		t.Desc = *a.Desc
	}
	if a.Project != nil {
		t.Project = *a.Project
	}
	if a.Tags != nil {
		t.Tags = task.NormalizeTags(*a.Tags)
	}
	if a.Priority != nil {
		if t.Priority, err = task.ParsePriority(*a.Priority); err != nil {
			return "", err
		}
	}
	if a.Due != nil {
		if strings.TrimSpace(*a.Due) == "" {
			t.Due, t.DueHasTime = nil, false
		} else {
			d, hasTime, err := datearg.Parse(*a.Due, now)
			if err != nil {
				return "", err
			}
			t.Due, t.DueHasTime = &d, hasTime
		}
	}
	t.UpdatedAt = now
	if err := s.Store.Update(t); err != nil {
		return "", fmt.Errorf("#%d: %w", t.ID, err)
	}
	return fmt.Sprintf("updated #%d: %s", t.ID, t.Title), nil
}

func (s *Server) setDone(raw json.RawMessage, done bool) (string, error) {
	id, err := withID(raw)
	if err != nil {
		return "", err
	}
	t, err := s.Store.Get(id)
	if err != nil {
		return "", fmt.Errorf("#%d: %w", id, err)
	}
	if err := s.Store.SetDone(id, done, s.Now()); err != nil {
		return "", fmt.Errorf("#%d: %w", id, err)
	}
	verb := "done"
	if !done {
		verb = "reopened"
	}
	return fmt.Sprintf("%s #%d: %s", verb, id, t.Title), nil
}

func (s *Server) deleteTask(raw json.RawMessage) (string, error) {
	id, err := withID(raw)
	if err != nil {
		return "", err
	}
	t, err := s.Store.Get(id)
	if err != nil {
		return "", fmt.Errorf("#%d: %w", id, err)
	}
	if err := s.Store.Delete(id); err != nil {
		return "", fmt.Errorf("#%d: %w", id, err)
	}
	return fmt.Sprintf("deleted #%d: %s", id, t.Title), nil
}
