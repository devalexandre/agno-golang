// Package deepinfra provides integration with DeepInfra, which exposes an
// OpenAI-compatible API. It reuses the OpenAI-like implementation internally.
package deepinfra

import (
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for DeepInfra.
	DefaultBaseURL = "https://api.deepinfra.com/v1/openai"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "DEEPINFRA_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "meta-llama/Llama-2-70b-chat-hf"
)

// NewDeepInfraChat creates a new DeepInfra chat model. Options override the defaults
// (models.WithBaseURL, models.WithID, models.WithAPIKey, models.WithDefaultHeaders).
func NewDeepInfraChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return like.NewPreset(like.Preset{
		BaseURL:        DefaultBaseURL,
		EnvAPIKey:      EnvAPIKey,
		DefaultModel:   DefaultModel,
		FallbackAPIKey: "",
	}, options...)
}

// New is an alias of NewDeepInfraChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewDeepInfraChat(options...)
}
