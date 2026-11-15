package agentos

import (
	"context"

	"github.com/devalexandre/agno-golang/agno/os/component"
)

// RemoteComponent is an agent, team or workflow served by a remote AgentOS
// (Python RemoteAgent / RemoteTeam / RemoteWorkflow with protocol="agentos").
type RemoteComponent struct {
	Client *Client
	kind   component.Kind
	id     string
	name   string
	desc   string
}

// NewRemote returns a remote component. Name/Description are loaded by Refresh.
func NewRemote(client *Client, kind component.Kind, id string) *RemoteComponent {
	return &RemoteComponent{Client: client, kind: kind, id: id, name: id}
}

// Refresh loads name and description from GET /{kind}s/{id}.
func (r *RemoteComponent) Refresh(ctx context.Context) error {
	info, err := r.Client.Get(ctx, r.kind, r.id)
	if err != nil {
		return err
	}
	if n, _ := info["name"].(string); n != "" {
		r.name = n
	}
	r.desc, _ = info["description"].(string)
	return nil
}

func (r *RemoteComponent) ID() string           { return r.id }
func (r *RemoteComponent) Name() string         { return r.name }
func (r *RemoteComponent) Description() string  { return r.desc }
func (r *RemoteComponent) Kind() component.Kind { return r.kind }

func (r *RemoteComponent) result(out *RunOutput) *component.RunResult {
	res := &component.RunResult{RunID: out.RunID, SessionID: out.SessionID, ComponentID: r.id, Kind: r.kind,
		Status: component.NormalizeStatus(out.Status), Content: out.Text(), Metrics: out.Metrics, Raw: out}
	res.Requirements = out.Requirements
	if res.Status == component.StatusPaused && len(res.Requirements) == 0 {
		res.Requirements = out.Tools
	}
	if res.Status == component.StatusError {
		res.Error = res.Content
	}
	return res
}

func (r *RemoteComponent) Run(ctx context.Context, req component.RunRequest) (*component.RunResult, error) {
	out, err := r.Client.Run(ctx, r.kind, r.id, RunParams{Message: req.Input, SessionID: req.SessionID, UserID: req.UserID})
	if err != nil {
		return nil, err
	}
	res := r.result(out)
	if res.SessionID == "" {
		res.SessionID = req.SessionID
	}
	return res, nil
}

func (r *RemoteComponent) RunStream(ctx context.Context, req component.RunRequest, emit func(component.Event) error) (*component.RunResult, error) {
	out, err := r.Client.RunStream(ctx, r.kind, r.id, RunParams{Message: req.Input, SessionID: req.SessionID, UserID: req.UserID}, func(ev RunEvent) error {
		ce := component.Event{RunID: ev.RunID, Content: ev.Text(), Data: ev.Data}
		switch ev.NormalizedEvent() {
		case "RunStarted":
			ce.Type = component.EventRunStarted
		case "RunContent":
			ce.Type = component.EventRunContent
		case "ToolCallStarted", "ToolCallCompleted":
			ce.Type = component.EventToolCallStarted
			if ev.NormalizedEvent() == "ToolCallCompleted" {
				ce.Type = component.EventToolCallCompleted
			}
			if t, ok := ev.Data["tool"].(map[string]any); ok {
				tc := &component.ToolCall{}
				tc.ID, _ = t["tool_call_id"].(string)
				tc.Name, _ = t["tool_name"].(string)
				tc.Args, _ = t["tool_args"].(map[string]any)
				if res, ok := t["result"].(string); ok {
					tc.Result = res
				}
				ce.Tool = tc
			}
		case "ReasoningContentDelta", "ReasoningStep":
			ce.Type = component.EventReasoningContent
		default:
			return nil // terminal events are emitted once below
		}
		return emit(ce)
	})
	if err != nil {
		return nil, err
	}
	res := r.result(out)
	if res.SessionID == "" {
		res.SessionID = req.SessionID
	}
	return res, emit(component.TerminalEvent(res))
}

func (r *RemoteComponent) ContinueRun(ctx context.Context, req component.ContinueRequest) (*component.RunResult, error) {
	out, err := r.Client.ContinueRun(ctx, r.kind, r.id, req.RunID, ContinueParams{Tools: req.Requirements, SessionID: req.SessionID, UserID: req.UserID})
	if err != nil {
		return nil, err
	}
	return r.result(out), nil
}

func (r *RemoteComponent) CancelRun(ctx context.Context, runID string) (bool, error) {
	if err := r.Client.CancelRun(ctx, r.kind, r.id, runID); err != nil {
		return false, err
	}
	return true, nil
}
