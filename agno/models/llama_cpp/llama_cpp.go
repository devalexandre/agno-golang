// Package llamacpp provides integration with a local llama.cpp server (llama-server), which exposes an
// OpenAI-compatible API. It reuses the OpenAI-like implementation internally.
package llamacpp

import (
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for LlamaCpp.
	DefaultBaseURL = "http://127.0.0.1:8080/v1"
	// EnvAPIKey is empty: the server does not require authentication.
	EnvAPIKey = ""
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "ggml-org/gpt-oss-20b-GGUF"
)

// NewLlamaCppChat creates a new LlamaCpp chat model. Options override the defaults
// (models.WithBaseURL, models.WithID, models.WithAPIKey, models.WithDefaultHeaders).
func NewLlamaCppChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return like.NewPreset(like.Preset{
		BaseURL:        DefaultBaseURL,
		EnvAPIKey:      EnvAPIKey,
		DefaultModel:   DefaultModel,
		FallbackAPIKey: "not-provided",
	}, options...)
}

// New is an alias of NewLlamaCppChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewLlamaCppChat(options...)
}
