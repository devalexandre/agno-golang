package models

import (
	"context"

	"github.com/devalexandre/agno-golang/agno/tools"
)

type Role string

const (
	TypeSystemRole    = "system"
	TypeUserRole      = "user"
	TypeAssistantRole = "assistant"
	TypeToolRole      = "tool"
)

type contextKey string

const DebugKey contextKey = "debug"
const ShowToolsCallKey contextKey = "showToolsCall"

type Message struct {
	Role       Role             `json:"role"`
	Content    string           `json:"content"`
	ToolCallID *string          `json:"tool_call_id,omitempty"`
	ToolCalls  []tools.ToolCall `json:"tool_calls,omitempty"`
}

type MessageResponse struct {
	Model            string           `json:"model"`
	Role             string           `json:"role"`
	Content          string           `json:"content"`
	ToolCalls        []tools.ToolCall `json:"tool_calls,omitempty"`
	ReasoningContent string           `json:"reasoning_content,omitempty"`
}

func (r Role) IsValid() bool {
	switch r {
	case TypeUserRole, TypeAssistantRole:
		return true
	default:
		return false
	}
}

// AgnoModelInterface represents the interface for integration with language models.
type AgnoModelInterface interface {
	Invoke(ctx context.Context, messages []Message, options ...Option) (*MessageResponse, error)
	AInvoke(ctx context.Context, messages []Message, options ...Option) (<-chan *MessageResponse, <-chan error)
	InvokeStream(ctx context.Context, messages []Message, options ...Option) error
	AInvokeStream(ctx context.Context, messages []Message, options ...Option) (<-chan *MessageResponse, <-chan error)
}

type RunResponse struct {
	TextContent        string                   `json:"text_content,omitempty"`
	ContentType        string                   `json:"content_type,omitempty"`
	Thinking           string                   `json:"thinking,omitempty"`
	Event              string                   `json:"event,omitempty"`
	Messages           []Message                `json:"messages,omitempty"`
	Metrics            map[string]interface{}   `json:"metrics,omitempty"`
	Model              string                   `json:"model,omitempty"`
	RunID              string                   `json:"run_id,omitempty"`
	AgentID            string                   `json:"agent_id,omitempty"`
	SessionID          string                   `json:"session_id,omitempty"`
	WorkflowID         string                   `json:"workflow_id,omitempty"`
	Tools              []map[string]interface{} `json:"tools,omitempty"`
	FormattedToolCalls []string                 `json:"formatted_tool_calls,omitempty"`
	CreatedAt          int64                    `json:"created_at,omitempty"`
	// TODO: implement images, videos, audio, response_audio, citations, extra_data
}
