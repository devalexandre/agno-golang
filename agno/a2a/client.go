package a2a

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TaskResult is the simplified result of a non-streaming message (Python
// agno.client.a2a.schemas.TaskResult).
type TaskResult struct {
	TaskID    string
	ContextID string
	// Status is the short state: "completed", "failed", "canceled",
	// "input-required", "working", ...
	Status    string
	Content   string
	Artifacts []Artifact
	Metadata  map[string]any
	// Data is the structured content (e.g. {"requirements": [...]} of a paused task).
	Data any
	Task *Task
}

func (r *TaskResult) IsCompleted() bool     { return r.Status == "completed" }
func (r *TaskResult) IsFailed() bool        { return r.Status == "failed" }
func (r *TaskResult) IsCanceled() bool      { return r.Status == "canceled" }
func (r *TaskResult) IsInputRequired() bool { return r.Status == "input-required" }

// StreamEvent is a simplified streamed event (Python StreamEvent).
type StreamEvent struct {
	// EventType: "started", "working", "content", "reasoning", "artifact",
	// "tool_call_started", "tool_call_completed", "completed", "failed",
	// "canceled", "input-required", "message".
	EventType string
	Content   string
	TaskID    string
	ContextID string
	Metadata  map[string]any
	IsFinal   bool
	Data      any
	Status    string
	Raw       *StreamResponse
}

// SendOptions are the optional fields of SendMessage / StreamMessage.
type SendOptions struct {
	ContextID string
	UserID    string
	// TaskID continues a task waiting for input.
	TaskID   string
	Metadata map[string]any
	// Data is structured data, e.g. the resolved requirements of a paused task.
	Data    any
	Parts   []Part
	Headers map[string]string
}

// Client talks to one remote A2A agent (Python agno.client.a2a.A2AClient).
// The protocol version and endpoint are negotiated from the agent card.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	Headers    map[string]string

	mu       sync.Mutex
	card     *AgentCard
	endpoint string
	version  string
}

// NewClient returns a client for the agent served at baseURL (e.g.
// "http://localhost:7777/a2a/agents/my-agent").
func NewClient(baseURL string) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), HTTPClient: &http.Client{Timeout: 5 * time.Minute}}
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *Client) setHeaders(req *http.Request, extra map[string]string) {
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	for k, v := range extra {
		req.Header.Set(k, v)
	}
}

// GetAgentCard fetches (and caches) the agent card from
// {BaseURL}/.well-known/agent-card.json.
func (c *Client) GetAgentCard(ctx context.Context) (*AgentCard, error) {
	c.mu.Lock()
	if c.card != nil {
		card := c.card
		c.mu.Unlock()
		return card, nil
	}
	c.mu.Unlock()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/.well-known/agent-card.json", nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req, nil)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("a2a: fetching agent card: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("a2a: agent card: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var card AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, fmt.Errorf("a2a: decoding agent card: %w", err)
	}
	endpoint, version := card.Endpoint()
	// Like Python's _use_base_url: a card advertising a relative or
	// unreachable URL is served from the base URL the client was given.
	if endpoint == "" || !strings.HasPrefix(endpoint, "http") {
		endpoint = c.BaseURL
	}
	c.mu.Lock()
	c.card, c.endpoint, c.version = &card, endpoint, version
	c.mu.Unlock()
	return &card, nil
}

func (c *Client) negotiate(ctx context.Context) (string, string, error) {
	if _, err := c.GetAgentCard(ctx); err != nil {
		return "", "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.endpoint, c.version, nil
}

func (c *Client) buildMessage(text string, o SendOptions) Message {
	m := Message{MessageID: uuid.NewString(), Role: RoleUser, ContextID: o.ContextID, TaskID: o.TaskID, Metadata: map[string]any{}}
	if text != "" {
		m.Parts = append(m.Parts, TextPart(text))
	}
	m.Parts = append(m.Parts, o.Parts...)
	if o.Data != nil {
		m.Parts = append(m.Parts, DataPart(o.Data))
	}
	for k, v := range o.Metadata {
		m.Metadata[k] = v
	}
	if o.UserID != "" {
		m.Metadata["userId"] = o.UserID
	}
	if len(m.Metadata) == 0 {
		m.Metadata = nil
	}
	return m
}

func methodFor(version, v1, v03 string) string {
	if version == Version10 {
		return v1
	}
	return v03
}

func encodeParams(version string, params map[string]any) map[string]any {
	if version == Version10 {
		return params
	}
	if m, ok := params["message"].(Message); ok {
		params["message"] = MessageV03(&m)
	}
	return params
}

func (c *Client) rpc(ctx context.Context, method string, params any, headers map[string]string, stream bool) (*http.Response, string, error) {
	endpoint, version, err := c.negotiate(ctx)
	if err != nil {
		return nil, "", err
	}
	body, err := json.Marshal(RPCRequest{JSONRPC: "2.0", ID: uuid.NewString(), Method: method, Params: mustJSON(params)})
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if version == Version10 {
		req.Header.Set(VersionHeader, Version10)
	}
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	c.setHeaders(req, headers)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("a2a: %w", err)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, "", fmt.Errorf("a2a: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return resp, version, nil
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func (c *Client) call(ctx context.Context, v1, v03 string, params map[string]any, headers map[string]string) (*StreamResponse, error) {
	_, version, err := c.negotiate(ctx)
	if err != nil {
		return nil, err
	}
	resp, _, err := c.rpc(ctx, methodFor(version, v1, v03), encodeParams(version, params), headers, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var rpcResp RPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("a2a: decoding response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, rpcResp.Error
	}
	var out StreamResponse
	if err := json.Unmarshal(rpcResp.Result, &out); err != nil {
		return nil, err
	}
	// GetTask / CancelTask return a bare Task in v1.0.
	if out.Task == nil && out.Message == nil && out.StatusUpdate == nil && out.ArtifactUpdate == nil {
		var t Task
		if json.Unmarshal(rpcResp.Result, &t) == nil && t.ID != "" {
			out.Task = &t
		}
	}
	return &out, nil
}

// SendMessage sends a message and waits for the result.
func (c *Client) SendMessage(ctx context.Context, text string, opts SendOptions) (*TaskResult, error) {
	msg := c.buildMessage(text, opts)
	resp, err := c.call(ctx, MethodSendMessage, MethodV03Send, map[string]any{"message": msg}, opts.Headers)
	if err != nil {
		return nil, err
	}
	return ParseTaskResult(resp), nil
}

// GetTask reads a task.
func (c *Client) GetTask(ctx context.Context, taskID string) (*TaskResult, error) {
	resp, err := c.call(ctx, MethodGetTask, MethodV03GetTask, map[string]any{"id": taskID}, nil)
	if err != nil {
		return nil, err
	}
	return ParseTaskResult(resp), nil
}

// CancelTask cancels a task.
func (c *Client) CancelTask(ctx context.Context, taskID string) (*TaskResult, error) {
	resp, err := c.call(ctx, MethodCancelTask, MethodV03Cancel, map[string]any{"id": taskID}, nil)
	if err != nil {
		return nil, err
	}
	return ParseTaskResult(resp), nil
}

// StreamMessage sends a message and streams its events to fn; the final
// TaskResult is assembled from the stream.
func (c *Client) StreamMessage(ctx context.Context, text string, opts SendOptions, fn func(StreamEvent) error) (*TaskResult, error) {
	msg := c.buildMessage(text, opts)
	_, version, err := c.negotiate(ctx)
	if err != nil {
		return nil, err
	}
	resp, _, err := c.rpc(ctx, methodFor(version, MethodSendStreamingMessage, MethodV03Stream),
		encodeParams(version, map[string]any{"message": msg}), opts.Headers, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	result := &TaskResult{}
	artifacts := map[string]*Artifact{}
	var order []string
	err = ReadSSE(resp.Body, func(data []byte) error {
		var rpcResp RPCResponse
		if err := json.Unmarshal(data, &rpcResp); err != nil {
			return fmt.Errorf("a2a: decoding stream event: %w", err)
		}
		if rpcResp.Error != nil {
			return rpcResp.Error
		}
		var sr StreamResponse
		if err := json.Unmarshal(rpcResp.Result, &sr); err != nil {
			return err
		}
		accumulate(result, &sr, artifacts, &order)
		if fn != nil {
			return fn(ParseStreamEvent(&sr))
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	for _, id := range order {
		result.Artifacts = append(result.Artifacts, *artifacts[id])
	}
	if result.Content == "" {
		for _, a := range result.Artifacts {
			if a.Name == "response" || a.Name == "" {
				result.Content = partsText(a.Parts)
			}
		}
	}
	return result, nil
}

func accumulate(r *TaskResult, sr *StreamResponse, artifacts map[string]*Artifact, order *[]string) {
	switch {
	case sr.Task != nil:
		full := ParseTaskResult(sr)
		*r = *full
		for i := range sr.Task.Artifacts {
			a := sr.Task.Artifacts[i]
			if _, ok := artifacts[a.ArtifactID]; !ok {
				*order = append(*order, a.ArtifactID)
			}
			artifacts[a.ArtifactID] = &a
		}
	case sr.StatusUpdate != nil:
		e := sr.StatusUpdate
		r.TaskID, r.ContextID, r.Status = e.TaskID, e.ContextID, e.Status.State.Short()
		if e.Metadata != nil {
			r.Metadata = e.Metadata
		}
		if e.Status.Message != nil {
			if d := e.Status.Message.DataParts(); len(d) > 0 {
				r.Data = d[0]
			}
			if r.Status != "completed" {
				r.Content = e.Status.Message.Text()
			}
		}
	case sr.ArtifactUpdate != nil:
		e := sr.ArtifactUpdate
		a, ok := artifacts[e.Artifact.ArtifactID]
		if !ok {
			cp := e.Artifact
			cp.Parts = nil
			a = &cp
			artifacts[e.Artifact.ArtifactID] = a
			*order = append(*order, e.Artifact.ArtifactID)
		}
		if e.Append {
			a.Parts = mergeText(a.Parts, e.Artifact.Parts)
		} else {
			a.Parts = append([]Part(nil), e.Artifact.Parts...)
		}
	case sr.Message != nil:
		r.Content = sr.Message.Text()
		r.Status = "completed"
		r.ContextID = sr.Message.ContextID
	}
}

func mergeText(existing, more []Part) []Part {
	if len(existing) == 1 && len(more) == 1 && existing[0].Text != nil && more[0].Text != nil {
		s := *existing[0].Text + *more[0].Text
		return []Part{TextPart(s)}
	}
	return append(existing, more...)
}

func partsText(ps []Part) string {
	var sb strings.Builder
	for _, p := range ps {
		if p.Text != nil {
			sb.WriteString(*p.Text)
		}
	}
	return sb.String()
}

// ParseTaskResult maps a SendMessage / GetTask response to a TaskResult.
func ParseTaskResult(sr *StreamResponse) *TaskResult {
	if sr == nil {
		return &TaskResult{}
	}
	if sr.Message != nil {
		r := &TaskResult{ContextID: sr.Message.ContextID, TaskID: sr.Message.TaskID, Status: "completed", Content: sr.Message.Text()}
		if d := sr.Message.DataParts(); len(d) > 0 {
			r.Data = d[0]
		}
		return r
	}
	t := sr.Task
	if t == nil {
		return &TaskResult{}
	}
	r := &TaskResult{TaskID: t.ID, ContextID: t.ContextID, Status: t.Status.State.Short(), Artifacts: t.Artifacts, Metadata: t.Metadata, Task: t}
	var texts []string
	for _, a := range t.Artifacts {
		if a.Name == "reasoning" {
			continue
		}
		if txt := partsText(a.Parts); txt != "" {
			texts = append(texts, txt)
		}
		for _, p := range a.Parts {
			if p.Data != nil && r.Data == nil {
				r.Data = p.Data
			}
		}
	}
	r.Content = strings.Join(texts, "\n")
	if m := t.Status.Message; m != nil {
		if d := m.DataParts(); len(d) > 0 {
			r.Data = d[0]
		}
		if r.Content == "" || t.Status.State != StateCompleted {
			if txt := m.Text(); txt != "" && t.Status.State != StateCompleted {
				r.Content = txt
			}
		}
	}
	if r.Content == "" && len(t.History) > 0 {
		last := t.History[len(t.History)-1]
		if last.Role == RoleAgent {
			r.Content = last.Text()
		}
	}
	return r
}

// ParseStreamEvent maps a stream response to a simplified event.
func ParseStreamEvent(sr *StreamResponse) StreamEvent {
	ev := StreamEvent{Raw: sr}
	switch {
	case sr.Task != nil:
		ev.EventType, ev.TaskID, ev.ContextID = "started", sr.Task.ID, sr.Task.ContextID
		ev.Status = sr.Task.Status.State.Short()
	case sr.Message != nil:
		ev.EventType, ev.Content, ev.ContextID, ev.IsFinal = "message", sr.Message.Text(), sr.Message.ContextID, true
	case sr.ArtifactUpdate != nil:
		e := sr.ArtifactUpdate
		ev.TaskID, ev.ContextID, ev.Metadata = e.TaskID, e.ContextID, e.Artifact.Metadata
		ev.Content = partsText(e.Artifact.Parts)
		switch e.Artifact.Name {
		case "response":
			ev.EventType = "content"
		case "reasoning":
			ev.EventType = "reasoning"
		default:
			ev.EventType = "artifact"
		}
		for _, p := range e.Artifact.Parts {
			if p.Data != nil {
				ev.Data = p.Data
			}
		}
	case sr.StatusUpdate != nil:
		e := sr.StatusUpdate
		ev.TaskID, ev.ContextID, ev.Metadata = e.TaskID, e.ContextID, e.Metadata
		ev.Status = e.Status.State.Short()
		ev.IsFinal = e.Final || e.Status.State.IsTerminal() || e.Status.State == StateInputRequired
		ev.EventType = ev.Status
		if t, _ := e.Metadata["agno_event_type"].(string); t == "tool_call_started" || t == "tool_call_completed" {
			ev.EventType = t
		}
		if m := e.Status.Message; m != nil {
			ev.Content = m.Text()
			if d := m.DataParts(); len(d) > 0 {
				ev.Data = d[0]
			}
		}
	}
	return ev
}

// ReadSSE reads "data:" payloads from a server-sent event stream.
func ReadSSE(r io.Reader, fn func([]byte) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	var buf bytes.Buffer
	flush := func() error {
		if buf.Len() == 0 {
			return nil
		}
		data := append([]byte(nil), buf.Bytes()...)
		buf.Reset()
		return fn(data)
	}
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			if buf.Len() > 0 {
				buf.WriteByte('\n')
			}
			buf.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return flush()
}
