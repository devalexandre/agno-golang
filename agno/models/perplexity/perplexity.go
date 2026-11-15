// Package perplexity provides integration with the Perplexity AI API, which is
// compatible with the OpenAI API format. It reuses the OpenAI-like
// implementation internally.
package perplexity

import (
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for Perplexity.
	DefaultBaseURL = "https://api.perplexity.ai"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "PERPLEXITY_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "sonar"
)

// NewPerplexityChat creates a new Perplexity chat model.
//
//	chat, err := perplexity.NewPerplexityChat(
//	    models.WithID("sonar"),
//	    models.WithAPIKey("your-perplexity-api-key"), // or set PERPLEXITY_API_KEY
//	)
func NewPerplexityChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
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

// Popular Perplexity model constants for convenience.
const (
	ModelSonar         = "sonar"
	ModelSonarPro      = "sonar-pro"
	ModelSonarReasoning = "sonar-reasoning"
)
