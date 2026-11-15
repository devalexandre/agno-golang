// Package meta provides integration with the Meta Llama API via its
// OpenAI-compatible endpoint. It reuses the OpenAI-like implementation
// internally.
package meta

import (
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for the Meta Llama API.
	DefaultBaseURL = "https://api.llama.com/compat/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "LLAMA_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "Llama-3.3-70B-Instruct"
)

// NewMetaChat creates a new Meta Llama API chat model.
//
//	chat, err := meta.NewMetaChat(
//	    models.WithID("Llama-3.3-70B-Instruct"),
//	    models.WithAPIKey("your-llama-api-key"), // or set LLAMA_API_KEY
//	)
func NewMetaChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
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

// Popular Meta Llama API model constants for convenience.
const (
	ModelLlama33_70B       = "Llama-3.3-70B-Instruct"
	ModelLlama4Maverick    = "Llama-4-Maverick-17B-128E-Instruct-FP8"
	ModelLlama4Scout       = "Llama-4-Scout-17B-16E-Instruct-FP8"
)
