// Package nebius provides integration with Nebius Token Factory, which exposes an
// OpenAI-compatible API. It reuses the OpenAI-like implementation internally.
package nebius

import (
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for Nebius.
	DefaultBaseURL = "https://api.tokenfactory.nebius.com/v1/"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "NEBIUS_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "openai/gpt-oss-20b"
)

// NewNebiusChat creates a new Nebius chat model. Options override the defaults
// (models.WithBaseURL, models.WithID, models.WithAPIKey, models.WithDefaultHeaders).
func NewNebiusChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return like.NewPreset(like.Preset{
		BaseURL:        DefaultBaseURL,
		EnvAPIKey:      EnvAPIKey,
		DefaultModel:   DefaultModel,
		FallbackAPIKey: "",
	}, options...)
}

// New is an alias of NewNebiusChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewNebiusChat(options...)
}
