// Package tokenlab provides integration with TokenLab, which exposes an
// OpenAI-compatible API. It reuses the OpenAI-like implementation internally.
package tokenlab

import (
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for TokenLab.
	DefaultBaseURL = "https://api.tokenlab.sh/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "TOKENLAB_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "gpt-5.4-mini"
)

// NewTokenLabChat creates a new TokenLab chat model. Options override the defaults
// (models.WithBaseURL, models.WithID, models.WithAPIKey, models.WithDefaultHeaders).
func NewTokenLabChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return like.NewPreset(like.Preset{
		BaseURL:        DefaultBaseURL,
		EnvAPIKey:      EnvAPIKey,
		DefaultModel:   DefaultModel,
		FallbackAPIKey: "",
	}, options...)
}

// New is an alias of NewTokenLabChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewTokenLabChat(options...)
}
