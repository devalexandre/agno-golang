// Package siliconflow provides integration with SiliconFlow, which exposes an
// OpenAI-compatible API. It reuses the OpenAI-like implementation internally.
package siliconflow

import (
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for Siliconflow.
	DefaultBaseURL = "https://api.siliconflow.cn/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "SILICONFLOW_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "Qwen/QwQ-32B"
)

// NewSiliconflowChat creates a new Siliconflow chat model. Options override the defaults
// (models.WithBaseURL, models.WithID, models.WithAPIKey, models.WithDefaultHeaders).
func NewSiliconflowChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return like.NewPreset(like.Preset{
		BaseURL:        DefaultBaseURL,
		EnvAPIKey:      EnvAPIKey,
		DefaultModel:   DefaultModel,
		FallbackAPIKey: "",
	}, options...)
}

// New is an alias of NewSiliconflowChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewSiliconflowChat(options...)
}
