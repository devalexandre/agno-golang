// Package cerebras provides integration with Cerebras Inference, which exposes an
// OpenAI-compatible API. It reuses the OpenAI-like implementation internally.
package cerebras

import (
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for Cerebras.
	DefaultBaseURL = "https://api.cerebras.ai/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "CEREBRAS_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "gpt-oss-120b"
)

// NewCerebrasChat creates a new Cerebras chat model. Options override the defaults
// (models.WithBaseURL, models.WithID, models.WithAPIKey, models.WithDefaultHeaders).
func NewCerebrasChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return like.NewPreset(like.Preset{
		BaseURL:        DefaultBaseURL,
		EnvAPIKey:      EnvAPIKey,
		DefaultModel:   DefaultModel,
		FallbackAPIKey: "",
	}, options...)
}

// New is an alias of NewCerebrasChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewCerebrasChat(options...)
}
