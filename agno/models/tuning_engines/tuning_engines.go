// Package tuningengines provides integration with Tuning Engines, which exposes an
// OpenAI-compatible API. It reuses the OpenAI-like implementation internally.
package tuningengines

import (
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for TuningEngines.
	DefaultBaseURL = "https://api.tuningengines.com/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "TUNING_ENGINES_API_KEY"
	// EnvBaseURL overrides DefaultBaseURL when set.
	EnvBaseURL = "TUNING_ENGINES_BASE_URL"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "gpt-4o"
)

// NewTuningEnginesChat creates a new TuningEngines chat model. Options override the defaults
// (models.WithBaseURL, models.WithID, models.WithAPIKey, models.WithDefaultHeaders).
func NewTuningEnginesChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	baseURL := DefaultBaseURL
	if env := os.Getenv(EnvBaseURL); env != "" {
		baseURL = env
	}
	return like.NewPreset(like.Preset{
		BaseURL:        baseURL,
		EnvAPIKey:      EnvAPIKey,
		DefaultModel:   DefaultModel,
		FallbackAPIKey: "",
	}, options...)
}

// New is an alias of NewTuningEnginesChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewTuningEnginesChat(options...)
}
