// Package nexus provides integration with a Nexus AI router, which exposes an
// OpenAI-compatible API. It reuses the OpenAI-like implementation internally.
package nexus

import (
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for Nexus.
	DefaultBaseURL = "http://localhost:8000/llm/v1/"
	// EnvAPIKey is empty: the server does not require authentication.
	EnvAPIKey = ""
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "openai/gpt-4"
)

// NewNexusChat creates a new Nexus chat model. Options override the defaults
// (models.WithBaseURL, models.WithID, models.WithAPIKey, models.WithDefaultHeaders).
func NewNexusChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return like.NewPreset(like.Preset{
		BaseURL:        DefaultBaseURL,
		EnvAPIKey:      EnvAPIKey,
		DefaultModel:   DefaultModel,
		FallbackAPIKey: "not-provided",
	}, options...)
}

// New is an alias of NewNexusChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewNexusChat(options...)
}
