// Package maritaca provides integration with Maritaca AI (Sabiá), a Brazilian
// LLM provider, via its OpenAI-compatible API.
// Docs: https://docs.maritaca.ai/pt/api/openai-compatibilidade
package maritaca

import (
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for Maritaca AI.
	DefaultBaseURL = "https://chat.maritaca.ai/api"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "MARITACA_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "sabia-3"
)

// NewMaritacaChat creates a new Maritaca AI (Sabiá) chat model.
//
//	chat, err := maritaca.NewMaritacaChat(
//	    models.WithID("sabia-3"),
//	    models.WithAPIKey("your-maritaca-api-key"), // or set MARITACA_API_KEY
//	)
func NewMaritacaChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
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

// Popular Maritaca (Sabiá) model constants for convenience.
const (
	ModelSabia3     = "sabia-3"
	ModelSabiazinho3 = "sabiazinho-3"
)
