// Package mistral provides integration with the Mistral AI API, which is
// compatible with the OpenAI API format. It reuses the OpenAI-like
// implementation internally.
package mistral

import (
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for Mistral.
	DefaultBaseURL = "https://api.mistral.ai/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "MISTRAL_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "mistral-large-latest"
)

// NewMistralChat creates a new Mistral chat model.
//
//	chat, err := mistral.NewMistralChat(
//	    models.WithID("mistral-large-latest"),
//	    models.WithAPIKey("your-mistral-api-key"), // or set MISTRAL_API_KEY
//	)
func NewMistralChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
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

// Popular Mistral model constants for convenience.
const (
	ModelMistralLargeLatest  = "mistral-large-latest"
	ModelMistralSmallLatest  = "mistral-small-latest"
	ModelCodestralLatest     = "codestral-latest"
	ModelMistralNemo         = "open-mistral-nemo"
)
