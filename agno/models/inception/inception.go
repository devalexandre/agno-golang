// Package inception provides integration with Inception Labs (Mercury), which exposes an
// OpenAI-compatible API. It reuses the OpenAI-like implementation internally.
package inception

import (
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for Inception.
	DefaultBaseURL = "https://api.inceptionlabs.ai/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "INCEPTION_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "mercury-2"
)

// NewInceptionChat creates a new Inception chat model. Options override the defaults
// (models.WithBaseURL, models.WithID, models.WithAPIKey, models.WithDefaultHeaders).
func NewInceptionChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return like.NewPreset(like.Preset{
		BaseURL:        DefaultBaseURL,
		EnvAPIKey:      EnvAPIKey,
		DefaultModel:   DefaultModel,
		FallbackAPIKey: "",
	}, options...)
}

// New is an alias of NewInceptionChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewInceptionChat(options...)
}
