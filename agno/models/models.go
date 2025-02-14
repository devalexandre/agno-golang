package models

import "context"

type Role string

const (
	TypeUserRole      = "user"
	TypeAssistantRole = "assistant"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

func (r Role) IsValid() bool {
	switch r {
	case TypeUserRole, TypeAssistantRole:
		return true
	default:
		return false
	}
}

type OpenAIInterface interface {
	Invoke(ctx context.Context, messages []Message) (*Message, error)
	AInvoke(ctx context.Context, messages []Message) (*Message, error)
	InvokeStream(ctx context.Context, messages []Message) (<-chan Message, error)
	AInvokeStream(ctx context.Context, messages []Message) (<-chan Message, error)
}
