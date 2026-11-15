// Package lmstudio provides integration with a local LM Studio server, which exposes an
// OpenAI-compatible API. It reuses the OpenAI-like implementation internally.
package lmstudio

import (
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for LMStudio.
	DefaultBaseURL = "http://127.0.0.1:1234/v1"
	// EnvAPIKey is empty: the server does not require authentication.
	EnvAPIKey = ""
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "qwen2.5-7b-instruct-1m"
)

// NewLMStudioChat creates a new LMStudio chat model. Options override the defaults
// (models.WithBaseURL, models.WithID, models.WithAPIKey, models.WithDefaultHeaders).
func NewLMStudioChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return like.NewPreset(like.Preset{
		BaseURL:        DefaultBaseURL,
		EnvAPIKey:      EnvAPIKey,
		DefaultModel:   DefaultModel,
		FallbackAPIKey: "not-provided",
	}, options...)
}

// New is an alias of NewLMStudioChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewLMStudioChat(options...)
}
