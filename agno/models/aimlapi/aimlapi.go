// Package aimlapi provides integration with AIMLAPI, the OpenAI-compatible AI/ML API gateway.
// It reuses the OpenAI-like implementation internally.
package aimlapi

import (
	"net/http"
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for AIMLAPI.
	DefaultBaseURL = "https://api.aimlapi.com/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "AIMLAPI_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "gpt-5.6-luna"
)

// AttributionHeaders are sent with every AI/ML API call so the gateway can
// attribute traffic to Agno. They are analytics-only and do not affect routing,
// model selection or billing (mirrors Python's AIMLAPI_HEADERS).
var AttributionHeaders = map[string]string{
	"HTTP-Referer":         "https://github.com/agno-agi/agno",
	"X-Title":              "Agno",
	"X-AIMLAPI-Partner-ID": "part_VhLgeTWXXG9RwBOTptNQtcq0",
	"X-AIMLAPI-Source":     "agent/agno",
}

// mergeHeaders returns the attribution headers merged with caller headers; the
// caller wins on a (case-insensitive) clash.
func mergeHeaders(caller http.Header) http.Header {
	merged := http.Header{}
	for k, v := range AttributionHeaders {
		merged.Set(k, v)
	}
	for k, values := range caller {
		key := http.CanonicalHeaderKey(k)
		merged.Del(key)
		for _, v := range values {
			merged.Add(key, v)
		}
	}
	return merged
}

// buildOptions applies the AIMLAPI defaults; caller options win.
func buildOptions(options ...models.OptionClient) []models.OptionClient {
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
	// Applied last so the merged set (attribution + caller headers) is kept.
	return append(final, models.WithDefaultHeaders(mergeHeaders(opts.DefaultHeaders)))
}

// NewAIMLAPIChat creates a new AIMLAPI chat model.
//
//	chat, err := aimlapi.NewAIMLAPIChat(
//	    models.WithID("gpt-5.6-luna"),
//	    models.WithAPIKey("your-key"), // or set AIMLAPI_API_KEY
//	)
func NewAIMLAPIChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return like.NewLikeOpenAIChat(buildOptions(options...)...)
}

// New is an alias for NewAIMLAPIChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewAIMLAPIChat(options...)
}
