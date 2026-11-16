// Package minimax provides integration with MiniMax, which exposes an
// OpenAI-compatible API. It reuses the OpenAI-like implementation internally.
package minimax

import (
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for MiniMax.
	DefaultBaseURL = "https://api.minimax.io/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "MINIMAX_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "MiniMax-M3"
)

// NewMiniMaxChat creates a new MiniMax chat model. Options override the defaults
// (models.WithBaseURL, models.WithID, models.WithAPIKey, models.WithDefaultHeaders).
func NewMiniMaxChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return like.NewPreset(like.Preset{
		BaseURL:        DefaultBaseURL,
		EnvAPIKey:      EnvAPIKey,
		DefaultModel:   DefaultModel,
		FallbackAPIKey: "",
	}, options...)
}

// New is an alias of NewMiniMaxChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewMiniMaxChat(options...)
}
