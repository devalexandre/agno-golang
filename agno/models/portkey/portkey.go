// Package portkey provides integration with the Portkey AI Gateway, which is
// OpenAI-compatible and authenticates with x-portkey-* headers.
package portkey

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the Portkey gateway URL (PORTKEY_GATEWAY_URL).
	DefaultBaseURL = "https://api.portkey.ai/v1"
	// EnvAPIKey holds the Portkey API key (sent as x-portkey-api-key).
	EnvAPIKey = "PORTKEY_API_KEY"
	// EnvVirtualKey holds the Portkey virtual key used for model routing.
	EnvVirtualKey = "PORTKEY_VIRTUAL_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "gpt-4o-mini"

	// ParamVirtualKey and ParamConfig are ClientParams keys. The config (any
	// JSON-serializable value, or a config id string) is sent as x-portkey-config.
	ParamVirtualKey = "virtual_key"
	ParamConfig     = "config"
)

// NewPortkeyChat creates a new Portkey gateway chat model. models.WithAPIKey
// sets the Portkey API key (falls back to PORTKEY_API_KEY).
func NewPortkeyChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	co := like.ResolveClientOptions(options...)

	portkeyKey := co.APIKey
	if portkeyKey == "" {
		portkeyKey = os.Getenv(EnvAPIKey)
	}
	if portkeyKey == "" {
		return nil, fmt.Errorf("portkey: %s not set", EnvAPIKey)
	}

	headers := http.Header{}
	headers.Set("x-portkey-api-key", portkeyKey)

	virtualKey, _ := co.ClientParams[ParamVirtualKey].(string)
	if virtualKey == "" {
		virtualKey = os.Getenv(EnvVirtualKey)
	}
	if virtualKey != "" {
		headers.Set("x-portkey-virtual-key", virtualKey)
	}

	if cfg, ok := co.ClientParams[ParamConfig]; ok && cfg != nil {
		if s, isStr := cfg.(string); isStr {
			headers.Set("x-portkey-config", s)
		} else {
			raw, err := json.Marshal(cfg)
			if err != nil {
				return nil, fmt.Errorf("portkey: invalid config: %w", err)
			}
			headers.Set("x-portkey-config", string(raw))
		}
	}

	// The OpenAI Authorization header is not used: Portkey authenticates with
	// the x-portkey-* headers (virtual keys hold the provider credentials).
	final := append([]models.OptionClient{}, options...)
	final = append(final, models.WithAPIKey("not-needed"))
	return like.NewPreset(like.Preset{
		BaseURL:      DefaultBaseURL,
		DefaultModel: DefaultModel,
		Headers:      headers,
	}, final...)
}

// New is an alias of NewPortkeyChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewPortkeyChat(options...)
}
