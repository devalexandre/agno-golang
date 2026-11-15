// Package trustedrouter provides integration with TrustedRouter, which exposes an
// OpenAI-compatible API. It reuses the OpenAI-like implementation internally.
package trustedrouter

import (
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for TrustedRouter.
	DefaultBaseURL = "https://api.trustedrouter.com/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "TRUSTEDROUTER_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "trustedrouter/zdr"
)

// NewTrustedRouterChat creates a new TrustedRouter chat model. Options override the defaults
// (models.WithBaseURL, models.WithID, models.WithAPIKey, models.WithDefaultHeaders).
func NewTrustedRouterChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return like.NewPreset(like.Preset{
		BaseURL:        DefaultBaseURL,
		EnvAPIKey:      EnvAPIKey,
		DefaultModel:   DefaultModel,
		FallbackAPIKey: "",
	}, options...)
}

// New is an alias of NewTrustedRouterChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewTrustedRouterChat(options...)
}
