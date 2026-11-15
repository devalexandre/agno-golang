package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/devalexandre/agno-golang/agno/os/component"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// RemoteAgent is a remote A2A agent usable as a component.Component (so an
// AgentOS can serve it over MCP/A2A/AG-UI) and as an agent tool (AsTool).
// Python: RemoteAgent(base_url, protocol="a2a").
type RemoteAgent struct {
	Client *Client
	id     string
	name   string
	desc   string
	kind   component.Kind
}

// NewRemoteAgent returns a remote agent backed by client. The id defaults to
// the last path segment of the base URL; Name/Description come from the agent
// card when it is reachable (see Refresh).
func NewRemoteAgent(client *Client, id string) *RemoteAgent {
	if id == "" {
		id = client.BaseURL[strings.LastIndex(client.BaseURL, "/")+1:]
	}
	return &RemoteAgent{Client: client, id: id, name: id, kind: component.KindAgent}
}

// Refresh loads the name and description from the agent card.
func (r *RemoteAgent) Refresh(ctx context.Context) error {
	card, err := r.Client.GetAgentCard(ctx)
	if err != nil {
		return err
	}
	if card.Name != "" {
		r.name = card.Name
	}
	r.desc = card.Description
	return nil
}

func (r *RemoteAgent) ID() string           { return r.id }
func (r *RemoteAgent) Name() string         { return r.name }
func (r *RemoteAgent) Description() string  { return r.desc }
func (r *RemoteAgent) Kind() component.Kind { return r.kind }

// TaskResultToRunResult maps an A2A task result to a component.RunResult
// (Python agno.client.a2a.utils.map_task_result_to_run_output).
func TaskResultToRunResult(tr *TaskResult) *component.RunResult {
	res := &component.RunResult{RunID: tr.TaskID, SessionID: tr.ContextID, Content: tr.Content, Metadata: tr.Metadata}
	switch tr.Status {
	case "completed":
		res.Status = component.StatusCompleted
	case "failed", "rejected":
		res.Status = component.StatusError
		res.Error = tr.Content
	case "canceled":
		res.Status = component.StatusCancelled
	case "input-required", "auth-required":
		res.Status = component.StatusPaused
	default:
		res.Status = component.StatusRunning
	}
	if m, ok := tr.Data.(map[string]any); ok {
		for _, key := range []string{"requirements", "step_requirements"} {
			if list, ok := m[key].([]any); ok {
				for _, item := range list {
					if req, ok := item.(map[string]any); ok {
						res.Requirements = append(res.Requirements, req)
					}
				}
			}
		}
	}
	if res.Metadata != nil {
		if u, ok := res.Metadata["userId"].(string); ok {
			res.UserID = u
		}
	}
	return res
}

func (r *RemoteAgent) finish(tr *TaskResult) *component.RunResult {
	res := TaskResultToRunResult(tr)
	res.ComponentID, res.Kind = r.id, r.kind
	return res
}

// Run sends the input as an A2A message; the session id is the A2A context id.
func (r *RemoteAgent) Run(ctx context.Context, req component.RunRequest) (*component.RunResult, error) {
	tr, err := r.Client.SendMessage(ctx, req.Input, SendOptions{ContextID: req.SessionID, UserID: req.UserID, Metadata: req.Metadata})
	if err != nil {
		return nil, err
	}
	return r.finish(tr), nil
}

// RunStream streams the remote task's events.
func (r *RemoteAgent) RunStream(ctx context.Context, req component.RunRequest, emit func(component.Event) error) (*component.RunResult, error) {
	tr, err := r.Client.StreamMessage(ctx, req.Input, SendOptions{ContextID: req.SessionID, UserID: req.UserID, Metadata: req.Metadata}, func(ev StreamEvent) error {
		switch ev.EventType {
		case "started":
			return emit(component.Event{Type: component.EventRunStarted, RunID: ev.TaskID})
		case "content":
			if ev.Raw != nil && ev.Raw.ArtifactUpdate != nil && ev.Raw.ArtifactUpdate.LastChunk {
				return nil // the final chunk repeats the whole answer
			}
			return emit(component.Event{Type: component.EventRunContent, RunID: ev.TaskID, Content: ev.Content})
		case "reasoning":
			return emit(component.Event{Type: component.EventReasoningContent, RunID: ev.TaskID, Content: ev.Content})
		case "tool_call_started", "tool_call_completed":
			t := component.EventToolCallStarted
			if ev.EventType == "tool_call_completed" {
				t = component.EventToolCallCompleted
			}
			name, _ := ev.Metadata["tool_name"].(string)
			id, _ := ev.Metadata["tool_call_id"].(string)
			return emit(component.Event{Type: t, RunID: ev.TaskID, Tool: &component.ToolCall{ID: id, Name: name}})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	res := r.finish(tr)
	return res, emit(component.TerminalEvent(res))
}

// ContinueRun answers a task waiting for input with resolved requirements.
func (r *RemoteAgent) ContinueRun(ctx context.Context, req component.ContinueRequest) (*component.RunResult, error) {
	tr, err := r.Client.SendMessage(ctx, "", SendOptions{TaskID: req.RunID, ContextID: req.SessionID, UserID: req.UserID,
		Data: map[string]any{"requirements": req.Requirements}})
	if err != nil {
		return nil, err
	}
	return r.finish(tr), nil
}

// CancelRun cancels the remote task.
func (r *RemoteAgent) CancelRun(ctx context.Context, runID string) (bool, error) {
	tr, err := r.Client.CancelTask(ctx, runID)
	if err != nil {
		return false, err
	}
	return tr.IsCanceled(), nil
}

// ---------------------------------------------------------------- tool

// RemoteAgentTool exposes a remote A2A agent as an Agno tool, so a local
// agent (or a team leader) can delegate to it.
type RemoteAgentTool struct {
	toolkit.Toolkit
	remote *RemoteAgent
}

// AskParams are the arguments of the tool.
type AskParams struct {
	Message   string `json:"message" description:"The request to send to the remote agent." required:"true"`
	ContextID string `json:"context_id,omitempty" description:"Context id returned by a previous call, to continue that conversation."`
}

// AsTool wraps the remote agent as a toolkit with one method, "ask". name is
// the toolkit name (default "a2a_<id>").
func (r *RemoteAgent) AsTool(name, description string) *RemoteAgentTool {
	if name == "" {
		name = "a2a_" + strings.NewReplacer("-", "_", ".", "_").Replace(r.id)
	}
	if description == "" {
		description = r.desc
	}
	if description == "" {
		description = fmt.Sprintf("Send a request to the remote A2A agent %s and return its answer.", r.name)
	}
	t := &RemoteAgentTool{remote: r}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = name
	t.Toolkit.Description = description
	t.Toolkit.Register("ask", description, t, t.Ask, AskParams{})
	return t
}

// Ask sends the message and returns the answer (with the context id so the
// model can continue the conversation).
func (t *RemoteAgentTool) Ask(params AskParams) (interface{}, error) {
	res, err := t.remote.Run(context.Background(), component.RunRequest{Input: params.Message, SessionID: params.ContextID})
	if err != nil {
		return nil, err
	}
	out := map[string]any{"content": res.Content, "context_id": res.SessionID, "status": string(res.Status)}
	if res.Status == component.StatusPaused {
		out["requirements"] = res.Requirements
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}
