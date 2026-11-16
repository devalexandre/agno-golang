// Package flexai provides integration with FlexAI, the OpenAI-compatible FlexAI Inference API.
// It reuses the OpenAI-like implementation internally.
package flexai

import (
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for FlexAI.
	DefaultBaseURL = "https://api.flex.ai/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "FLEXAI_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "DeepSeek-V4-Flash-0731"
)

// buildOptions applies the FlexAI defaults; caller options win.
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

// NewFlexAIChat creates a new FlexAI chat model.
//
//	chat, err := flexai.NewFlexAIChat(
//	    models.WithID("DeepSeek-V4-Flash-0731"),
//	    models.WithAPIKey("your-key"), // or set FLEXAI_API_KEY
//	)
func NewFlexAIChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return like.NewLikeOpenAIChat(buildOptions(options...)...)
}

// New is an alias for NewFlexAIChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewFlexAIChat(options...)
}
