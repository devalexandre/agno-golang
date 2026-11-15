// Package agui exposes an Agno agent or team over the AG-UI protocol
// (https://docs.ag-ui.com). Port of Python agno/os/interfaces/agui.
//
// Routes (under Config.Prefix, default ""):
//
//	POST {prefix}/agui    RunAgentInput in, AG-UI events out (SSE)
//	GET  {prefix}/status  {"status": "available"}
//
// A run that pauses (human-in-the-loop) ends with TOOL_CALL_START/ARGS/END
// events for the tools awaiting the frontend, then RUN_FINISHED. The frontend
// answers with trailing "tool" messages on the same thread, which resume the
// paused run (component.Continuer -> Agent.ContinueRun).
package agui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/os/component"
	"github.com/google/uuid"
)

// Event types (subset of ag_ui.core.EventType used by Agno).
const (
	RunStarted         = "RUN_STARTED"
	RunFinished        = "RUN_FINISHED"
	RunError           = "RUN_ERROR"
	TextMessageStart   = "TEXT_MESSAGE_START"
	TextMessageContent = "TEXT_MESSAGE_CONTENT"
	TextMessageEnd     = "TEXT_MESSAGE_END"
	ToolCallStart      = "TOOL_CALL_START"
	ToolCallArgs       = "TOOL_CALL_ARGS"
	ToolCallEnd        = "TOOL_CALL_END"
	ToolCallResult     = "TOOL_CALL_RESULT"
	StateSnapshot      = "STATE_SNAPSHOT"
	StepStarted        = "STEP_STARTED"
	StepFinished       = "STEP_FINISHED"
	Custom             = "CUSTOM"
)

// Event is an AG-UI event; fields are omitted when empty.
type Event map[string]any

// Message is an AG-UI message.
type Message struct {
	ID         string          `json:"id"`
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content,omitempty"`
	ToolCalls  []ToolCall      `json:"toolCalls,omitempty"`
	ToolCallID string          `json:"toolCallId,omitempty"`
	Error      string          `json:"error,omitempty"`
}

// ToolCall is an assistant tool call.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Text returns the message text (string content or the text parts of a list).
func (m Message) Text() string {
	if len(m.Content) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(m.Content, &s) == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(m.Content, &parts) == nil {
		var texts []string
		for _, p := range parts {
			if p.Type == "text" {
				texts = append(texts, p.Text)
			}
		}
		return strings.Join(texts, "\n")
	}
	return string(m.Content)
}

// RunAgentInput is the AG-UI request body.
type RunAgentInput struct {
	ThreadID       string           `json:"threadId"`
	RunID          string           `json:"runId"`
	State          any              `json:"state,omitempty"`
	Messages       []Message        `json:"messages"`
	Tools          []map[string]any `json:"tools,omitempty"`
	Context        []map[string]any `json:"context,omitempty"`
	ForwardedProps map[string]any   `json:"forwardedProps,omitempty"`
}

// Config configures the AG-UI interface.
type Config struct {
	Prefix string
	// Identity returns the authenticated caller; when empty the client's
	// forwardedProps.user_id is used for attribution.
	Identity func(r *http.Request) string
}

type pausedRun struct {
	runID        string
	requirements []map[string]any
}

// Server serves one agent or team over AG-UI.
type Server struct {
	cfg    Config
	comp   component.Component
	mu     sync.Mutex
	paused map[string]*pausedRun // thread id -> paused run
}

// New returns an AG-UI server for an agent or team.
func New(comp component.Component, cfg *Config) (*Server, error) {
	if comp == nil {
		return nil, errors.New("agui: an agent or a team is required")
	}
	c := Config{}
	if cfg != nil {
		c = *cfg
	}
	c.Prefix = strings.TrimRight(c.Prefix, "/")
	if c.Prefix != "" && !strings.HasPrefix(c.Prefix, "/") {
		c.Prefix = "/" + c.Prefix
	}
	return &Server{cfg: c, comp: comp, paused: map[string]*pausedRun{}}, nil
}

// Prefix returns the mount prefix.
func (s *Server) Prefix() string { return s.cfg.Prefix }

// Handler returns the HTTP handler (expects full paths, prefix included).
func (s *Server) Handler() http.Handler { return s }

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, s.cfg.Prefix)
	switch {
	case path == "/status" && r.Method == http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"available"}`))
	case path == "/agui" && r.Method == http.MethodPost:
		s.serveRun(w, r)
	case path == "/agui" && r.Method == http.MethodOptions:
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) serveRun(w http.ResponseWriter, r *http.Request) {
	var in RunAgentInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<20)).Decode(&in); err != nil {
		http.Error(w, fmt.Sprintf(`{"detail":"invalid RunAgentInput: %v"}`, err), http.StatusUnprocessableEntity)
		return
	}
	if in.ThreadID == "" {
		in.ThreadID = uuid.NewString()
	}
	userID := ""
	if s.cfg.Identity != nil {
		userID = s.cfg.Identity(r)
	}
	if userID == "" {
		userID, _ = in.ForwardedProps["user_id"].(string)
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	emit := func(ev Event) error {
		b, err := json.Marshal(ev)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}
	s.Run(r.Context(), in, userID, emit)
}

// ExtractUserInput returns the content of the last user message.
func ExtractUserInput(msgs []Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			return msgs[i].Text()
		}
	}
	return ""
}

// ExtractToolMessages returns the trailing tool messages (a resume).
func ExtractToolMessages(msgs []Message) []Message {
	var out []Message
	for i := len(msgs) - 1; i >= 0 && msgs[i].Role == "tool"; i-- {
		out = append([]Message{msgs[i]}, out...)
	}
	return out
}

// Run executes one AG-UI run, emitting its events. Exported so other
// transports (e.g. WebSocket) can reuse the mapping.
func (s *Server) Run(ctx context.Context, in RunAgentInput, userID string, emit func(Event) error) {
	runID := in.RunID
	if runID == "" {
		runID = uuid.NewString()
	}
	_ = emit(Event{"type": RunStarted, "threadId": in.ThreadID, "runId": runID})
	state, _ := in.State.(map[string]any)
	if state != nil {
		_ = emit(Event{"type": StateSnapshot, "snapshot": state})
	}
	fail := func(err error) { _ = emit(Event{"type": RunError, "message": err.Error()}) }

	st := &streamState{emit: emit}
	var res *component.RunResult
	var err error
	if tools := ExtractToolMessages(in.Messages); len(tools) > 0 {
		res, err = s.resume(ctx, in.ThreadID, userID, tools)
		if err == nil && res.Content != "" {
			st.content(res.Content)
		}
	} else {
		input := ExtractUserInput(in.Messages)
		if input == "" {
			fail(errors.New("no user message to run"))
			return
		}
		meta := map[string]any{}
		if len(in.Context) > 0 {
			meta["agui_context"] = in.Context
		}
		if state != nil {
			meta["session_state"] = state
		}
		res, err = component.Stream(ctx, s.comp, component.RunRequest{Input: input, SessionID: in.ThreadID, UserID: userID, RunID: runID, Metadata: meta}, st.onEvent)
	}
	st.closeText()
	if err != nil {
		fail(err)
		return
	}
	switch res.Status {
	case component.StatusError:
		msg := res.Error
		if msg == "" {
			msg = "Run failed"
		}
		fail(errors.New(msg))
		return
	case component.StatusPaused:
		s.mu.Lock()
		s.paused[in.ThreadID] = &pausedRun{runID: res.RunID, requirements: res.Requirements}
		s.mu.Unlock()
		st.pausedTools(res)
	default:
		s.mu.Lock()
		delete(s.paused, in.ThreadID)
		s.mu.Unlock()
	}
	if state != nil {
		_ = emit(Event{"type": StateSnapshot, "snapshot": state})
	}
	_ = emit(Event{"type": RunFinished, "threadId": in.ThreadID, "runId": runID})
}

// streamState maps component events to AG-UI events (Python handlers.py).
type streamState struct {
	emit      func(Event) error
	messageID string
	streamed  bool
}

func (s *streamState) content(delta string) {
	if s.messageID == "" {
		s.messageID = uuid.NewString()
		_ = s.emit(Event{"type": TextMessageStart, "messageId": s.messageID, "role": "assistant"})
	}
	_ = s.emit(Event{"type": TextMessageContent, "messageId": s.messageID, "delta": delta})
	s.streamed = true
}

func (s *streamState) closeText() {
	if s.messageID != "" {
		_ = s.emit(Event{"type": TextMessageEnd, "messageId": s.messageID})
		s.messageID = ""
	}
}

func (s *streamState) onEvent(ev component.Event) error {
	switch ev.Type {
	case component.EventRunContent:
		if ev.Content != "" {
			s.content(ev.Content)
		}
	case component.EventToolCallStarted:
		if ev.Tool == nil {
			return nil
		}
		parent := s.messageID
		s.closeText()
		id := ev.Tool.ID
		if id == "" {
			id = uuid.NewString()
		}
		start := Event{"type": ToolCallStart, "toolCallId": id, "toolCallName": ev.Tool.Name}
		if parent != "" {
			start["parentMessageId"] = parent
		}
		_ = s.emit(start)
		args, _ := json.Marshal(ev.Tool.Args)
		_ = s.emit(Event{"type": ToolCallArgs, "toolCallId": id, "delta": string(args)})
		_ = s.emit(Event{"type": ToolCallEnd, "toolCallId": id})
	case component.EventToolCallCompleted:
		if ev.Tool != nil && ev.Tool.ID != "" {
			_ = s.emit(Event{"type": ToolCallResult, "messageId": uuid.NewString(), "toolCallId": ev.Tool.ID, "content": ev.Tool.Result, "role": "tool"})
		}
	case component.EventCustom:
		_ = s.emit(Event{"type": Custom, "name": ev.Content, "value": ev.Data})
	}
	return nil
}

// toolOf extracts the tool call of a requirement (or of a paused tool).
func toolOf(r map[string]any) (id, name string, args any) {
	src := r
	if te, ok := r["tool_execution"].(map[string]any); ok {
		src = te
	}
	id, _ = src["tool_call_id"].(string)
	if id == "" {
		id, _ = src["id"].(string)
	}
	name, _ = src["tool_name"].(string)
	if name == "" {
		name, _ = src["name"].(string)
	}
	args = src["tool_args"]
	if args == nil {
		args = src["arguments"]
	}
	return
}

func (s *streamState) pausedTools(res *component.RunResult) {
	if len(res.Requirements) == 0 {
		return
	}
	msgID := uuid.NewString()
	_ = s.emit(Event{"type": TextMessageStart, "messageId": msgID, "role": "assistant"})
	if res.Content != "" && !s.streamed {
		_ = s.emit(Event{"type": TextMessageContent, "messageId": msgID, "delta": res.Content})
	}
	_ = s.emit(Event{"type": TextMessageEnd, "messageId": msgID})
	for _, r := range res.Requirements {
		id, name, args := toolOf(r)
		if id == "" || name == "" {
			continue
		}
		delta, _ := args.(string)
		if delta == "" {
			b, _ := json.Marshal(args)
			delta = string(b)
		}
		_ = s.emit(Event{"type": ToolCallStart, "toolCallId": id, "toolCallName": name, "parentMessageId": msgID})
		_ = s.emit(Event{"type": ToolCallArgs, "toolCallId": id, "delta": delta})
		_ = s.emit(Event{"type": ToolCallEnd, "toolCallId": id})
	}
}

// ResolveRequirements answers paused requirements with the frontend's tool
// messages (Python resolve_requirements_from_tool_messages) through the
// models.RunRequirement API, and fails on a partial resume. The tool message
// content per pause type is:
//
//	confirmation        {"accepted": true|false, "note": "..."}
//	user_input          {"values": {"field": value, ...}}
//	user_feedback       {"selections": {"question": ["label", ...]}}
//	external_execution  the raw result (an error message marks a failed call)
//
// The input maps are not modified; the resolved requirements are returned in
// the component wire form.
func ResolveRequirements(reqs []map[string]any, tools []Message) ([]map[string]any, error) {
	decoded, err := component.DecodeRequirements(reqs)
	if err != nil {
		return nil, err
	}
	byID := map[string]Message{}
	for _, m := range tools {
		byID[m.ToolCallID] = m
	}
	for _, r := range decoded {
		te := r.ToolExecution
		if r.IsResolved() || te == nil || te.ToolCallID == "" {
			continue
		}
		msg, ok := byID[te.ToolCallID]
		if !ok {
			continue
		}
		content := msg.Text()
		if r.PauseType() == models.PauseTypeExternalExecution {
			if msg.Error != "" {
				te.ToolCallError = true
				content = msg.Error
			}
			if err := r.SetExternalExecutionResult(content); err != nil {
				return nil, fmt.Errorf("tool call %s: %w", te.ToolCallID, err)
			}
			continue
		}
		var payload map[string]any
		_ = json.Unmarshal([]byte(content), &payload)
		switch r.PauseType() {
		case models.PauseTypeConfirmation:
			if msg.Error != "" || payload["accepted"] != true {
				note, _ := payload["note"].(string)
				if note == "" {
					note = msg.Error
				}
				err = r.Reject(note)
			} else {
				err = r.Confirm()
			}
		case models.PauseTypeUserInput:
			values, ok := payload["values"].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("user_input expects {'values': {...}} for tool call %s", te.ToolCallID)
			}
			err = r.ProvideUserInput(values)
		case models.PauseTypeUserFeedback:
			raw, ok := payload["selections"].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("user_feedback expects {'selections': {question: [labels]}} for tool call %s", te.ToolCallID)
			}
			selections := map[string][]string{}
			for q, v := range raw {
				list, ok := v.([]any)
				if !ok {
					return nil, fmt.Errorf("user_feedback expects {'selections': {question: [labels]}} for tool call %s", te.ToolCallID)
				}
				labels := []string{}
				for _, l := range list {
					labels = append(labels, fmt.Sprint(l))
				}
				selections[q] = labels
			}
			err = r.ProvideUserFeedback(selections)
		}
		if err != nil {
			return nil, fmt.Errorf("tool call %s: %w", te.ToolCallID, err)
		}
	}
	var unresolved []string
	for _, r := range decoded {
		if !r.IsResolved() && r.ToolExecution != nil {
			unresolved = append(unresolved, r.ToolExecution.ToolCallID)
		}
	}
	if len(unresolved) > 0 {
		return nil, fmt.Errorf("partial resume: requirements %v still unresolved", unresolved)
	}
	return component.EncodeRequirements(decoded), nil
}

func (s *Server) resume(ctx context.Context, threadID, userID string, tools []Message) (*component.RunResult, error) {
	s.mu.Lock()
	p := s.paused[threadID]
	s.mu.Unlock()
	if p == nil {
		return nil, fmt.Errorf("no paused run found for thread %s", threadID)
	}
	reqs, err := ResolveRequirements(p.requirements, tools)
	if err != nil {
		return nil, err
	}
	return component.Continue(ctx, s.comp, component.ContinueRequest{RunID: p.runID, SessionID: threadID, UserID: userID, Requirements: reqs})
}
