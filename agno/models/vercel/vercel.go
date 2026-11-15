// Package vercel provides integration with Vercel v0, which exposes an
// OpenAI-compatible API. It reuses the OpenAI-like implementation internally.
package vercel

import (
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for V0.
	DefaultBaseURL = "https://api.v0.dev/v1/"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "V0_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "v0-1.0-md"
)

// NewV0Chat creates a new V0 chat model. Options override the defaults
// (models.WithBaseURL, models.WithID, models.WithAPIKey, models.WithDefaultHeaders).
func NewV0Chat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return like.NewPreset(like.Preset{
		BaseURL:        DefaultBaseURL,
		EnvAPIKey:      EnvAPIKey,
		DefaultModel:   DefaultModel,
		FallbackAPIKey: "",
	}, options...)
}

// New is an alias of NewV0Chat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewV0Chat(options...)
}
