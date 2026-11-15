// Package xiaomi provides integration with Xiaomi MiMo, which exposes an
// OpenAI-compatible API. It reuses the OpenAI-like implementation internally.
package xiaomi

import (
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for MiMo.
	DefaultBaseURL = "https://api.xiaomimimo.com/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "MIMO_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "mimo-v2.5-pro"
)

// NewMiMoChat creates a new MiMo chat model. Options override the defaults
// (models.WithBaseURL, models.WithID, models.WithAPIKey, models.WithDefaultHeaders).
func NewMiMoChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	preset := like.Preset{
		BaseURL:      DefaultBaseURL,
		EnvAPIKey:    EnvAPIKey,
		DefaultModel: DefaultModel,
	}
	// ClientParams["use_thinking"] (bool) toggles thinking mode through the
	// nested {"thinking": {"type": ...}} request object. Unset keeps the model
	// default.
	co := like.ResolveClientOptions(options...)
	if v, ok := co.ClientParams[ParamUseThinking].(bool); ok {
		preset.CallDefaults = append(preset.CallDefaults, models.WithRequestParams(like.ThinkingParams(v)))
	}
	return like.NewPreset(preset, options...)
}

// ParamUseThinking is the ClientParams key that toggles thinking mode:
//
//	NewMiMoChat(models.WithClientParams(map[string]interface{}{"use_thinking": false}))
const ParamUseThinking = "use_thinking"

// New is an alias of NewMiMoChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewMiMoChat(options...)
}
