// Package llmman provides integration with a local Llmman server, which exposes an
// OpenAI-compatible API. It reuses the OpenAI-like implementation internally.
package llmman

import (
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for Llmman.
	DefaultBaseURL = "http://127.0.0.1:17434/v1"
	// EnvAPIKey is empty: the server does not require authentication.
	EnvAPIKey = ""
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "qwen3:0.6b-q4_K_M"
)

// NewLlmmanChat creates a new Llmman chat model. Options override the defaults
// (models.WithBaseURL, models.WithID, models.WithAPIKey, models.WithDefaultHeaders).
func NewLlmmanChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return like.NewPreset(like.Preset{
		BaseURL:        DefaultBaseURL,
		EnvAPIKey:      EnvAPIKey,
		DefaultModel:   DefaultModel,
		FallbackAPIKey: "not-provided",
	}, options...)
}

// New is an alias of NewLlmmanChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewLlmmanChat(options...)
}
