// Package cometapi provides integration with CometAPI, which exposes an
// OpenAI-compatible API. It reuses the OpenAI-like implementation internally.
package cometapi

import (
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for CometAPI.
	DefaultBaseURL = "https://api.cometapi.com/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "COMETAPI_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "gpt-5-mini"
)

// NewCometAPIChat creates a new CometAPI chat model. Options override the defaults
// (models.WithBaseURL, models.WithID, models.WithAPIKey, models.WithDefaultHeaders).
func NewCometAPIChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return like.NewPreset(like.Preset{
		BaseURL:        DefaultBaseURL,
		EnvAPIKey:      EnvAPIKey,
		DefaultModel:   DefaultModel,
		FallbackAPIKey: "",
	}, options...)
}

// New is an alias of NewCometAPIChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewCometAPIChat(options...)
}
