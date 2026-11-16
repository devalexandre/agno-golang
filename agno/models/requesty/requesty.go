// Package requesty provides integration with the Requesty router, which exposes an
// OpenAI-compatible API. It reuses the OpenAI-like implementation internally.
package requesty

import (
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for Requesty.
	DefaultBaseURL = "https://router.requesty.ai/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "REQUESTY_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "openai/gpt-4.1"
)

// NewRequestyChat creates a new Requesty chat model. Options override the defaults
// (models.WithBaseURL, models.WithID, models.WithAPIKey, models.WithDefaultHeaders).
func NewRequestyChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return like.NewPreset(like.Preset{
		BaseURL:        DefaultBaseURL,
		EnvAPIKey:      EnvAPIKey,
		DefaultModel:   DefaultModel,
		FallbackAPIKey: "",
	}, options...)
}

// New is an alias of NewRequestyChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewRequestyChat(options...)
}
