// Package fireworks provides integration with the Fireworks AI API, which is
// compatible with the OpenAI API format. It reuses the OpenAI-like
// implementation internally.
package fireworks

import (
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for Fireworks AI.
	DefaultBaseURL = "https://api.fireworks.ai/inference/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "FIREWORKS_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "accounts/fireworks/models/llama-v3p3-70b-instruct"
)

// NewFireworksChat creates a new Fireworks AI chat model.
//
//	chat, err := fireworks.NewFireworksChat(
//	    models.WithID("accounts/fireworks/models/llama-v3p3-70b-instruct"),
//	    models.WithAPIKey("your-fireworks-api-key"), // or set FIREWORKS_API_KEY
//	)
func NewFireworksChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
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

// Popular Fireworks AI model constants for convenience.
const (
	ModelLlama33_70B = "accounts/fireworks/models/llama-v3p3-70b-instruct"
	ModelLlama31_8B  = "accounts/fireworks/models/llama-v3p1-8b-instruct"
	ModelQwen2_5_72B = "accounts/fireworks/models/qwen2p5-72b-instruct"
	ModelMixtral8x7B = "accounts/fireworks/models/mixtral-8x7b-instruct"
)
