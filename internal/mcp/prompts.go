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

// promptArg is one argument a prompt takes.
type promptArg struct {
	name        string
	description string
	required    bool
}

// prompt is a canned request with the current tasks written into it, so the
// model is not asked to plan a day it cannot see.
type prompt struct {
	name        string
	title       string
	description string
	args        []promptArg
	build       func(s *Server, args map[string]string) (string, error)
}

func (s *Server) prompts() []prompt {
	return []prompt{
		{
			name: "plan_today", title: "Plan today",
			description: "Put today's tasks, and anything overdue, in an order worth working through.",
			args: []promptArg{
				{name: "project", description: `Only this project's tasks. Omit for every project; "" for uncategorized only.`},
			},
			build: (*Server).planToday,
		},
		{
			name: "review_project", title: "Review a project",
			description: "Go through everything still open in one project.",
			args: []promptArg{
				{name: "project", description: "The project path, as task://projects lists it.", required: true},
			},
			build: (*Server).reviewProject,
		},
	}
}

func (s *Server) listPrompts() (any, *rpcError) {
	out := make([]map[string]any, 0, len(s.prompts()))
	for _, p := range s.prompts() {
		args := make([]map[string]any, 0, len(p.args))
		for _, a := range p.args {
			args = append(args, map[string]any{
				"name": a.name, "description": a.description, "required": a.required,
			})
		}
		out = append(out, map[string]any{
			"name": p.name, "title": p.title, "description": p.description, "arguments": args,
		})
	}
	return map[string]any{"prompts": out}, nil
}

func (s *Server) getPrompt(params json.RawMessage) (any, *rpcError) {
	var p struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, errf(codeInvalidParams, "cannot read the request: %s", err)
	}
	for _, pr := range s.prompts() {
		if pr.name != p.Name {
			continue
		}
		for _, a := range pr.args {
			if a.required && strings.TrimSpace(p.Arguments[a.name]) == "" {
				return nil, errf(codeInvalidParams, "%s needs a %q argument", pr.name, a.name)
			}
		}
		text, err := pr.build(s, p.Arguments)
		if err != nil {
			return nil, errf(codeInternal, "building %s: %s", pr.name, err)
		}
		return map[string]any{
			"description": pr.description,
			"messages": []any{map[string]any{
				"role":    "user",
				"content": map[string]any{"type": "text", "text": text},
			}},
		}, nil
	}
	return nil, errf(codeInvalidParams, "unknown prompt %q", p.Name)
}

// promptLine renders one task the way a listing does, ids included: the whole
// point is that the model can act on what it is shown.
func promptLine(t task.Task, now time.Time) string {
	parts := []string{fmt.Sprintf("#%d", t.ID)}
	if m := t.Priority.Marks(); m != "" {
		parts = append(parts, m)
	}
	if t.Due != nil {
		parts = append(parts, datearg.Remaining(*t.Due, t.DueHasTime, now))
	}
	parts = append(parts, t.Title)
	if p := project.Label(t.Project); p != "" {
		parts = append(parts, p)
	}
	if len(t.Tags) > 0 {
		parts = append(parts, "@"+strings.Join(t.Tags, " @"))
	}
	if t.Desc != "" {
		parts = append(parts, "(has a description)")
	}
	return strings.Join(parts, " ")
}

// lines renders a filtered query, or says there was nothing. An empty list
// rendered as an empty list reads like a mistake.
func (s *Server) lines(f task.Filter) (string, bool, error) {
	now := s.Now()
	ts, err := s.Store.List(f, now)
	if err != nil {
		return "", false, err
	}
	if len(ts) == 0 {
		return "", false, nil
	}
	var b strings.Builder
	for _, t := range ts {
		b.WriteString(promptLine(t, now) + "\n")
	}
	return b.String(), true, nil
}

// section renders a heading and its tasks, or nothing at all when the bucket is
// empty: an empty heading is noise the model has to read past.
func section(heading string, ts []task.Task, now time.Time) string {
	if len(ts) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(heading + "\n")
	for _, t := range ts {
		b.WriteString(promptLine(t, now) + "\n")
	}
	b.WriteString("\n")
	return b.String()
}

// projectFilter reads the optional project argument: absent means every
// project, present means that one, including the empty string for uncategorized.
func projectFilter(args map[string]string) *string {
	v, ok := args["project"]
	if !ok {
		return nil
	}
	return &v
}

func (s *Server) planToday(args map[string]string) (string, error) {
	now := s.Now()
	// One query for the week, then split by day: three round trips would say
	// the same thing, and the boundaries have to agree with each other anyway.
	ts, err := s.Store.List(task.Filter{
		Project: projectFilter(args), DueRange: task.DueWeek, Sort: task.SortDue,
	}, now)
	if err != nil {
		return "", err
	}
	today := datearg.Day(now)
	var overdue, due, later []task.Task
	for _, t := range ts {
		switch d := datearg.Day(*t.Due); {
		case d.Before(today):
			overdue = append(overdue, t)
		case d.Equal(today):
			due = append(due, t)
		default:
			later = append(later, t)
		}
	}
	if len(overdue)+len(due) == 0 && len(later) == 0 {
		return "Nothing is due today, nothing is overdue, and nothing falls due this week. Say so, and offer to look at what has no due date at all.", nil
	}
	body := section("Overdue:", overdue, now) +
		section("Due today:", due, now) +
		section("Later this week:", later, now)
	return body + `Work out an order to work through today. Say what matters now and what can
wait, call out anything that looks stale or needs a decision from me, and refer
to tasks by id. If I ask you to act on any of it, use complete_task and
edit_task — but change nothing before I ask.`, nil
}

func (s *Server) reviewProject(args map[string]string) (string, error) {
	p := args["project"]
	f := task.Filter{Project: &p, Sort: task.SortDue}
	list, any, err := s.lines(f)
	if err != nil {
		return "", err
	}
	label := project.Label(p)
	if label == "" {
		label = "the uncategorized tasks"
	}
	if !any {
		return "There is nothing open in " + label + ". Say so.", nil
	}
	return "Everything still open in " + label + ":\n\n" + list + `
Group these into what is nearly done, what has not been started, and what looks
like it should be dropped. Point out anything missing a due date that clearly
needs one, and refer to tasks by id.`, nil
}
