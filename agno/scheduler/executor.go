package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// Result is what an Executor reports for one attempt.
type Result struct {
	// Status is one of the RunStatus* constants; empty means success.
	Status       string
	StatusCode   *int
	RunID        string
	SessionID    string
	Error        string
	Input        map[string]any
	Output       map[string]any
	Requirements []any
}

// Executor fires a schedule once. Returning an error marks the attempt failed.
type Executor interface {
	Execute(ctx context.Context, s *Schedule) (*Result, error)
}

// ExecutorFunc adapts a function to Executor.
type ExecutorFunc func(ctx context.Context, s *Schedule) (*Result, error)

// Execute implements Executor.
func (f ExecutorFunc) Execute(ctx context.Context, s *Schedule) (*Result, error) { return f(ctx, s) }

// RunRequest is what a scheduled component run receives.
type RunRequest struct {
	Input     string
	SessionID string
	UserID    string
	// Payload is the schedule payload without the "message"/"session_id" keys.
	Payload map[string]any
}

// RunOutput is a component run's outcome.
type RunOutput struct {
	RunID     string
	SessionID string
	Content   any
	// Status may be RunStatusPaused / RunStatusCancelled; empty means success.
	Status string
}

// Runner is the minimal surface of an agent, team or workflow the scheduler
// needs. Wrap concrete types with RunnerFunc, e.g.
//
//	scheduler.RunnerFunc(func(ctx context.Context, r scheduler.RunRequest) (*scheduler.RunOutput, error) {
//	    resp, err := myAgent.Run(r.Input)
//	    if err != nil { return nil, err }
//	    return &scheduler.RunOutput{Content: resp.TextContent}, nil
//	})
type Runner interface {
	Run(ctx context.Context, req RunRequest) (*RunOutput, error)
}

// RunnerFunc adapts a function to Runner.
type RunnerFunc func(ctx context.Context, req RunRequest) (*RunOutput, error)

// Run implements Runner.
func (f RunnerFunc) Run(ctx context.Context, req RunRequest) (*RunOutput, error) { return f(ctx, req) }

// ComponentExecutor runs schedules in process against registered runners,
// resolving the target from the schedule's run endpoint
// (/agents/{id}/runs, /teams/{id}/runs, /workflows/{id}/runs) or from its
// TargetType/TargetID.
type ComponentExecutor struct {
	mu      sync.RWMutex
	runners map[string]Runner // key "agent:id"
}

// NewComponentExecutor returns an empty executor.
func NewComponentExecutor() *ComponentExecutor {
	return &ComponentExecutor{runners: map[string]Runner{}}
}

// Register adds a runner for targetType ("agent", "team", "workflow") and id.
func (e *ComponentExecutor) Register(targetType, id string, r Runner) *ComponentExecutor {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.runners[targetType+":"+id] = r
	return e
}

// RegisterAgent registers an agent runner.
func (e *ComponentExecutor) RegisterAgent(id string, r Runner) *ComponentExecutor {
	return e.Register("agent", id, r)
}

// RegisterTeam registers a team runner.
func (e *ComponentExecutor) RegisterTeam(id string, r Runner) *ComponentExecutor {
	return e.Register("team", id, r)
}

// RegisterWorkflow registers a workflow runner.
func (e *ComponentExecutor) RegisterWorkflow(id string, r Runner) *ComponentExecutor {
	return e.Register("workflow", id, r)
}

// Execute implements Executor.
func (e *ComponentExecutor) Execute(ctx context.Context, s *Schedule) (*Result, error) {
	tt, tid := strOr(s.TargetType), strOr(s.TargetID)
	if tt == "" || tid == "" {
		var ok bool
		if tt, tid, ok = ParseRunEndpoint(s.Endpoint); !ok {
			return nil, fmt.Errorf("schedule %s endpoint %q is not a component run endpoint", s.ID, s.Endpoint)
		}
	}
	e.mu.RLock()
	r, ok := e.runners[tt+":"+tid]
	e.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no %s registered with id %q", tt, tid)
	}
	req := RunRequest{Payload: map[string]any{}}
	for k, v := range s.Payload {
		switch k {
		case "message", "input":
			req.Input = fmt.Sprint(v)
		case "session_id":
			req.SessionID = fmt.Sprint(v)
		case "user_id":
			// The owner wins over the payload.
		default:
			req.Payload[k] = v
		}
	}
	req.UserID = strOr(s.UserID)
	out, err := r.Run(ctx, req)
	res := &Result{Input: map[string]any{"input_content": req.Input}}
	if err != nil {
		return res, err
	}
	if out != nil {
		res.RunID, res.SessionID, res.Status = out.RunID, out.SessionID, out.Status
		if out.Content != nil {
			res.Output = map[string]any{"content": out.Content}
		}
	}
	return res, nil
}

// HTTPExecutor fires schedules by calling an AgentOS over HTTP, like Python's
// ScheduleExecutor. Run endpoints get a form-encoded POST with stream=false;
// other endpoints get a JSON request with the payload as body.
type HTTPExecutor struct {
	BaseURL string
	// Token is sent as "Authorization: Bearer <Token>" when set.
	Token  string
	Client *http.Client
}

// OwnerHeader carries the schedule owner, as in Python.
const OwnerHeader = "X-Schedule-Owner"

// Execute implements Executor.
func (h *HTTPExecutor) Execute(ctx context.Context, s *Schedule) (*Result, error) {
	client := h.Client
	if client == nil {
		client = http.DefaultClient
	}
	method := strings.ToUpper(s.Method)
	if method == "" {
		method = http.MethodPost
	}
	u := strings.TrimRight(h.BaseURL, "/") + s.Endpoint
	var (
		body        io.Reader
		contentType string
	)
	_, _, isRun := ParseRunEndpoint(s.Endpoint)
	isRun = isRun && method == http.MethodPost
	if isRun {
		form := url.Values{}
		for k, v := range s.Payload {
			if k == "stream" || k == "background" || k == "user_id" {
				continue
			}
			form.Set(k, formValue(v))
		}
		form.Set("stream", "false")
		if s.UserID != nil {
			form.Set("user_id", *s.UserID)
		}
		body, contentType = strings.NewReader(form.Encode()), "application/x-www-form-urlencoded"
	} else if len(s.Payload) > 0 {
		b, err := json.Marshal(s.Payload)
		if err != nil {
			return nil, err
		}
		body, contentType = bytes.NewReader(b), "application/json"
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if h.Token != "" {
		req.Header.Set("Authorization", "Bearer "+h.Token)
	}
	if s.UserID != nil {
		req.Header.Set(OwnerHeader, url.PathEscape(*s.UserID))
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	code := resp.StatusCode
	res := &Result{StatusCode: &code, Status: RunStatusSuccess}
	if code < 200 || code >= 300 {
		res.Status, res.Error = RunStatusFailed, string(raw)
		return res, nil
	}
	if isRun {
		var data map[string]any
		if json.Unmarshal(raw, &data) == nil {
			res.RunID, _ = data["run_id"].(string)
			res.SessionID, _ = data["session_id"].(string)
			if c, ok := data["content"]; ok {
				res.Output = map[string]any{"content": c}
			}
			if st, ok := data["status"].(string); ok {
				switch strings.ToUpper(st) {
				case "PAUSED":
					res.Status = RunStatusPaused
				case "CANCELLED":
					res.Status = RunStatusCancelled
				case "ERROR", "FAILED":
					res.Status = RunStatusFailed
					res.Error, _ = data["error"].(string)
				}
			}
		}
	}
	return res, nil
}

func formValue(v any) string {
	switch t := v.(type) {
	case bool:
		if t {
			return "true"
		}
		return "false"
	case map[string]any, []any:
		b, _ := json.Marshal(t)
		return string(b)
	case string:
		return t
	}
	return fmt.Sprint(v)
}
