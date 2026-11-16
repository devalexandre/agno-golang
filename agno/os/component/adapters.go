package component

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/team"
	v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
	"github.com/google/uuid"
)

// DeterministicID reproduces the id scheme the AgentOS REST routes use for
// components ("agent_<hash>"), so MCP/A2A/AG-UI ids match /agents/{id}.
func DeterministicID(prefix, input string) string {
	hash := 0
	for _, char := range input {
		hash = int(char) + ((hash << 5) - hash)
	}
	if hash < 0 {
		hash = -hash
	}
	return fmt.Sprintf("%s_%x", prefix, hash%0xFFFFFF)
}

// FromAgents, FromTeams and FromWorkflows adapt slices of concrete components.
func FromAgents(agents []*agent.Agent) []Component {
	out := make([]Component, 0, len(agents))
	for _, a := range agents {
		if a != nil {
			out = append(out, FromAgent(a))
		}
	}
	return out
}

func FromTeams(teams []*team.Team) []Component {
	out := make([]Component, 0, len(teams))
	for _, t := range teams {
		if t != nil {
			out = append(out, FromTeam(t))
		}
	}
	return out
}

func FromWorkflows(workflows []*v2.Workflow) []Component {
	out := make([]Component, 0, len(workflows))
	for _, w := range workflows {
		if w != nil {
			out = append(out, FromWorkflow(w))
		}
	}
	return out
}

// RegistryFrom builds a registry from concrete Agno components.
func RegistryFrom(agents []*agent.Agent, teams []*team.Team, workflows []*v2.Workflow) *Registry {
	r := NewRegistry()
	for _, c := range FromAgents(agents) {
		r.Add(c)
	}
	for _, c := range FromTeams(teams) {
		r.Add(c)
	}
	for _, c := range FromWorkflows(workflows) {
		r.Add(c)
	}
	return r
}

// ----------------------------------------------------------------------------
// Agent

// AgentComponent adapts *agent.Agent. Runs are serialized per agent because
// Agent.Run mutates the agent's session and user ids.
type AgentComponent struct {
	agent *agent.Agent
	id    string
	mu    sync.Mutex
}

// FromAgent adapts an agent.
func FromAgent(a *agent.Agent) *AgentComponent {
	return &AgentComponent{agent: a, id: DeterministicID("agent", a.GetName())}
}

// Agent returns the wrapped agent.
func (c *AgentComponent) Agent() *agent.Agent { return c.agent }

func (c *AgentComponent) ID() string          { return c.id }
func (c *AgentComponent) Name() string        { return c.agent.GetName() }
func (c *AgentComponent) Kind() Kind          { return KindAgent }
func (c *AgentComponent) Description() string { return unexportedString(c.agent, "description") }

func agentRunOptions(req RunRequest) []interface{} {
	// Passed as the unnamed func type: Agent.Run type-asserts func(*RunOptions).
	var opts []interface{}
	if req.SessionID != "" {
		opts = append(opts, (func(*agent.RunOptions))(agent.WithSessionID(req.SessionID)))
	}
	if req.UserID != "" {
		opts = append(opts, (func(*agent.RunOptions))(agent.WithUserID(req.UserID)))
	}
	if len(req.Metadata) > 0 {
		opts = append(opts, (func(*agent.RunOptions))(agent.WithMetadata(req.Metadata)))
	}
	return opts
}

func (c *AgentComponent) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	resp, err := c.agent.Run(req.Input, agentRunOptions(req)...)
	if err != nil {
		return nil, err
	}
	return c.result(resp, req), nil
}

// RunStream streams the agent's model output. Agent.RunStream cannot carry a
// session or user id, so a request naming either runs through Run instead
// (session continuity wins over token streaming).
func (c *AgentComponent) RunStream(ctx context.Context, req RunRequest, emit func(Event) error) (*RunResult, error) {
	if req.SessionID != "" || req.UserID != "" {
		return streamViaRun(ctx, c, req, emit)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	runID := req.RunID
	if runID == "" {
		runID = uuid.NewString()
	}
	if err := emit(Event{Type: EventRunStarted, RunID: runID}); err != nil {
		return nil, err
	}
	var sb strings.Builder
	err := c.agent.RunStream(req.Input, func(chunk []byte) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		sb.Write(chunk)
		return emit(Event{Type: EventRunContent, RunID: runID, Content: string(chunk)})
	})
	if err != nil {
		_ = emit(Event{Type: EventRunError, RunID: runID, Content: err.Error()})
		return nil, err
	}
	res := &RunResult{RunID: runID, SessionID: req.SessionID, UserID: req.UserID, ComponentID: c.id, Kind: KindAgent, Status: StatusCompleted, Content: sb.String()}
	return res, emit(TerminalEvent(res))
}

// ContinueRun resumes a paused run with Agent.ContinueRun. The requirements
// are the wire form documented in requirements.go.
func (c *AgentComponent) ContinueRun(ctx context.Context, req ContinueRequest) (*RunResult, error) {
	reqs, err := DecodeRequirements(req.Requirements)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	resp, err := c.agent.ContinueRun(ctx, models.RunResponse{RunID: req.RunID, SessionID: req.SessionID, Requirements: reqs})
	if err != nil {
		return nil, err
	}
	res := c.result(resp, RunRequest{SessionID: req.SessionID, UserID: req.UserID, RunID: req.RunID})
	return res, nil
}

func (c *AgentComponent) CancelRun(_ context.Context, runID string) (bool, error) {
	return c.agent.CancelRun(runID), nil
}

func (c *AgentComponent) result(resp models.RunResponse, req RunRequest) *RunResult {
	res := FromRunResponse(resp)
	res.ComponentID = c.id
	res.Kind = KindAgent
	fillIDs(res, req)
	return res
}

// ----------------------------------------------------------------------------
// Team

// TeamComponent adapts *team.Team.
type TeamComponent struct {
	team *team.Team
	id   string
	mu   sync.Mutex
}

// FromTeam adapts a team.
func FromTeam(t *team.Team) *TeamComponent {
	return &TeamComponent{team: t, id: DeterministicID("team", t.GetName())}
}

func (c *TeamComponent) ID() string          { return c.id }
func (c *TeamComponent) Name() string        { return c.team.GetName() }
func (c *TeamComponent) Kind() Kind          { return KindTeam }
func (c *TeamComponent) Description() string { return unexportedString(c.team, "description") }

func (c *TeamComponent) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	resp, err := c.team.Run(req.Input)
	if err != nil {
		return nil, err
	}
	res := FromRunResponse(resp)
	res.ComponentID = c.id
	res.Kind = KindTeam
	fillIDs(res, req)
	return res, nil
}

// ContinueRun resumes a paused team run with Team.ContinueRun.
func (c *TeamComponent) ContinueRun(ctx context.Context, req ContinueRequest) (*RunResult, error) {
	reqs, err := DecodeRequirements(req.Requirements)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	resp, err := c.team.ContinueRun(ctx, models.RunResponse{RunID: req.RunID, SessionID: req.SessionID, Requirements: reqs})
	if err != nil {
		return nil, err
	}
	res := FromRunResponse(resp)
	res.ComponentID = c.id
	res.Kind = KindTeam
	fillIDs(res, RunRequest{SessionID: req.SessionID, UserID: req.UserID, RunID: req.RunID})
	return res, nil
}

// ----------------------------------------------------------------------------
// Workflow

// WorkflowComponent adapts *v2.Workflow.
type WorkflowComponent struct {
	workflow *v2.Workflow
	id       string
	mu       sync.Mutex
}

// FromWorkflow adapts a workflow.
func FromWorkflow(w *v2.Workflow) *WorkflowComponent {
	return &WorkflowComponent{workflow: w, id: DeterministicID("workflow", w.Name)}
}

func (c *WorkflowComponent) ID() string          { return c.id }
func (c *WorkflowComponent) Name() string        { return c.workflow.Name }
func (c *WorkflowComponent) Kind() Kind          { return KindWorkflow }
func (c *WorkflowComponent) Description() string { return c.workflow.Description }

func (c *WorkflowComponent) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if req.SessionID != "" {
		c.workflow.SessionID = req.SessionID
	}
	if req.UserID != "" {
		c.workflow.UserID = req.UserID
	}
	resp, err := c.workflow.Run(ctx, req.Input)
	if err != nil {
		return nil, err
	}
	res := &RunResult{ComponentID: c.id, Kind: KindWorkflow, Status: StatusCompleted, Raw: resp}
	if resp != nil {
		res.RunID = resp.RunID
		res.Status = NormalizeStatus(string(resp.Status))
		res.Metadata = resp.Metadata
		switch v := resp.Content.(type) {
		case nil:
		case string:
			res.Content = v
		default:
			res.Output = v
			if b, err := json.Marshal(v); err == nil {
				res.Content = string(b)
			} else {
				res.Content = fmt.Sprint(v)
			}
		}
	}
	fillIDs(res, req)
	return res, nil
}

// ----------------------------------------------------------------------------
// Func

// Func is a component built from plain functions: handy for tests, custom
// runtimes and wrapping remote services. Nil optional funcs report
// ErrNotSupported.
type Func struct {
	IDValue          string
	NameValue        string
	DescriptionValue string
	KindValue        Kind
	RunFunc          func(ctx context.Context, req RunRequest) (*RunResult, error)
	StreamFunc       func(ctx context.Context, req RunRequest, emit func(Event) error) (*RunResult, error)
	ContinueFunc     func(ctx context.Context, req ContinueRequest) (*RunResult, error)
	CancelFunc       func(ctx context.Context, runID string) (bool, error)
}

func (f *Func) ID() string { return f.IDValue }
func (f *Func) Name() string {
	if f.NameValue == "" {
		return f.IDValue
	}
	return f.NameValue
}
func (f *Func) Description() string { return f.DescriptionValue }
func (f *Func) Kind() Kind {
	if f.KindValue == "" {
		return KindAgent
	}
	return f.KindValue
}

func (f *Func) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	if f.RunFunc == nil {
		return nil, ErrNotSupported
	}
	res, err := f.RunFunc(ctx, req)
	if res != nil {
		if res.ComponentID == "" {
			res.ComponentID = f.IDValue
		}
		if res.Kind == "" {
			res.Kind = f.Kind()
		}
		fillIDs(res, req)
	}
	return res, err
}

func (f *Func) RunStream(ctx context.Context, req RunRequest, emit func(Event) error) (*RunResult, error) {
	if f.StreamFunc == nil {
		return streamViaRun(ctx, f, req, emit)
	}
	return f.StreamFunc(ctx, req, emit)
}

func (f *Func) ContinueRun(ctx context.Context, req ContinueRequest) (*RunResult, error) {
	if f.ContinueFunc == nil {
		return nil, ErrNotSupported
	}
	return f.ContinueFunc(ctx, req)
}

func (f *Func) CancelRun(ctx context.Context, runID string) (bool, error) {
	if f.CancelFunc == nil {
		return false, ErrNotSupported
	}
	return f.CancelFunc(ctx, runID)
}

// ----------------------------------------------------------------------------
// helpers

// streamViaRun is the non-streaming fallback of Stream, usable by components
// whose RunStream must sometimes decline to stream.
func streamViaRun(ctx context.Context, c Component, req RunRequest, emit func(Event) error) (*RunResult, error) {
	type runOnly struct{ Component }
	return Stream(ctx, runOnly{c}, req, emit)
}

func fillIDs(res *RunResult, req RunRequest) {
	if res.RunID == "" {
		res.RunID = req.RunID
	}
	if res.RunID == "" {
		res.RunID = uuid.NewString()
	}
	if res.SessionID == "" {
		res.SessionID = req.SessionID
	}
	if res.UserID == "" {
		res.UserID = req.UserID
	}
	if res.Status == "" {
		res.Status = StatusCompleted
	}
}

// FromRunResponse converts a models.RunResponse. A PAUSED run carries its
// unresolved requirements (RunResponse.ActiveRequirements) in wire form; a
// completed run carries none, even when it went through a pause.
func FromRunResponse(resp models.RunResponse) *RunResult {
	res := &RunResult{
		RunID:     resp.RunID,
		SessionID: resp.SessionID,
		Content:   resp.TextContent,
		Output:    resp.Output,
		Metrics:   resp.Metrics,
		Status:    StatusCompleted,
		Raw:       resp,
	}
	if res.Content == "" && resp.Output != nil {
		if b, err := json.Marshal(resp.Output); err == nil {
			res.Content = string(b)
		}
	}
	if resp.Status != "" {
		res.Status = NormalizeStatus(string(resp.Status))
	}
	if resp.IsPaused() || resp.Event == models.RunEventPaused {
		res.Status = StatusPaused
		res.Requirements = EncodeRequirements(resp.ActiveRequirements())
	}
	return res
}

func toMaps(v any) []map[string]any {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out []map[string]any
	if json.Unmarshal(b, &out) != nil {
		return nil
	}
	return out
}

// unexportedString reads an unexported string field (e.g. Agent.description,
// which has no getter) without unsafe tricks: reflect may read but not set it.
func unexportedString(ptr any, field string) string {
	v := reflect.ValueOf(ptr)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return ""
	}
	f := v.FieldByName(field)
	if !f.IsValid() || f.Kind() != reflect.String {
		return ""
	}
	return f.String()
}
