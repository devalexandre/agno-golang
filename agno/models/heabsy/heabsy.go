// Package heabsy provides integration with Heabsy, an OpenAI-compatible inference API for open models.
// It reuses the OpenAI-like implementation internally.
package heabsy

import (
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for Heabsy.
	DefaultBaseURL = "https://api.heabsy.com/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "HEABSY_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "qwen38"
)

// buildOptions applies the Heabsy defaults; caller options win.
func buildOptions(options ...models.OptionClient) []models.OptionClient {
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
	return append(final, options...)
}

// NewHeabsyChat creates a new Heabsy chat model.
//
//	chat, err := heabsy.NewHeabsyChat(
//	    models.WithID("qwen38"),
//	    models.WithAPIKey("your-key"), // or set HEABSY_API_KEY
//	)
func NewHeabsyChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return like.NewLikeOpenAIChat(buildOptions(options...)...)
}

// New is an alias for NewHeabsyChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewHeabsyChat(options...)
}
