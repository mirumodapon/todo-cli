// Package mcp serves the task list over the Model Context Protocol, so an MCP
// client can read and write the same ~/.todo the CLI and the TUI use.
//
// The transport is JSON-RPC 2.0 over stdio: one JSON object per line, in and
// out. That is little enough to write by hand, and writing it by hand is what
// keeps this program's dependency list at four.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"todo.mirumo.net/internal/store"
)

// Protocol versions this server speaks, newest first. A client asking for one
// of these is answered with the same; anything else is answered with the newest,
// which is how the client learns what it is talking to.
var versions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

const latestVersion = "2025-06-18"

// JSON-RPC error codes, from the specification.
const (
	codeParse          = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternal       = -32603
)

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *rpcError) Error() string { return e.Message }

func errf(code int, format string, a ...any) *rpcError {
	return &rpcError{Code: code, Message: fmt.Sprintf(format, a...)}
}

// Server answers MCP requests against a store. Now is a function rather than a
// timestamp because this process can outlive a day: "overdue" has to be worked
// out per request, not once at startup.
type Server struct {
	Store store.Store
	Now   func() time.Time
	// Version is what the client is told it is talking to.
	Version string
}

// maxLine caps one request. The default scanner buffer is 64KB, which a task
// description can outgrow; a megabyte cannot be reached by anything sane.
const maxLine = 1 << 20

// Serve reads requests from r and writes responses to w until r ends. Requests
// are handled one at a time: this is one person's task list, and a queue of one
// removes every question about concurrent writes.
func (s *Server) Serve(r io.Reader, w io.Writer) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), maxLine)
	enc := json.NewEncoder(w)
	// Every response is a single line: stdout is the transport, and anything
	// else written there would corrupt the session.
	enc.SetIndent("", "")

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		res, reply := s.handleLine([]byte(line))
		if !reply {
			continue
		}
		if err := enc.Encode(res); err != nil {
			return err
		}
	}
	return sc.Err()
}

// handleLine turns one line into a response, and reports whether there is one to
// send: a notification is answered with silence.
func (s *Server) handleLine(line []byte) (response, bool) {
	if bracket := strings.TrimLeft(string(line), " \t"); strings.HasPrefix(bracket, "[") {
		return errorResponse(nil, errf(codeInvalidRequest,
			"this server does not take batched requests; send one request per line")), true
	}
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		return errorResponse(nil, errf(codeParse, "cannot parse the request: %s", err)), true
	}
	if req.Method == "" {
		return errorResponse(req.ID, errf(codeInvalidRequest, "the request has no method")), true
	}
	// A request without an id is a notification: act on it, answer nothing.
	if len(req.ID) == 0 {
		s.dispatch(req)
		return response{}, false
	}
	result, rerr := s.dispatch(req)
	if rerr != nil {
		return errorResponse(req.ID, rerr), true
	}
	return response{JSONRPC: "2.0", ID: req.ID, Result: result}, true
}

func errorResponse(id json.RawMessage, e *rpcError) response {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	return response{JSONRPC: "2.0", ID: id, Error: e}
}

func (s *Server) dispatch(req request) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		return s.initialize(req.Params)
	case "ping":
		// A ping carries nothing and means "are you there".
		return map[string]any{}, nil
	case "tools/list":
		return s.listTools()
	case "tools/call":
		return s.callTool(req.Params)
	case "resources/list":
		return s.listResources()
	case "resources/read":
		return s.readResource(req.Params)
	case "prompts/list":
		return s.listPrompts()
	case "prompts/get":
		return s.getPrompt(req.Params)
	case "notifications/initialized", "notifications/cancelled":
		return nil, nil
	}
	return nil, errf(codeMethodNotFound, "unknown method %q", req.Method)
}

func (s *Server) initialize(params json.RawMessage) (any, *rpcError) {
	var p struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, errf(codeInvalidParams, "cannot read the initialize parameters: %s", err)
		}
	}
	version := latestVersion
	for _, v := range versions {
		if v == p.ProtocolVersion {
			version = v
			break
		}
	}
	return map[string]any{
		"protocolVersion": version,
		"capabilities": map[string]any{
			// No listChanged anywhere: the data changes underneath this server
			// whenever the CLI writes, and promising to announce that would be
			// a promise it cannot keep.
			"tools":     map[string]any{},
			"resources": map[string]any{},
			"prompts":   map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "todo",
			"title":   "todo — a local task list",
			"version": s.version(),
		},
	}, nil
}

// version is what initialize reports. An unstamped build says so rather than
// inventing a number.
func (s *Server) version() string {
	if s.Version == "" {
		return "devel"
	}
	return s.Version
}
