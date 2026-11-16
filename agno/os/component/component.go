// Package component is the protocol-neutral runtime surface shared by the
// AgentOS protocol interfaces (MCP server, A2A, AG-UI) and the remote clients.
//
// The interfaces (mcpserver, a2aserver, agui) never touch *agent.Agent,
// *team.Team or *v2.Workflow directly: they operate on a Component, so the same
// code serves local components, remote AgentOS components and remote A2A agents.
// Adapters for the concrete Agno types live in adapters.go.
package component

import (
	"context"
	"errors"
	"strings"
	"sync"
)

// Kind is the kind of a runnable component.
type Kind string

const (
	KindAgent    Kind = "agent"
	KindTeam     Kind = "team"
	KindWorkflow Kind = "workflow"
)

// Plural returns the REST collection name for the kind ("agents", "teams", "workflows").
func (k Kind) Plural() string { return string(k) + "s" }

// ParseKind accepts "agent"/"agents" (and team/workflow equivalents).
func ParseKind(s string) (Kind, bool) {
	switch strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), "s") {
	case "agent":
		return KindAgent, true
	case "team":
		return KindTeam, true
	case "workflow":
		return KindWorkflow, true
	}
	return "", false
}

// Status mirrors Agno's RunStatus values (Python agno.run.base.RunStatus).
type Status string

const (
	StatusPending   Status = "PENDING"
	StatusRunning   Status = "RUNNING"
	StatusCompleted Status = "COMPLETED"
	StatusPaused    Status = "PAUSED"
	StatusCancelled Status = "CANCELLED"
	StatusError     Status = "ERROR"
)

// NormalizeStatus maps the various status spellings used across the Go port
// (workflow "completed"/"failed", agent "PAUSED", ...) to a Status.
func NormalizeStatus(s string) Status {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "", "COMPLETED", "COMPLETE", "SUCCESS", "DONE":
		return StatusCompleted
	case "PAUSED", "INPUT_REQUIRED", "INPUT-REQUIRED":
		return StatusPaused
	case "CANCELLED", "CANCELED":
		return StatusCancelled
	case "ERROR", "FAILED", "FAILURE":
		return StatusError
	case "RUNNING", "WORKING", "STARTED":
		return StatusRunning
	case "PENDING", "QUEUED", "SUBMITTED":
		return StatusPending
	}
	return Status(strings.ToUpper(s))
}

// RunRequest is a protocol-neutral run request.
type RunRequest struct {
	Input     string
	SessionID string
	UserID    string
	// RunID is the run id the caller would like the run to carry. Components
	// that cannot choose their run id ignore it and report their own in RunResult.
	RunID    string
	Metadata map[string]any
}

// ContinueRequest resumes a paused (human-in-the-loop) run.
type ContinueRequest struct {
	RunID     string
	SessionID string
	UserID    string
	// Requirements are the requirements of the paused run, resolved by the
	// caller, in the wire form of models.RunRequirement (see requirements.go),
	// e.g. {"id": "...", "tool_execution": {...}, "confirmation": true}.
	Requirements []map[string]any
}

// RunResult is the outcome of a run.
type RunResult struct {
	RunID       string         `json:"run_id,omitempty"`
	SessionID   string         `json:"session_id,omitempty"`
	UserID      string         `json:"user_id,omitempty"`
	ComponentID string         `json:"component_id,omitempty"`
	Kind        Kind           `json:"kind,omitempty"`
	Status      Status         `json:"status"`
	Content     string         `json:"content,omitempty"`
	Output      any            `json:"output,omitempty"`
	Error       string         `json:"error,omitempty"`
	Metrics     map[string]any `json:"metrics,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	// Requirements are the unresolved requirements of a PAUSED run.
	Requirements []map[string]any `json:"requirements,omitempty"`
	// Raw is the underlying run output (models.RunResponse, WorkflowRunResponse, ...).
	Raw any `json:"-"`
}

// EventType identifies a streamed run event.
type EventType string

const (
	EventRunStarted        EventType = "RunStarted"
	EventRunContent        EventType = "RunContent"
	EventReasoningContent  EventType = "ReasoningContent"
	EventToolCallStarted   EventType = "ToolCallStarted"
	EventToolCallCompleted EventType = "ToolCallCompleted"
	EventRunCompleted      EventType = "RunCompleted"
	EventRunPaused         EventType = "RunPaused"
	EventRunCancelled      EventType = "RunCancelled"
	EventRunError          EventType = "RunError"
	EventCustom            EventType = "CustomEvent"
)

// ToolCall describes a tool call carried by a tool event.
type ToolCall struct {
	ID     string         `json:"tool_call_id,omitempty"`
	Name   string         `json:"tool_name,omitempty"`
	Args   map[string]any `json:"tool_args,omitempty"`
	Result string         `json:"result,omitempty"`
	Error  bool           `json:"tool_call_error,omitempty"`
}

// Event is a streamed run event.
type Event struct {
	Type    EventType      `json:"event"`
	RunID   string         `json:"run_id,omitempty"`
	Content string         `json:"content,omitempty"`
	Tool    *ToolCall      `json:"tool,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

// Component is a runnable agent, team or workflow.
type Component interface {
	ID() string
	Name() string
	Description() string
	Kind() Kind
	Run(ctx context.Context, req RunRequest) (*RunResult, error)
}

// Streamer is implemented by components that stream their output.
// RunStream calls emit for each event and returns the final result.
type Streamer interface {
	RunStream(ctx context.Context, req RunRequest, emit func(Event) error) (*RunResult, error)
}

// Continuer is implemented by components that can resume a paused run.
type Continuer interface {
	ContinueRun(ctx context.Context, req ContinueRequest) (*RunResult, error)
}

// Canceller is implemented by components that can cancel an in-flight run.
type Canceller interface {
	CancelRun(ctx context.Context, runID string) (bool, error)
}

// ErrNotSupported is returned when a component lacks an optional capability.
var ErrNotSupported = errors.New("operation not supported by this component")

// ErrNotFound is returned when a component, run or session does not exist.
var ErrNotFound = errors.New("not found")

// Stream runs c, streaming when it implements Streamer. A component that does
// not stream still produces RunStarted / RunContent / terminal events, so
// protocol layers can treat every component alike.
func Stream(ctx context.Context, c Component, req RunRequest, emit func(Event) error) (*RunResult, error) {
	if emit == nil {
		emit = func(Event) error { return nil }
	}
	if s, ok := c.(Streamer); ok {
		return s.RunStream(ctx, req, emit)
	}
	if err := emit(Event{Type: EventRunStarted, RunID: req.RunID}); err != nil {
		return nil, err
	}
	res, err := c.Run(ctx, req)
	if err != nil {
		_ = emit(Event{Type: EventRunError, RunID: req.RunID, Content: err.Error()})
		return nil, err
	}
	if res.Content != "" {
		if err := emit(Event{Type: EventRunContent, RunID: res.RunID, Content: res.Content}); err != nil {
			return res, err
		}
	}
	return res, emit(TerminalEvent(res))
}

// TerminalEvent returns the event that ends a run with the given result.
func TerminalEvent(res *RunResult) Event {
	ev := Event{RunID: res.RunID, Content: res.Content}
	switch res.Status {
	case StatusPaused:
		ev.Type = EventRunPaused
		ev.Data = map[string]any{"requirements": res.Requirements}
	case StatusCancelled:
		ev.Type = EventRunCancelled
	case StatusError:
		ev.Type = EventRunError
		if res.Error != "" {
			ev.Content = res.Error
		}
	default:
		ev.Type = EventRunCompleted
	}
	return ev
}

// Continue resumes a paused run on c, or returns ErrNotSupported.
func Continue(ctx context.Context, c Component, req ContinueRequest) (*RunResult, error) {
	if k, ok := c.(Continuer); ok {
		return k.ContinueRun(ctx, req)
	}
	return nil, ErrNotSupported
}

// Cancel cancels a run on c, or returns ErrNotSupported.
func Cancel(ctx context.Context, c Component, runID string) (bool, error) {
	if k, ok := c.(Canceller); ok {
		return k.CancelRun(ctx, runID)
	}
	return false, ErrNotSupported
}

// Registry is a concurrency-safe set of components indexed by kind and id.
type Registry struct {
	mu         sync.RWMutex
	components []Component
}

// NewRegistry returns a registry holding cs.
func NewRegistry(cs ...Component) *Registry {
	r := &Registry{}
	for _, c := range cs {
		r.Add(c)
	}
	return r
}

// Add registers c (nil is ignored).
func (r *Registry) Add(c Component) {
	if c == nil {
		return
	}
	r.mu.Lock()
	r.components = append(r.components, c)
	r.mu.Unlock()
}

// All returns every registered component.
func (r *Registry) All() []Component {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]Component(nil), r.components...)
}

// List returns the components of a kind.
func (r *Registry) List(kind Kind) []Component {
	var out []Component
	for _, c := range r.All() {
		if c.Kind() == kind {
			out = append(out, c)
		}
	}
	return out
}

// Get finds a component by kind and id (the name is accepted too, as the REST
// routes do).
func (r *Registry) Get(kind Kind, id string) (Component, bool) {
	list := r.List(kind)
	for _, c := range list {
		if c.ID() == id {
			return c, true
		}
	}
	for _, c := range list {
		if c.Name() == id {
			return c, true
		}
	}
	return nil, false
}
