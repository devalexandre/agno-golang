// Package xai provides integration with the xAI (Grok) API, which is compatible
// with the OpenAI API format. It reuses the OpenAI-like implementation internally.
package xai

import (
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for xAI.
	DefaultBaseURL = "https://api.x.ai/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "XAI_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "grok-2-latest"
)

// NewXaiChat creates a new xAI (Grok) chat model.
//
//	chat, err := xai.NewXaiChat(
//	    models.WithID("grok-2-latest"),
//	    models.WithAPIKey("your-xai-api-key"), // or set XAI_API_KEY
//	)
func NewXaiChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	opts := &models.ClientOptions{}
	for _, option := range options {
		option(opts)
	}

	model := opts.ID
	if model == "" {
		model = DefaultModel
	}
	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = os.Getenv(EnvAPIKey)
	}

	final := []models.OptionClient{
		models.WithBaseURL(DefaultBaseURL),
		models.WithID(model),
	}
	if apiKey != "" {
		final = append(final, models.WithAPIKey(apiKey))
	}
	final = append(final, options...)

	return like.NewLikeOpenAIChat(final...)
}

// Popular xAI (Grok) model constants for convenience.
const (
	ModelGrok2Latest = "grok-2-latest"
	ModelGrok2       = "grok-2"
	ModelGrokBeta    = "grok-beta"
)
