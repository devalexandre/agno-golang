// Package langdb provides integration with the LangDB AI gateway, which exposes
// a per-project OpenAI-compatible API.
package langdb

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "LANGDB_API_KEY"
	// EnvProjectID holds the LangDB project id (part of the base URL).
	EnvProjectID = "LANGDB_PROJECT_ID"
	// EnvBaseHostURL overrides DefaultBaseHostURL.
	EnvBaseHostURL = "LANGDB_API_BASE_URL"
	// DefaultBaseHostURL is the LangDB API host.
	DefaultBaseHostURL = "https://api.us-east-1.langdb.ai"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "gpt-4o"

	// ParamProjectID and ParamLabel are ClientParams keys. The label is sent as
	// the "x-label" header.
	ParamProjectID = "project_id"
	ParamLabel     = "label"
)

// NewLangDBChat creates a new LangDB chat model. The base URL is
// "{LANGDB_API_BASE_URL}/{project_id}/v1" unless models.WithBaseURL is given.
func NewLangDBChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	co := like.ResolveClientOptions(options...)

	baseURL := co.BaseURL
	if baseURL == "" {
		project, _ := co.ClientParams[ParamProjectID].(string)
		if project == "" {
			project = os.Getenv(EnvProjectID)
		}
		if project == "" {
			return nil, fmt.Errorf("langdb: %s not set; set it or pass ClientParams[%q]", EnvProjectID, ParamProjectID)
		}
		host := os.Getenv(EnvBaseHostURL)
		if host == "" {
			host = DefaultBaseHostURL
		}
		baseURL = strings.TrimRight(host, "/") + "/" + project + "/v1"
	}

	preset := like.Preset{BaseURL: baseURL, EnvAPIKey: EnvAPIKey, DefaultModel: DefaultModel}
	if label, _ := co.ClientParams[ParamLabel].(string); label != "" {
		preset.Headers = http.Header{"X-Label": []string{label}}
	}
	return like.NewPreset(preset, options...)
}

// New is an alias of NewLangDBChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewLangDBChat(options...)
}
