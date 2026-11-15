// Package neosantara provides integration with Neosantara, which exposes an
// OpenAI-compatible API. It reuses the OpenAI-like implementation internally.
package neosantara

import (
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for Neosantara.
	DefaultBaseURL = "https://api.neosantara.xyz/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "NEOSANTARA_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "grok-4.1-fast-non-reasoning"
)

// NewNeosantaraChat creates a new Neosantara chat model. Options override the defaults
// (models.WithBaseURL, models.WithID, models.WithAPIKey, models.WithDefaultHeaders).
func NewNeosantaraChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return like.NewPreset(like.Preset{
		BaseURL:        DefaultBaseURL,
		EnvAPIKey:      EnvAPIKey,
		DefaultModel:   DefaultModel,
		FallbackAPIKey: "",
	}, options...)
}

// New is an alias of NewNeosantaraChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewNeosantaraChat(options...)
}
