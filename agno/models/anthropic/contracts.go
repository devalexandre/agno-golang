package anthropic

import (
	"context"

	"github.com/devalexandre/agno-golang/agno/models"
)

// Choices represents a choice in the completion response (OpenAI-compatible shape
// used internally by agno-golang, mirroring the other providers).
type Choices struct {
	Index        int                    `json:"index"`
	Message      models.MessageResponse `json:"message"`
	FinishReason string                 `json:"finish_reason"`
	Delta        models.MessageResponse `json:"delta"`
}

// Usage represents token usage information.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// CompletionResponse represents the normalized response to a completion request.
type CompletionResponse struct {
	ID      string    `json:"id"`
	Object  string    `json:"object"`
	Model   string    `json:"model"`
	Choices []Choices `json:"choices"`
	Usage   Usage     `json:"usage"`
}

// ClientInterface defines the interface for communication with the Anthropic API.
type ClientInterface interface {
	CreateChatCompletion(ctx context.Context, messages []models.Message, options ...models.Option) (*CompletionResponse, error)
	StreamChatCompletion(ctx context.Context, messages []models.Message, options ...models.Option) error
}

// Aliases for compatibility with the conventions used by other providers.
type ChatCompletionMessage = models.Message
type ChatCompletionResponse = CompletionResponse
