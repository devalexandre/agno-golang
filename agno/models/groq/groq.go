// Package groq provides integration with the Groq API, which is compatible with
// the OpenAI API format. It reuses the OpenAI-like implementation internally.
package groq

import (
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for Groq.
	DefaultBaseURL = "https://api.groq.com/openai/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "GROQ_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "llama-3.3-70b-versatile"
)

// NewGroqChat creates a new Groq chat model.
//
//	chat, err := groq.NewGroqChat(
//	    models.WithID("llama-3.3-70b-versatile"),
//	    models.WithAPIKey("your-groq-api-key"), // or set GROQ_API_KEY
//	)
func NewGroqChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
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

// Popular Groq model constants for convenience.
const (
	ModelLlama33_70BVersatile = "llama-3.3-70b-versatile"
	ModelLlama31_8BInstant    = "llama-3.1-8b-instant"
	ModelMixtral8x7B          = "mixtral-8x7b-32768"
	ModelGemma2_9B            = "gemma2-9b-it"
)
