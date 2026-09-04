package mcp

import (
	"encoding/json"

	"todo.mirumo.net/internal/project"
)

// codeResourceNotFound is MCP's own code, outside the JSON-RPC range.
const codeResourceNotFound = -32002

// resource is one readable thing. read returns its body as JSON text.
type resource struct {
	uri         string
	name        string
	title       string
	description string
	read        func(s *Server) (string, error)
}

func (s *Server) resources() []resource {
	return []resource{
		{
			uri: "todo://projects", name: "projects", title: "Projects",
			description: "Every project that has tasks, with how many are still open.",
			read:        (*Server).readProjects,
		},
		{
			uri: "todo://tags", name: "tags", title: "Tags",
			description: "Every tag at least one task carries.",
			read:        (*Server).readTags,
		},
	}
}

func (s *Server) listResources() (any, *rpcError) {
	out := make([]map[string]any, 0, len(s.resources()))
	for _, r := range s.resources() {
		out = append(out, map[string]any{
			"uri":         r.uri,
			"name":        r.name,
			"title":       r.title,
			"description": r.description,
			"mimeType":    "application/json",
		})
	}
	return map[string]any{"resources": out}, nil
}

func (s *Server) readResource(params json.RawMessage) (any, *rpcError) {
	var p struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, errf(codeInvalidParams, "cannot read the request: %s", err)
	}
	for _, r := range s.resources() {
		if r.uri != p.URI {
			continue
		}
		text, err := r.read(s)
		if err != nil {
			return nil, errf(codeInternal, "reading %s: %s", r.uri, err)
		}
		return map[string]any{"contents": []any{map[string]any{
			"uri":      r.uri,
			"mimeType": "application/json",
			"text":     text,
		}}}, nil
	}
	return nil, errf(codeResourceNotFound, "no resource at %q", p.URI)
}

func (s *Server) readProjects() (string, error) {
	ps, err := s.Store.Projects()
	if err != nil {
		return "", err
	}
	type projectView struct {
		Path  string `json:"path"`
		Label string `json:"label"`
		Open  int    `json:"open"`
	}
	out := make([]projectView, 0, len(ps))
	for _, p := range ps {
		label := project.Label(p.Path)
		if label == "" {
			// Named rather than blank: an empty label reads as missing data,
			// and uncategorized is a place, not the absence of one.
			label = "(uncategorized)"
		}
		out = append(out, projectView{Path: p.Path, Label: label, Open: p.Open})
	}
	return asJSON(out)
}

func (s *Server) readTags() (string, error) {
	tags, err := s.Store.Tags()
	if err != nil {
		return "", err
	}
	if tags == nil {
		tags = []string{}
	}
	return asJSON(tags)
}
