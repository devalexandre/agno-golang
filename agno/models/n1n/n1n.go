// Package n1n provides integration with N1N, which exposes an
// OpenAI-compatible API. It reuses the OpenAI-like implementation internally.
package n1n

import (
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for N1N.
	DefaultBaseURL = "https://api.n1n.ai/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "N1N_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "gpt-4o"
)

// NewN1NChat creates a new N1N chat model. Options override the defaults
// (models.WithBaseURL, models.WithID, models.WithAPIKey, models.WithDefaultHeaders).
func NewN1NChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return like.NewPreset(like.Preset{
		BaseURL:        DefaultBaseURL,
		EnvAPIKey:      EnvAPIKey,
		DefaultModel:   DefaultModel,
		FallbackAPIKey: "",
	}, options...)
}

// New is an alias of NewN1NChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewN1NChat(options...)
}
