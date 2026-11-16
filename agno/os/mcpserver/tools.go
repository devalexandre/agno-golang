package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/os/component"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ---------------------------------------------------------------- schemas

func strProp(desc string, maxLen int) *jsonschema.Schema {
	return &jsonschema.Schema{Type: "string", Description: desc, MaxLength: ptr(maxLen)}
}

func optStrProp(desc string, maxLen int) *jsonschema.Schema {
	return &jsonschema.Schema{Types: []string{"string", "null"}, Description: desc, MaxLength: ptr(maxLen)}
}

func enumProp(desc string, def string, values ...string) *jsonschema.Schema {
	vals := make([]any, len(values))
	for i, v := range values {
		vals[i] = v
	}
	s := &jsonschema.Schema{Type: "string", Description: desc, Enum: vals}
	if def != "" {
		s.Default = json.RawMessage(fmt.Sprintf("%q", def))
	}
	return s
}

func intProp(desc string, min, max, def int) *jsonschema.Schema {
	s := &jsonschema.Schema{Type: "integer", Description: desc, Minimum: ptr(float64(min)), Default: json.RawMessage(fmt.Sprint(def))}
	if max > 0 {
		s.Maximum = ptr(float64(max))
	}
	return s
}

func object(props map[string]*jsonschema.Schema, required ...string) *jsonschema.Schema {
	return &jsonschema.Schema{Type: "object", Properties: props, Required: required, AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}}}
}

func (s *Server) maxMsg() int { return orDefault(s.cfg.MaxMessageLength, DefaultMaxMessageLength) }
func (s *Server) maxID() int  { return orDefault(s.cfg.MaxIDLength, DefaultMaxIDLength) }

// runSchema is the input schema of the run tools. idName is the component id
// parameter of the generic tools ("" for an exposed component).
func (s *Server) runSchema(_ bool, idName string) *jsonschema.Schema {
	props := map[string]*jsonschema.Schema{
		"message": {Type: "string", Description: "The request or message to send to the agent, team, or workflow.",
			MinLength: ptr(1), MaxLength: ptr(s.maxMsg())},
		"user_id":    optStrProp("Optional user ID to associate with the run. Authenticated deployments use the caller's identity.", s.maxID()),
		"session_id": optStrProp("Existing session ID to continue the same conversation. Omit to start a new session.", s.maxID()),
	}
	required := []string{"message"}
	if idName != "" {
		props[idName] = strProp(fmt.Sprintf("ID of the %s to run, returned by get_agentos_config.", idName[:len(idName)-3]), s.maxID())
		required = append([]string{idName}, required...)
	}
	return object(props, required...)
}

func ownerProps(s *Server, props map[string]*jsonschema.Schema) {
	props["agent_id"] = optStrProp("ID of the agent that owns the run. Set exactly one of agent_id, team_id, workflow_id.", s.maxID())
	props["team_id"] = optStrProp("ID of the team that owns the run.", s.maxID())
	props["workflow_id"] = optStrProp("ID of the workflow that owns the run.", s.maxID())
}

// ---------------------------------------------------------------- args

func argStr(args map[string]any, k string) string {
	v, _ := args[k].(string)
	return v
}

func argInt(args map[string]any, k string, def int) int {
	switch v := args[k].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	}
	return def
}

// resolveUserID: an authenticated identity always replaces the advisory arg.
func resolveUserID(ctx context.Context, advisory string) string {
	if id := IdentityFromContext(ctx); id != nil && id.UserID != "" {
		return id.UserID
	}
	return advisory
}

// ---------------------------------------------------------------- builtins

var readOnly = &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, DestructiveHint: ptr(false), OpenWorldHint: ptr(false)}
var runAnnotations = &mcp.ToolAnnotations{DestructiveHint: ptr(true), OpenWorldHint: ptr(true)}

func (s *Server) registerBuiltins() error {
	type builtin struct {
		def  *toolDef
		tags []string
	}
	maxReq := DefaultMaxRequirements
	continueProps := map[string]*jsonschema.Schema{
		"run_id":     strProp("ID of the paused run, returned in the PAUSED result.", s.maxID()),
		"session_id": strProp("ID of the session containing the paused run, returned in the PAUSED result.", s.maxID()),
		"requirements": {Types: []string{"array", "null"}, MaxItems: ptr(maxReq), Items: &jsonschema.Schema{Type: "object"},
			Description: "Copy the requirements from the PAUSED result and fill in each requested resolution, such as confirmation=true (or false with confirmation_note), user_input_schema[].value, user_feedback_schema[].selected_options or external_execution_result."},
		"user_id": optStrProp("Optional user ID to associate with the run. Authenticated deployments use the caller's identity.", s.maxID()),
	}
	ownerProps(s, continueProps)
	cancelProps := map[string]*jsonschema.Schema{"run_id": strProp("ID of the run to cancel.", s.maxID())}
	ownerProps(s, cancelProps)

	builtins := []builtin{
		{tags: []string{TagCore}, def: &toolDef{
			tool: &mcp.Tool{Name: "get_agentos_config", Title: "Get AgentOS Configuration",
				Description: "Discover this AgentOS: the agents, teams, and workflows available to run (with their ids and descriptions), and the database ids used by the session tools. Call this first to learn what you can operate.",
				InputSchema: object(map[string]*jsonschema.Schema{}), Annotations: readOnly},
			handler: s.toolConfig}},
		{tags: []string{TagCore}, def: s.runTool("run_agent", "Run Agent", component.KindAgent, "agent_id",
			"Run an agent with a message and get its response. If the result status is PAUSED, resolve the returned requirements and call continue_run.")},
		{tags: []string{TagCore}, def: s.runTool("run_team", "Run Team", component.KindTeam, "team_id",
			"Run a team with a message and get its response. If the result status is PAUSED, resolve the returned requirements and call continue_run.")},
		{tags: []string{TagCore}, def: s.runTool("run_workflow", "Run Workflow", component.KindWorkflow, "workflow_id",
			"Run a workflow with a message and get its result.")},
		{tags: []string{TagCore, TagLifecycle}, def: &toolDef{
			tool: &mcp.Tool{Name: "continue_run", Title: "Continue Paused Run",
				Description: "Resume a PAUSED run after resolving its requirements (human-in-the-loop). The paused result's structuredContent carries the run_id, session_id, owning component id, and the unresolved requirements.",
				InputSchema: object(continueProps, "run_id", "session_id"), Annotations: runAnnotations},
			handler: s.toolContinue}},
		{tags: []string{TagCore, TagLifecycle}, def: &toolDef{
			tool: &mcp.Tool{Name: "cancel_run", Title: "Cancel Run",
				Description: "Request cancellation of an in-flight run of an agent, team, or workflow.",
				InputSchema: object(cancelProps, "run_id"), Annotations: runAnnotations},
			handler: s.toolCancel}},
		{tags: []string{TagSession}, def: &toolDef{
			tool: &mcp.Tool{Name: "get_sessions", Title: "List Sessions",
				Description: "List past sessions (conversations), newest first. Use a returned session_id with the run tools to continue that conversation, or with get_session_runs to read its history.",
				InputSchema: object(map[string]*jsonschema.Schema{
					"session_type": enumProp("Type of component that created the sessions. Defaults to agent.", "agent", "agent", "team", "workflow"),
					"component_id": optStrProp("Only return sessions created by this agent, team, or workflow.", s.maxID()),
					"user_id":      optStrProp("Only return sessions belonging to this user.", s.maxID()),
					"session_name": optStrProp("Only return sessions whose name matches this value.", s.maxID()),
					"db_id":        optStrProp("Database ID returned by get_agentos_config. Required only when multiple databases are listed.", s.maxID()),
					"limit":        intProp("Maximum number of sessions to return per page.", 1, orDefault(s.cfg.MaxPageSize, DefaultMaxPageSize), 20),
					"page":         intProp("One-based page number to return.", 1, 0, 1),
					"sort_by":      enumProp("Timestamp used to sort sessions: created_at or updated_at.", "created_at", "created_at", "updated_at"),
					"sort_order":   enumProp("Sort in ascending (asc) or descending (desc) order.", "desc", "asc", "desc"),
				}), Annotations: readOnly},
			handler: s.toolSessions}},
		{tags: []string{TagSession}, def: &toolDef{
			tool: &mcp.Tool{Name: "get_session_runs", Title: "Read Session History",
				Description: "Read a session's conversation history: each run's input and response content with its run_id, status, and timestamp, oldest first.",
				InputSchema: object(map[string]*jsonschema.Schema{
					"session_id":   strProp("ID of the session to read, returned by get_sessions or a run result.", s.maxID()),
					"run_id":       optStrProp("Return this run with its full stored details. Omit for a compact history of every run.", s.maxID()),
					"session_type": {Types: []string{"string", "null"}, Enum: []any{"agent", "team", "workflow", nil}, Description: "Type of component that created the session."},
					"user_id":      optStrProp("Only read the session if it belongs to this user.", s.maxID()),
					"db_id":        optStrProp("Database ID returned by get_agentos_config.", s.maxID()),
				}, "session_id"), Annotations: readOnly},
			handler: s.toolSessionRuns}},
	}
	for _, b := range builtins {
		if !hasTag(s.enabled, b.tags...) {
			continue
		}
		b.def.tags = b.tags
		if err := s.add(b.def); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) runTool(name, title string, kind component.Kind, idName, desc string) *toolDef {
	return &toolDef{
		tool: &mcp.Tool{Name: name, Title: title, Description: desc, InputSchema: s.runSchema(true, idName), Annotations: runAnnotations},
		handler: func(ctx context.Context, req *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, error) {
			id := argStr(args, idName)
			c, ok := s.reg.Get(kind, id)
			if !ok {
				return nil, fmt.Errorf("%s %q not found", kind, id)
			}
			return s.runComponent(ctx, req, c, args)
		},
	}
}

func (s *Server) continueAvailable() bool { _, ok := s.tools["continue_run"]; return ok }

func (s *Server) runComponent(ctx context.Context, req *mcp.CallToolRequest, c component.Component, args map[string]any) (*mcp.CallToolResult, error) {
	sessionID := argStr(args, "session_id")
	if sessionID == "" {
		// A fresh session per call when omitted (matches REST).
		sessionID = uuid.NewString()
	}
	rr := component.RunRequest{Input: argStr(args, "message"), SessionID: sessionID, UserID: resolveUserID(ctx, argStr(args, "user_id")), RunID: uuid.NewString()}
	progress := progressReporter(ctx, req, fmt.Sprintf("%s %s", c.Kind(), c.Name()))
	res, err := component.Stream(ctx, c, rr, func(ev component.Event) error {
		progress(ev)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.runResult(c, res), nil
}

func progressReporter(ctx context.Context, req *mcp.CallToolRequest, label string) func(component.Event) {
	if req == nil || req.Session == nil || req.Params == nil {
		return func(component.Event) {}
	}
	token := req.Params.GetProgressToken()
	if token == nil {
		return func(component.Event) {}
	}
	n := 0.0
	return func(ev component.Event) {
		var msg string
		switch ev.Type {
		case component.EventRunStarted:
			msg = label + " started"
		case component.EventToolCallStarted:
			if ev.Tool != nil {
				msg = "Calling tool " + ev.Tool.Name
			}
		case component.EventToolCallCompleted:
			if ev.Tool != nil {
				msg = "Tool " + ev.Tool.Name + " completed"
			}
		default:
			return
		}
		n++
		_ = req.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{ProgressToken: token, Progress: n, Message: msg})
	}
}

// runResult renders a run as an MCP tool result (Python build_run_tool_result).
func (s *Server) runResult(c component.Component, res *component.RunResult) *mcp.CallToolResult {
	text := res.Content
	if res.Status == component.StatusPaused {
		if text == "" {
			text = fmt.Sprintf("Run paused: %d requirement(s) awaiting resolution.", len(res.Requirements))
		}
		if !s.continueAvailable() {
			text += " The continue_run tool is not registered on this server; resume this run over the REST API, or enable the run-lifecycle tools (LifecycleTools=true, and do not exclude the \"lifecycle\" tag)."
		}
	}
	if res.Status == component.StatusError && text == "" {
		text = res.Error
	}
	var structured any
	if s.cfg.ResultMode == ResultFull {
		full := map[string]any{}
		if b, err := json.Marshal(res); err == nil {
			_ = json.Unmarshal(b, &full)
		}
		if res.Raw != nil {
			if b, err := json.Marshal(res.Raw); err == nil {
				var raw any
				if json.Unmarshal(b, &raw) == nil {
					full["raw"] = raw
				}
			}
		}
		structured = full
	} else {
		trimmed := map[string]any{
			"run_id":     res.RunID,
			"session_id": res.SessionID,
			"status":     string(res.Status),
			"content":    text,
		}
		trimmed[string(c.Kind())+"_id"] = c.ID()
		if res.Status == component.StatusPaused {
			trimmed["requirements"] = res.Requirements
		}
		structured = trimmed
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}, StructuredContent: structured, IsError: res.Status == component.StatusError}
}

func (s *Server) toolConfig(ctx context.Context, _ *mcp.CallToolRequest, _ map[string]any) (*mcp.CallToolResult, error) {
	summary := func(kind component.Kind) []map[string]any {
		out := []map[string]any{}
		for _, c := range s.reg.List(kind) {
			out = append(out, map[string]any{"id": c.ID(), "name": c.Name(), "description": c.Description()})
		}
		return out
	}
	dbs := []string{}
	if s.cfg.Sessions != nil {
		dbs = append(dbs, s.cfg.Sessions.DatabaseIDs()...)
	}
	osID := s.cfg.OSID
	if osID == "" {
		osID = "AgentOS"
	}
	return jsonResult(map[string]any{
		"os_id":       osID,
		"description": s.cfg.Description,
		"databases":   dbs,
		"agents":      summary(component.KindAgent),
		"teams":       summary(component.KindTeam),
		"workflows":   summary(component.KindWorkflow),
	})
}

// lifecycleTarget resolves the owning component of a continue/cancel call.
func (s *Server) lifecycleTarget(tool string, args map[string]any) (component.Component, error) {
	var kind component.Kind
	var id string
	n := 0
	for _, k := range []component.Kind{component.KindAgent, component.KindTeam, component.KindWorkflow} {
		if v := argStr(args, string(k)+"_id"); v != "" {
			kind, id = k, v
			n++
		}
	}
	if n != 1 {
		return nil, fmt.Errorf("%s: set exactly one of agent_id, team_id, workflow_id", tool)
	}
	c, ok := s.reg.Get(kind, id)
	if !ok {
		return nil, fmt.Errorf("%s %q not found", kind, id)
	}
	if s.lifecycleTargets != nil && !s.lifecycleTargets[lifecycleKey(kind, c.ID())] {
		return nil, fmt.Errorf("%s: %s %q is not published on this server", tool, kind, id)
	}
	return c, nil
}

func (s *Server) toolContinue(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, error) {
	c, err := s.lifecycleTarget("continue_run", args)
	if err != nil {
		return nil, err
	}
	var reqs []map[string]any
	if raw, ok := args["requirements"].([]any); ok {
		for _, r := range raw {
			if m, ok := r.(map[string]any); ok {
				reqs = append(reqs, m)
			}
		}
	}
	res, err := component.Continue(ctx, c, component.ContinueRequest{
		RunID: argStr(args, "run_id"), SessionID: argStr(args, "session_id"),
		UserID: resolveUserID(ctx, argStr(args, "user_id")), Requirements: reqs,
	})
	if err != nil {
		return nil, err
	}
	return s.runResult(c, res), nil
}

func (s *Server) toolCancel(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, error) {
	c, err := s.lifecycleTarget("cancel_run", args)
	if err != nil {
		return nil, err
	}
	runID := argStr(args, "run_id")
	ok, err := component.Cancel(ctx, c, runID)
	if err != nil {
		return nil, err
	}
	return jsonResult(map[string]any{"run_id": runID, "cancelled": ok, string(c.Kind()) + "_id": c.ID()})
}

func (s *Server) readUserID(ctx context.Context, advisory string) string {
	// An authenticated caller reads only their own sessions.
	return resolveUserID(ctx, advisory)
}

func (s *Server) toolSessions(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, error) {
	if s.cfg.Sessions == nil {
		return nil, fmt.Errorf("no session database is configured on this server")
	}
	kind, _ := component.ParseKind(argStr(args, "session_type"))
	limit := argInt(args, "limit", 20)
	page := argInt(args, "page", 1)
	sessions, total, err := s.cfg.Sessions.ListSessions(ctx, component.SessionQuery{
		SessionType: kind, ComponentID: argStr(args, "component_id"),
		UserID: s.readUserID(ctx, argStr(args, "user_id")), SessionName: argStr(args, "session_name"),
		DBID: argStr(args, "db_id"), Limit: limit, Page: page,
		SortBy: argStr(args, "sort_by"), SortOrder: argStr(args, "sort_order"),
	})
	if err != nil {
		return nil, err
	}
	totalPages := 0
	if limit > 0 {
		totalPages = (total + limit - 1) / limit
	}
	return jsonResult(map[string]any{
		"data": sessions,
		"meta": map[string]any{"page": page, "limit": limit, "total_count": total, "total_pages": totalPages},
	})
}

func (s *Server) toolSessionRuns(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, error) {
	if s.cfg.Sessions == nil {
		return nil, fmt.Errorf("no session database is configured on this server")
	}
	sessionID := argStr(args, "session_id")
	kind, _ := component.ParseKind(argStr(args, "session_type"))
	runs, err := s.cfg.Sessions.GetSessionRuns(ctx, sessionID, kind, s.readUserID(ctx, argStr(args, "user_id")), argStr(args, "db_id"))
	if err != nil {
		return nil, err
	}
	if runID := argStr(args, "run_id"); runID != "" {
		for _, r := range runs {
			if r.RunID == runID {
				return jsonResult(map[string]any{"runs": []component.RunInfo{r}})
			}
		}
		return nil, fmt.Errorf("run %s not found in session %s", runID, sessionID)
	}
	for i := range runs {
		runs[i].Details = nil // compact history
	}
	return jsonResult(map[string]any{"runs": runs})
}
