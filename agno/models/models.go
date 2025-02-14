package models

import "context"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIInterface interface {
	Invoke(ctx context.Context, messages []Message) (*Message, error)
	AInvoke(ctx context.Context, messages []Message) (*Message, error)
	InvokeStream(ctx context.Context, messages []Message) (<-chan Message, error)
	AInvokeStream(ctx context.Context, messages []Message) (<-chan Message, error)
}
