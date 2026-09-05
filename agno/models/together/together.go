// Package together provides integration with the Together AI API, which is
// compatible with the OpenAI API format. It reuses the OpenAI-like
// implementation internally.
package together

import (
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for Together AI.
	DefaultBaseURL = "https://api.together.xyz/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "TOGETHER_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "meta-llama/Llama-3.3-70B-Instruct-Turbo"
)

// NewTogetherChat creates a new Together AI chat model.
//
//	chat, err := together.NewTogetherChat(
//	    models.WithID("meta-llama/Llama-3.3-70B-Instruct-Turbo"),
//	    models.WithAPIKey("your-together-api-key"), // or set TOGETHER_API_KEY
//	)
func NewTogetherChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
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

// Popular Together AI model constants for convenience.
const (
	ModelLlama33_70BTurbo   = "meta-llama/Llama-3.3-70B-Instruct-Turbo"
	ModelLlama31_8BTurbo    = "meta-llama/Meta-Llama-3.1-8B-Instruct-Turbo"
	ModelMixtral8x7B        = "mistralai/Mixtral-8x7B-Instruct-v0.1"
	ModelQwen2_5_72BTurbo   = "Qwen/Qwen2.5-72B-Instruct-Turbo"
)
