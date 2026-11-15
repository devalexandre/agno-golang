// Package agentos is a Go client for a remote AgentOS (Python
// agno/client/os.py + agno/remote): list and run agents, teams and workflows,
// stream their events, continue/cancel runs and read sessions. RemoteComponent
// turns a remote agent/team/workflow into a component.Component, so a local
// AgentOS can re-serve it over MCP, A2A or AG-UI.
package agentos

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/os/component"
)

// Client talks to a remote AgentOS over its REST API.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	// Headers are sent with every request (e.g. Authorization).
	Headers map[string]string
}

// New returns a client for the AgentOS at baseURL. A non-empty securityKey
// is sent as a bearer token.
func New(baseURL, securityKey string) *Client {
	c := &Client{BaseURL: strings.TrimRight(baseURL, "/"), HTTPClient: &http.Client{Timeout: 10 * time.Minute}, Headers: map[string]string{}}
	if securityKey != "" {
		c.Headers["Authorization"] = "Bearer " + securityKey
	}
	return c
}

// APIError is a non-2xx answer from the AgentOS.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("agentos: HTTP %d: %s", e.StatusCode, e.Body)
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader, contentType string, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	hc := c.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("agentos: %w", err)
	}
	if resp.StatusCode >= 300 {
		defer resp.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return nil, &APIError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(b))}
	}
	return resp, nil
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	resp, err := c.do(ctx, http.MethodGet, path, nil, "", "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}

// GetConfig returns GET /config.
func (c *Client) GetConfig(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	return out, c.getJSON(ctx, "/config", &out)
}

// ComponentSummary is one entry of /agents, /teams or /workflows.
type ComponentSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// List returns the components of a kind.
func (c *Client) List(ctx context.Context, kind component.Kind) ([]ComponentSummary, error) {
	var raw []map[string]any
	if err := c.getJSON(ctx, "/"+kind.Plural(), &raw); err != nil {
		return nil, err
	}
	out := make([]ComponentSummary, 0, len(raw))
	for _, m := range raw {
		s := ComponentSummary{}
		for _, k := range []string{"id", string(kind) + "_id"} {
			if v, _ := m[k].(string); v != "" && s.ID == "" {
				s.ID = v
			}
		}
		s.Name, _ = m["name"].(string)
		s.Description, _ = m["description"].(string)
		out = append(out, s)
	}
	return out, nil
}

// Get returns GET /{kind}s/{id}.
func (c *Client) Get(ctx context.Context, kind component.Kind, id string) (map[string]any, error) {
	var out map[string]any
	return out, c.getJSON(ctx, "/"+kind.Plural()+"/"+url.PathEscape(id), &out)
}

// RunParams are the form fields of a run.
type RunParams struct {
	Message   string
	SessionID string
	UserID    string
	// Extra are additional form fields (e.g. "session_state" as JSON).
	Extra map[string]string
}

func (p RunParams) form(stream bool) url.Values {
	v := url.Values{}
	v.Set("message", p.Message)
	v.Set("stream", strconv.FormatBool(stream))
	if p.SessionID != "" {
		v.Set("session_id", p.SessionID)
	}
	if p.UserID != "" {
		v.Set("user_id", p.UserID)
	}
	for k, x := range p.Extra {
		v.Set(k, x)
	}
	return v
}

// RunOutput is a (subset of a) run output.
type RunOutput struct {
	RunID        string           `json:"run_id"`
	SessionID    string           `json:"session_id"`
	AgentID      string           `json:"agent_id,omitempty"`
	TeamID       string           `json:"team_id,omitempty"`
	WorkflowID   string           `json:"workflow_id,omitempty"`
	Content      any              `json:"content"`
	Status       string           `json:"status,omitempty"`
	Requirements []map[string]any `json:"requirements,omitempty"`
	Tools        []map[string]any `json:"tools,omitempty"`
	Metrics      map[string]any   `json:"metrics,omitempty"`
	Raw          map[string]any   `json:"-"`
}

// Text returns the content as text.
func (o *RunOutput) Text() string {
	switch v := o.Content.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func decodeOutput(b []byte) (*RunOutput, error) {
	var out RunOutput
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	_ = json.Unmarshal(b, &out.Raw)
	if out.Content == nil {
		if tc, ok := out.Raw["text_content"].(string); ok {
			out.Content = tc
		}
	}
	return &out, nil
}

// Run runs a component and waits for the output (stream=false).
func (c *Client) Run(ctx context.Context, kind component.Kind, id string, p RunParams) (*RunOutput, error) {
	resp, err := c.do(ctx, http.MethodPost, "/"+kind.Plural()+"/"+url.PathEscape(id)+"/runs",
		strings.NewReader(p.form(false).Encode()), "application/x-www-form-urlencoded", "application/json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return decodeOutput(b)
}

// RunEvent is a streamed run event ({"event": "RunContent", ...}).
type RunEvent struct {
	Event   string         `json:"event"`
	RunID   string         `json:"run_id,omitempty"`
	Content any            `json:"content,omitempty"`
	Data    map[string]any `json:"-"`
}

// NormalizedEvent strips the Team/Workflow prefixes Python uses
// ("TeamRunContent" -> "RunContent", "WorkflowCompleted" -> "RunCompleted").
func (e RunEvent) NormalizedEvent() string {
	ev := strings.TrimPrefix(e.Event, "Team")
	switch ev {
	case "WorkflowStarted":
		return "RunStarted"
	case "WorkflowCompleted":
		return "RunCompleted"
	case "WorkflowError":
		return "RunError"
	case "WorkflowCancelled":
		return "RunCancelled"
	case "WorkflowPaused":
		return "RunPaused"
	}
	return ev
}

// Text returns the event content as text.
func (e RunEvent) Text() string {
	switch v := e.Content.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// RunStream runs a component with stream=true, calling fn per event; it
// returns the output assembled from the stream.
func (c *Client) RunStream(ctx context.Context, kind component.Kind, id string, p RunParams, fn func(RunEvent) error) (*RunOutput, error) {
	resp, err := c.do(ctx, http.MethodPost, "/"+kind.Plural()+"/"+url.PathEscape(id)+"/runs",
		strings.NewReader(p.form(true).Encode()), "application/x-www-form-urlencoded", "text/event-stream")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return consumeStream(resp.Body, fn)
}

func consumeStream(r io.Reader, fn func(RunEvent) error) (*RunOutput, error) {
	out := &RunOutput{Status: "COMPLETED"}
	var sb strings.Builder
	completed := false
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	var buf bytes.Buffer
	handle := func() error {
		if buf.Len() == 0 {
			return nil
		}
		data := buf.Bytes()
		defer buf.Reset()
		var ev RunEvent
		if err := json.Unmarshal(data, &ev); err != nil {
			return nil // ignore keep-alives / non-JSON payloads
		}
		_ = json.Unmarshal(data, &ev.Data)
		if ev.RunID != "" {
			out.RunID = ev.RunID
		}
		if sid, _ := ev.Data["session_id"].(string); sid != "" {
			out.SessionID = sid
		}
		switch ev.NormalizedEvent() {
		case "RunContent":
			sb.WriteString(ev.Text())
		case "RunCompleted":
			completed = true
			if t := ev.Text(); t != "" {
				out.Content = t
			}
		case "RunError":
			out.Status = "ERROR"
			out.Content = ev.Text()
		case "RunCancelled":
			out.Status = "CANCELLED"
		case "RunPaused":
			out.Status = "PAUSED"
			for _, key := range []string{"requirements", "active_requirements", "tools"} {
				if list, ok := ev.Data[key].([]any); ok {
					for _, it := range list {
						if m, ok := it.(map[string]any); ok {
							out.Requirements = append(out.Requirements, m)
						}
					}
					if len(out.Requirements) > 0 {
						break
					}
				}
			}
		}
		if fn != nil {
			return fn(ev)
		}
		return nil
	}
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			if err := handle(); err != nil {
				return out, err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			buf.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := sc.Err(); err != nil {
		return out, err
	}
	if err := handle(); err != nil {
		return out, err
	}
	if out.Content == nil || (!completed && out.Status == "COMPLETED") {
		out.Content = sb.String()
	}
	return out, nil
}

// ContinueParams resumes a paused run.
type ContinueParams struct {
	// Tools are the paused tool executions / requirements, resolved.
	Tools     []map[string]any
	SessionID string
	UserID    string
}

// ContinueRun calls POST /{kind}s/{id}/runs/{run_id}/continue (stream=false).
func (c *Client) ContinueRun(ctx context.Context, kind component.Kind, id, runID string, p ContinueParams) (*RunOutput, error) {
	v := url.Values{}
	tools, _ := json.Marshal(p.Tools)
	v.Set("tools", string(tools))
	v.Set("stream", "false")
	if p.SessionID != "" {
		v.Set("session_id", p.SessionID)
	}
	if p.UserID != "" {
		v.Set("user_id", p.UserID)
	}
	resp, err := c.do(ctx, http.MethodPost, "/"+kind.Plural()+"/"+url.PathEscape(id)+"/runs/"+url.PathEscape(runID)+"/continue",
		strings.NewReader(v.Encode()), "application/x-www-form-urlencoded", "application/json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(strings.TrimSpace(string(b)), "event:") || strings.HasPrefix(strings.TrimSpace(string(b)), "data:") {
		return consumeStream(bytes.NewReader(b), nil)
	}
	return decodeOutput(b)
}

// CancelRun calls POST /{kind}s/{id}/runs/{run_id}/cancel.
func (c *Client) CancelRun(ctx context.Context, kind component.Kind, id, runID string) error {
	resp, err := c.do(ctx, http.MethodPost, "/"+kind.Plural()+"/"+url.PathEscape(id)+"/runs/"+url.PathEscape(runID)+"/cancel", nil, "", "application/json")
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// SessionsQuery filters GetSessions.
type SessionsQuery struct {
	Type        string // agent | team | workflow
	ComponentID string
	UserID      string
	DBID        string
	Limit       int
	Page        int
}

// GetSessions returns GET /sessions (the paginated {data, meta} payload).
func (c *Client) GetSessions(ctx context.Context, q SessionsQuery) (map[string]any, error) {
	v := url.Values{}
	set := func(k, x string) {
		if x != "" {
			v.Set(k, x)
		}
	}
	set("type", q.Type)
	set("component_id", q.ComponentID)
	set("user_id", q.UserID)
	set("db_id", q.DBID)
	if q.Limit > 0 {
		v.Set("limit", strconv.Itoa(q.Limit))
	}
	if q.Page > 0 {
		v.Set("page", strconv.Itoa(q.Page))
	}
	var out map[string]any
	return out, c.getJSON(ctx, "/sessions?"+v.Encode(), &out)
}

// GetSessionRuns returns GET /sessions/{id}/runs.
func (c *Client) GetSessionRuns(ctx context.Context, sessionID, sessionType, userID string) (any, error) {
	v := url.Values{}
	if sessionType != "" {
		v.Set("type", sessionType)
	}
	if userID != "" {
		v.Set("user_id", userID)
	}
	var out any
	return out, c.getJSON(ctx, "/sessions/"+url.PathEscape(sessionID)+"/runs?"+v.Encode(), &out)
}
