// Package cohere provides integration with the Cohere API via its OpenAI
// compatibility endpoint (https://docs.cohere.com/docs/compatibility-api).
// It reuses the OpenAI-like implementation internally.
//
// For Cohere-native features not exposed by the compatibility layer, a native
// client may be added in a later phase.
package cohere

import (
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is Cohere's OpenAI-compatible endpoint.
	DefaultBaseURL = "https://api.cohere.ai/compatibility/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "COHERE_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "command-r-plus"
)

// NewCohereChat creates a new Cohere chat model.
//
//	chat, err := cohere.NewCohereChat(
//	    models.WithID("command-r-plus"),
//	    models.WithAPIKey("your-cohere-api-key"), // or set COHERE_API_KEY
//	)
func NewCohereChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
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

// Popular Cohere model constants for convenience.
const (
	ModelCommandRPlus = "command-r-plus"
	ModelCommandR     = "command-r"
	ModelCommandA     = "command-a-03-2025"
)
