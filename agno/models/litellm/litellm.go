// Package litellm provides integration with a LiteLLM proxy server, which
// exposes an OpenAI-compatible API in front of dozens of providers. It reuses
// the OpenAI-like implementation internally.
//
// LiteLLM is a gateway you run yourself, so the base URL points at your proxy
// (configured via WithBaseURL or the LITELLM_BASE_URL env var). The model ID is
// whatever your proxy exposes.
package litellm

import (
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the default LiteLLM proxy endpoint.
	DefaultBaseURL = "http://localhost:4000"
	// EnvBaseURL overrides the proxy endpoint.
	EnvBaseURL = "LITELLM_BASE_URL"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "LITELLM_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "gpt-3.5-turbo"
)

// NewLiteLLMChat creates a new chat model backed by a LiteLLM proxy.
//
//	chat, err := litellm.NewLiteLLMChat(
//	    models.WithID("gpt-4o"),
//	    models.WithBaseURL("http://localhost:4000"), // or set LITELLM_BASE_URL
//	    models.WithAPIKey("sk-..."),                 // or set LITELLM_API_KEY
//	)
func NewLiteLLMChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
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
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = os.Getenv(EnvBaseURL)
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	final := []models.OptionClient{
		models.WithBaseURL(baseURL),
		models.WithID(model),
	}
	if apiKey != "" {
		final = append(final, models.WithAPIKey(apiKey))
	}
	final = append(final, options...)

	return like.NewLikeOpenAIChat(final...)
}
