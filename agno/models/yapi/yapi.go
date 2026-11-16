// Package yapi provides integration with Y-API, an OpenAI-compatible gateway
// that serves models from multiple providers. It reuses the OpenAI-like
// implementation internally.
package yapi

import (
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for Y-API.
	DefaultBaseURL = "https://api.y-api.bestvirtualgoods.com/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "YAPI_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "deepseek/deepseek-v4-flash"
)

// buildOptions applies the Y-API defaults; caller options win.
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

// NewYAPIChat creates a new Y-API chat model.
//
//	chat, err := yapi.NewYAPIChat(
//	    models.WithID("deepseek/deepseek-v4-flash"),
//	    models.WithAPIKey("your-key"), // or set YAPI_API_KEY
//	)
func NewYAPIChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return like.NewLikeOpenAIChat(buildOptions(options...)...)
}

// New is an alias for NewYAPIChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewYAPIChat(options...)
}
