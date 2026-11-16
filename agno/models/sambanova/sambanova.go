// Package sambanova provides integration with SambaNova Cloud, which exposes an
// OpenAI-compatible API. It reuses the OpenAI-like implementation internally.
package sambanova

import (
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for Sambanova.
	DefaultBaseURL = "https://api.sambanova.ai/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "SAMBANOVA_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "Meta-Llama-3.1-8B-Instruct"
)

// NewSambanovaChat creates a new Sambanova chat model. Options override the defaults
// (models.WithBaseURL, models.WithID, models.WithAPIKey, models.WithDefaultHeaders).
func NewSambanovaChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return like.NewPreset(like.Preset{
		BaseURL:        DefaultBaseURL,
		EnvAPIKey:      EnvAPIKey,
		DefaultModel:   DefaultModel,
		FallbackAPIKey: "",
	}, options...)
}

// New is an alias of NewSambanovaChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewSambanovaChat(options...)
}
