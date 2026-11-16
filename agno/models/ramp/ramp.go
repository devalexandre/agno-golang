// Package ramp provides integration with Ramp Router, an OpenAI Responses API
// gateway (there is no /chat/completions endpoint). It reuses the Responses
// client from agno/models/openai/responses.
package ramp

import (
	"fmt"
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/responses"
)

const (
	// DefaultBaseURL is the Ramp Router endpoint.
	DefaultBaseURL = "https://api.router.com/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "RAMP_ROUTER_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "gpt-5.6-luna"
)

// routerParams are Router-only request fields, read from ClientParams and sent
// in the request body: models ([]string routing candidates), allow_flex_tier
// (bool), provider_timeout and timeout_before_headers (seconds).
var routerParams = []string{"models", "allow_flex_tier", "provider_timeout", "timeout_before_headers"}

// NewRampRouter creates a Ramp Router model. Store is left unset (Router's own
// default) and turns are replayed rather than chained, because chaining silently
// no-ops on some Router backings.
func NewRampRouter(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	co := &models.ClientOptions{}
	for _, o := range options {
		o(co)
	}
	cfg := responses.ConfigFromClientOptions(co)
	if cfg.ID == "" {
		cfg.ID = DefaultModel
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv(EnvAPIKey)
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("ramp: %s not set", EnvAPIKey)
	}
	if bg, _ := co.ClientParams["background"].(bool); bg {
		return nil, fmt.Errorf("ramp: Router does not support background mode")
	}
	notReasoning := false
	if cfg.IsReasoningModel == nil {
		cfg.IsReasoningModel = &notReasoning
	}
	cfg.DisablePreviousResponseID = true

	extra := map[string]interface{}{}
	for k, v := range cfg.ExtraBody {
		extra[k] = v
	}
	for _, k := range routerParams {
		if v, ok := co.ClientParams[k]; ok && v != nil {
			extra[k] = v
		}
	}
	if _, hasModels := extra["models"]; hasModels && co.ID == "" {
		// Router rejects requests that send both "model" and "models".
		extra["model"] = nil
	}
	cfg.ExtraBody = extra
	return responses.New(cfg)
}

// New is an alias of NewRampRouter.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewRampRouter(options...)
}
