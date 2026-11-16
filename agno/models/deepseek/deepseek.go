// Package deepseek provides integration with the DeepSeek API, which is
// compatible with the OpenAI API format. It reuses the OpenAI-like
// implementation internally.
package deepseek

import (
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for DeepSeek.
	DefaultBaseURL = "https://api.deepseek.com/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "DEEPSEEK_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "deepseek-chat"
)

// NewDeepSeekChat creates a new DeepSeek chat model.
//
//	chat, err := deepseek.NewDeepSeekChat(
//	    models.WithID("deepseek-chat"),
//	    models.WithAPIKey("your-deepseek-api-key"), // or set DEEPSEEK_API_KEY
//	)
func NewDeepSeekChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
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

// Popular DeepSeek model constants for convenience.
const (
	ModelDeepSeekChat     = "deepseek-chat"
	ModelDeepSeekReasoner = "deepseek-reasoner"
)

// New is kept for backward compatibility; it is equivalent to NewDeepSeekChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewDeepSeekChat(options...)
}
