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

type Message struct {
	Role       Role             `json:"role"`
	Content    string           `json:"content"`
	ToolCallID *string          `json:"tool_call_id,omitempty"`
	ToolCalls  []tools.ToolCall `json:"tool_calls,omitempty"`
}

type MessageResponse struct {
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
	InvokeStream(ctx context.Context, messages []Message, options ...Option) (<-chan *MessageResponse, error)
	AInvokeStream(ctx context.Context, messages []Message, options ...Option) (<-chan *MessageResponse, <-chan error)
}
