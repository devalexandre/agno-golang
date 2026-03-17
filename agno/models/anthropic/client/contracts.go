package client

import (
	"context"

	"github.com/devalexandre/agno-golang/agno/models"
)

type AnthropicRequest struct {
	Model         string           `json:"model"`
	Messages      []models.Message `json:"messages"`
	System        string           `json:"system,omitempty"`
	MaxTokens     int              `json:"max_tokens"`
	StopSequences []string         `json:"stop_sequences,omitempty"`
	Stream        bool             `json:"stream,omitempty"`
	Temperature   *float32         `json:"temperature,omitempty"`
	TopP          *float32         `json:"top_p,omitempty"`
	TopK          *int             `json:"top_k,omitempty"`
	Metadata      interface{}      `json:"metadata,omitempty"`
}

type ContentBlock struct {
	Type  string      `json:"type"`
	Text  string      `json:"text,omitempty"`
	ID    string      `json:"id,omitempty"`
	Name  string      `json:"name,omitempty"`
	Input interface{} `json:"input,omitempty"`
}

type AnthropicResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	StopSequence string         `json:"stop_sequence"`
	Usage        Usage          `json:"usage"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type ClientInterface interface {
	CreateMessage(ctx context.Context, messages []models.Message, options ...models.Option) (*AnthropicResponse, error)
	StreamMessage(ctx context.Context, messages []models.Message, options ...models.Option) error
}
