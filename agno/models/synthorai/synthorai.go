// Package synthorai provides integration with Synthorai, which exposes an
// OpenAI-compatible API. It reuses the OpenAI-like implementation internally.
package synthorai

import (
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for Synthorai.
	DefaultBaseURL = "https://synthorai.io/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "SYNTHORAI_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "claude-opus-5"
)

// NewSynthoraiChat creates a new Synthorai chat model. Options override the defaults
// (models.WithBaseURL, models.WithID, models.WithAPIKey, models.WithDefaultHeaders).
func NewSynthoraiChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return like.NewPreset(like.Preset{
		BaseURL:        DefaultBaseURL,
		EnvAPIKey:      EnvAPIKey,
		DefaultModel:   DefaultModel,
		FallbackAPIKey: "",
	}, options...)
}

// New is an alias of NewSynthoraiChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewSynthoraiChat(options...)
}
