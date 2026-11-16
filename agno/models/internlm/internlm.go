// Package internlm provides integration with InternLM (Shanghai AI Lab), which exposes an
// OpenAI-compatible API. It reuses the OpenAI-like implementation internally.
package internlm

import (
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for InternLM.
	DefaultBaseURL = "https://internlm-chat.intern-ai.org.cn/puyu/api/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "INTERNLM_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "internlm2.5-latest"
)

// NewInternLMChat creates a new InternLM chat model. Options override the defaults
// (models.WithBaseURL, models.WithID, models.WithAPIKey, models.WithDefaultHeaders).
func NewInternLMChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return like.NewPreset(like.Preset{
		BaseURL:        DefaultBaseURL,
		EnvAPIKey:      EnvAPIKey,
		DefaultModel:   DefaultModel,
		FallbackAPIKey: "",
	}, options...)
}

// New is an alias of NewInternLMChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewInternLMChat(options...)
}
