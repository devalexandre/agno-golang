// Package a2a implements the Agent2Agent (A2A) protocol types and a client
// for remote A2A agents (Python agno/client/a2a). The server side lives in
// agno/os/a2aserver.
//
// The canonical Go types follow the A2A v1.0 JSON-RPC binding (camelCase,
// TASK_STATE_* / ROLE_* enums, oneof-wrapped results). v0.3 payloads
// ("kind"-discriminated, lowercase states) are converted on the wire by
// EncodeV03 / the tolerant decoders, so both protocol versions interoperate.
package a2a

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

// Protocol versions.
const (
	Version10 = "1.0"
	Version03 = "0.3"
	// VersionHeader is sent by v1.0 clients.
	VersionHeader = "A2A-Version"
)

// JSON-RPC method names (v1.0 and their v0.3 equivalents).
const (
	MethodSendMessage          = "SendMessage"
	MethodSendStreamingMessage = "SendStreamingMessage"
	MethodGetTask              = "GetTask"
	MethodListTasks            = "ListTasks"
	MethodCancelTask           = "CancelTask"
	MethodSubscribeToTask      = "SubscribeToTask"

	MethodV03Send      = "message/send"
	MethodV03Stream    = "message/stream"
	MethodV03GetTask   = "tasks/get"
	MethodV03Cancel    = "tasks/cancel"
	MethodV03Subscribe = "tasks/resubscribe"
)

// TaskState is the v1.0 task state.
type TaskState string

const (
	StateUnspecified   TaskState = "TASK_STATE_UNSPECIFIED"
	StateSubmitted     TaskState = "TASK_STATE_SUBMITTED"
	StateWorking       TaskState = "TASK_STATE_WORKING"
	StateCompleted     TaskState = "TASK_STATE_COMPLETED"
	StateFailed        TaskState = "TASK_STATE_FAILED"
	StateCanceled      TaskState = "TASK_STATE_CANCELED"
	StateInputRequired TaskState = "TASK_STATE_INPUT_REQUIRED"
	StateRejected      TaskState = "TASK_STATE_REJECTED"
	StateAuthRequired  TaskState = "TASK_STATE_AUTH_REQUIRED"
)

// IsTerminal reports whether no further updates follow.
func (s TaskState) IsTerminal() bool {
	switch s {
	case StateCompleted, StateFailed, StateCanceled, StateRejected:
		return true
	}
	return false
}

// V03 returns the v0.3 spelling ("input-required").
func (s TaskState) V03() string {
	v := strings.ToLower(strings.TrimPrefix(string(s), "TASK_STATE_"))
	return strings.ReplaceAll(v, "_", "-")
}

// Short is the Agno client status string ("completed", "input-required", ...).
func (s TaskState) Short() string { return s.V03() }

// ParseTaskState accepts both v1.0 and v0.3 spellings.
func ParseTaskState(s string) TaskState {
	if strings.HasPrefix(s, "TASK_STATE_") {
		return TaskState(s)
	}
	if s == "cancelled" {
		s = "canceled"
	}
	return TaskState("TASK_STATE_" + strings.ToUpper(strings.ReplaceAll(s, "-", "_")))
}

// Role is the v1.0 message role.
type Role string

const (
	RoleUser  Role = "ROLE_USER"
	RoleAgent Role = "ROLE_AGENT"
)

// ParseRole accepts both spellings.
func ParseRole(s string) Role {
	switch strings.ToLower(strings.TrimPrefix(s, "ROLE_")) {
	case "agent":
		return RoleAgent
	}
	return RoleUser
}

// Part is one piece of message or artifact content. Exactly one of Text,
// Raw, URL or Data is set.
type Part struct {
	Text      *string        `json:"text,omitempty"`
	Raw       []byte         `json:"raw,omitempty"` // base64 on the wire
	URL       string         `json:"url,omitempty"`
	Data      any            `json:"data,omitempty"`
	MediaType string         `json:"mediaType,omitempty"`
	Filename  string         `json:"filename,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// TextPart builds a text part.
func TextPart(text string) Part { return Part{Text: &text} }

// DataPart builds a structured data part.
func DataPart(data any) Part { return Part{Data: data, MediaType: "application/json"} }

// IsText reports whether p is a text part.
func (p Part) IsText() bool { return p.Text != nil }

// UnmarshalJSON decodes v1.0 and v0.3 parts.
func (p *Part) UnmarshalJSON(b []byte) error {
	var raw struct {
		Kind      string          `json:"kind"`
		Text      *string         `json:"text"`
		Raw       string          `json:"raw"`
		URL       string          `json:"url"`
		Data      json.RawMessage `json:"data"`
		MediaType string          `json:"mediaType"`
		Filename  string          `json:"filename"`
		Metadata  map[string]any  `json:"metadata"`
		File      *struct {
			Bytes    string `json:"bytes"`
			URI      string `json:"uri"`
			MimeType string `json:"mimeType"`
			Name     string `json:"name"`
		} `json:"file"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*p = Part{Text: raw.Text, URL: raw.URL, MediaType: raw.MediaType, Filename: raw.Filename, Metadata: raw.Metadata}
	if raw.Raw != "" {
		dec, err := base64.StdEncoding.DecodeString(raw.Raw)
		if err != nil {
			return err
		}
		p.Raw = dec
	}
	if len(raw.Data) > 0 && string(raw.Data) != "null" {
		var d any
		if err := json.Unmarshal(raw.Data, &d); err != nil {
			return err
		}
		p.Data = d
	}
	if raw.File != nil { // v0.3 file part
		p.URL, p.MediaType, p.Filename = raw.File.URI, raw.File.MimeType, raw.File.Name
		if raw.File.Bytes != "" {
			dec, err := base64.StdEncoding.DecodeString(raw.File.Bytes)
			if err != nil {
				return err
			}
			p.Raw = dec
		}
	}
	return nil
}

// Message is an A2A message.
type Message struct {
	MessageID        string         `json:"messageId"`
	ContextID        string         `json:"contextId,omitempty"`
	TaskID           string         `json:"taskId,omitempty"`
	Role             Role           `json:"role"`
	Parts            []Part         `json:"parts"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	Extensions       []string       `json:"extensions,omitempty"`
	ReferenceTaskIDs []string       `json:"referenceTaskIds,omitempty"`
}

// UnmarshalJSON accepts both role spellings.
func (m *Message) UnmarshalJSON(b []byte) error {
	type alias Message
	var raw struct {
		alias
		Role string `json:"role"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*m = Message(raw.alias)
	m.Role = ParseRole(raw.Role)
	return nil
}

// Text concatenates the text parts.
func (m *Message) Text() string {
	if m == nil {
		return ""
	}
	var parts []string
	for _, p := range m.Parts {
		if p.Text != nil {
			parts = append(parts, *p.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// DataParts returns the structured data parts.
func (m *Message) DataParts() []any {
	if m == nil {
		return nil
	}
	var out []any
	for _, p := range m.Parts {
		if p.Data != nil {
			out = append(out, p.Data)
		}
	}
	return out
}

// TaskStatus is a task's current status.
type TaskStatus struct {
	State     TaskState `json:"state"`
	Message   *Message  `json:"message,omitempty"`
	Timestamp string    `json:"timestamp,omitempty"`
}

// UnmarshalJSON accepts both state spellings.
func (s *TaskStatus) UnmarshalJSON(b []byte) error {
	var raw struct {
		State     string   `json:"state"`
		Message   *Message `json:"message"`
		Timestamp string   `json:"timestamp"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*s = TaskStatus{State: ParseTaskState(raw.State), Message: raw.Message, Timestamp: raw.Timestamp}
	return nil
}

// NewStatus returns a status stamped now.
func NewStatus(state TaskState, msg *Message) TaskStatus {
	return TaskStatus{State: state, Message: msg, Timestamp: time.Now().UTC().Format(time.RFC3339Nano)}
}

// Artifact is an output of a task.
type Artifact struct {
	ArtifactID  string         `json:"artifactId"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Parts       []Part         `json:"parts"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// Task is a unit of work.
type Task struct {
	ID        string         `json:"id"`
	ContextID string         `json:"contextId"`
	Status    TaskStatus     `json:"status"`
	Artifacts []Artifact     `json:"artifacts,omitempty"`
	History   []Message      `json:"history,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// TaskStatusUpdateEvent is a streamed status change.
type TaskStatusUpdateEvent struct {
	TaskID    string         `json:"taskId"`
	ContextID string         `json:"contextId"`
	Status    TaskStatus     `json:"status"`
	Final     bool           `json:"-"` // v0.3 only
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// TaskArtifactUpdateEvent is a streamed artifact chunk.
type TaskArtifactUpdateEvent struct {
	TaskID    string         `json:"taskId"`
	ContextID string         `json:"contextId"`
	Artifact  Artifact       `json:"artifact"`
	Append    bool           `json:"append,omitempty"`
	LastChunk bool           `json:"lastChunk,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// StreamResponse is the v1.0 oneof result of SendMessage / stream events.
type StreamResponse struct {
	Task           *Task                    `json:"task,omitempty"`
	Message        *Message                 `json:"message,omitempty"`
	StatusUpdate   *TaskStatusUpdateEvent   `json:"statusUpdate,omitempty"`
	ArtifactUpdate *TaskArtifactUpdateEvent `json:"artifactUpdate,omitempty"`
}

// UnmarshalJSON decodes v1.0 wrappers and v0.3 "kind" objects.
func (r *StreamResponse) UnmarshalJSON(b []byte) error {
	var probe struct {
		Kind string `json:"kind"`
	}
	_ = json.Unmarshal(b, &probe)
	switch probe.Kind {
	case "task":
		var t Task
		if err := json.Unmarshal(b, &t); err != nil {
			return err
		}
		*r = StreamResponse{Task: &t}
		return nil
	case "message":
		var m Message
		if err := json.Unmarshal(b, &m); err != nil {
			return err
		}
		*r = StreamResponse{Message: &m}
		return nil
	case "status-update":
		var e TaskStatusUpdateEvent
		if err := json.Unmarshal(b, &e); err != nil {
			return err
		}
		var f struct {
			Final bool `json:"final"`
		}
		_ = json.Unmarshal(b, &f)
		e.Final = f.Final
		*r = StreamResponse{StatusUpdate: &e}
		return nil
	case "artifact-update":
		var e TaskArtifactUpdateEvent
		if err := json.Unmarshal(b, &e); err != nil {
			return err
		}
		*r = StreamResponse{ArtifactUpdate: &e}
		return nil
	}
	type alias StreamResponse
	var a alias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	*r = StreamResponse(a)
	return nil
}

// SendMessageConfiguration configures message/send.
type SendMessageConfiguration struct {
	AcceptedOutputModes []string `json:"acceptedOutputModes,omitempty"`
	HistoryLength       *int     `json:"historyLength,omitempty"`
	// Blocking (v0.3, default true) / ReturnImmediately (v1.0).
	Blocking          *bool `json:"blocking,omitempty"`
	ReturnImmediately bool  `json:"returnImmediately,omitempty"`
}

// SendMessageParams are the params of SendMessage / message/send.
type SendMessageParams struct {
	Message       Message                   `json:"message"`
	Configuration *SendMessageConfiguration `json:"configuration,omitempty"`
	Metadata      map[string]any            `json:"metadata,omitempty"`
}

// ---------------------------------------------------------------- agent card

// AgentInterface is one endpoint of an agent.
type AgentInterface struct {
	URL             string `json:"url"`
	ProtocolBinding string `json:"protocolBinding"`
	ProtocolVersion string `json:"protocolVersion"`
}

// AgentCapabilities lists optional features.
type AgentCapabilities struct {
	Streaming         bool `json:"streaming"`
	PushNotifications bool `json:"pushNotifications"`
	ExtendedAgentCard bool `json:"extendedAgentCard"`
}

// AgentSkill describes one skill.
type AgentSkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// AgentProvider names the organization serving an agent.
type AgentProvider struct {
	Organization string `json:"organization"`
	URL          string `json:"url,omitempty"`
}

// AgentCard is the A2A agent card (served at .well-known/agent-card.json).
// With v0.3 compatibility the card also carries URL / PreferredTransport /
// ProtocolVersion.
type AgentCard struct {
	Name                 string                    `json:"name"`
	Description          string                    `json:"description"`
	Version              string                    `json:"version"`
	SupportedInterfaces  []AgentInterface          `json:"supportedInterfaces,omitempty"`
	Capabilities         AgentCapabilities         `json:"capabilities"`
	DefaultInputModes    []string                  `json:"defaultInputModes"`
	DefaultOutputModes   []string                  `json:"defaultOutputModes"`
	Skills               []AgentSkill              `json:"skills"`
	Provider             *AgentProvider            `json:"provider,omitempty"`
	DocumentationURL     string                    `json:"documentationUrl,omitempty"`
	IconURL              string                    `json:"iconUrl,omitempty"`
	SecuritySchemes      map[string]map[string]any `json:"securitySchemes,omitempty"`
	SecurityRequirements []map[string]any          `json:"securityRequirements,omitempty"`

	// v0.3 fields.
	URL                string `json:"url,omitempty"`
	PreferredTransport string `json:"preferredTransport,omitempty"`
	ProtocolVersion    string `json:"protocolVersion,omitempty"`
}

// BearerSecuritySchemes is the security scheme map of an AgentOS that
// authenticates with bearer tokens (Python build_bearer_security_schemes).
func BearerSecuritySchemes(bearerFormat string) map[string]map[string]any {
	scheme := map[string]any{"scheme": "bearer"}
	if bearerFormat != "" {
		scheme["bearerFormat"] = bearerFormat
	}
	return map[string]map[string]any{"bearerAuth": {"httpAuthSecurityScheme": scheme}}
}

// Endpoint returns the URL of the card's preferred JSON-RPC interface and
// the protocol version it speaks (v1.0 preferred).
func (c *AgentCard) Endpoint() (string, string) {
	var v03 string
	for _, i := range c.SupportedInterfaces {
		if !strings.EqualFold(i.ProtocolBinding, "JSONRPC") {
			continue
		}
		if strings.HasPrefix(i.ProtocolVersion, "1") {
			return i.URL, Version10
		}
		if v03 == "" {
			v03 = i.URL
		}
	}
	if v03 != "" {
		return v03, Version03
	}
	return c.URL, Version03
}

// ---------------------------------------------------------------- JSON-RPC

// JSON-RPC and A2A error codes.
const (
	CodeParseError                   = -32700
	CodeInvalidRequest               = -32600
	CodeMethodNotFound               = -32601
	CodeInvalidParams                = -32602
	CodeInternalError                = -32603
	CodeTaskNotFound                 = -32001
	CodeTaskNotCancelable            = -32002
	CodePushNotificationNotSupported = -32003
	CodeUnsupportedOperation         = -32004
	CodeContentTypeNotSupported      = -32005
	CodeInvalidAgentResponse         = -32006
)

// RPCError is a JSON-RPC error.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *RPCError) Error() string { return e.Message }

// RPCRequest is a JSON-RPC request.
type RPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// RPCResponse is a JSON-RPC response.
type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// ---------------------------------------------------------------- v0.3 encoding

func partV03(p Part) map[string]any {
	m := map[string]any{}
	switch {
	case p.Text != nil:
		m["kind"], m["text"] = "text", *p.Text
	case p.Data != nil:
		m["kind"], m["data"] = "data", p.Data
	default:
		f := map[string]any{}
		if len(p.Raw) > 0 {
			f["bytes"] = base64.StdEncoding.EncodeToString(p.Raw)
		} else {
			f["uri"] = p.URL
		}
		if p.MediaType != "" {
			f["mimeType"] = p.MediaType
		}
		if p.Filename != "" {
			f["name"] = p.Filename
		}
		m["kind"], m["file"] = "file", f
	}
	if len(p.Metadata) > 0 {
		m["metadata"] = p.Metadata
	}
	return m
}

func partsV03(ps []Part) []any {
	out := make([]any, 0, len(ps))
	for _, p := range ps {
		out = append(out, partV03(p))
	}
	return out
}

func messageV03(m *Message) map[string]any {
	role := "user"
	if m.Role == RoleAgent {
		role = "agent"
	}
	out := map[string]any{"kind": "message", "messageId": m.MessageID, "role": role, "parts": partsV03(m.Parts)}
	if m.ContextID != "" {
		out["contextId"] = m.ContextID
	}
	if m.TaskID != "" {
		out["taskId"] = m.TaskID
	}
	if len(m.Metadata) > 0 {
		out["metadata"] = m.Metadata
	}
	return out
}

func statusV03(s TaskStatus) map[string]any {
	out := map[string]any{"state": s.State.V03()}
	if s.Message != nil {
		out["message"] = messageV03(s.Message)
	}
	if s.Timestamp != "" {
		out["timestamp"] = s.Timestamp
	}
	return out
}

func artifactV03(a Artifact) map[string]any {
	out := map[string]any{"artifactId": a.ArtifactID, "parts": partsV03(a.Parts)}
	if a.Name != "" {
		out["name"] = a.Name
	}
	if a.Description != "" {
		out["description"] = a.Description
	}
	if len(a.Metadata) > 0 {
		out["metadata"] = a.Metadata
	}
	return out
}

// TaskV03 encodes a task in the v0.3 shape.
func TaskV03(t *Task) map[string]any {
	out := map[string]any{"kind": "task", "id": t.ID, "contextId": t.ContextID, "status": statusV03(t.Status)}
	if len(t.Artifacts) > 0 {
		arts := make([]any, 0, len(t.Artifacts))
		for _, a := range t.Artifacts {
			arts = append(arts, artifactV03(a))
		}
		out["artifacts"] = arts
	}
	if len(t.History) > 0 {
		h := make([]any, 0, len(t.History))
		for i := range t.History {
			h = append(h, messageV03(&t.History[i]))
		}
		out["history"] = h
	}
	if len(t.Metadata) > 0 {
		out["metadata"] = t.Metadata
	}
	return out
}

// EncodeV03 encodes a stream response in the v0.3 shape.
func (r *StreamResponse) EncodeV03() any {
	switch {
	case r.Task != nil:
		return TaskV03(r.Task)
	case r.Message != nil:
		return messageV03(r.Message)
	case r.StatusUpdate != nil:
		e := r.StatusUpdate
		out := map[string]any{"kind": "status-update", "taskId": e.TaskID, "contextId": e.ContextID, "status": statusV03(e.Status), "final": e.Final}
		if len(e.Metadata) > 0 {
			out["metadata"] = e.Metadata
		}
		return out
	case r.ArtifactUpdate != nil:
		e := r.ArtifactUpdate
		out := map[string]any{"kind": "artifact-update", "taskId": e.TaskID, "contextId": e.ContextID, "artifact": artifactV03(e.Artifact), "append": e.Append, "lastChunk": e.LastChunk}
		if len(e.Metadata) > 0 {
			out["metadata"] = e.Metadata
		}
		return out
	}
	return map[string]any{}
}

// MessageV03 encodes a message in the v0.3 shape (used by the client).
func MessageV03(m *Message) map[string]any { return messageV03(m) }
