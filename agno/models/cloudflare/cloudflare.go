// Package cloudflare provides integration with Cloudflare AI Gateway's
// OpenAI-compatible unified ("compat") API. The default model runs on Workers
// AI; other vendors are reachable with "{provider}/{model}" ids (BYOK).
package cloudflare

import (
	"fmt"
	"os"
	"strings"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// EnvAPIKey holds a Cloudflare API token with AI Gateway access.
	EnvAPIKey = "CLOUDFLARE_API_TOKEN"
	// EnvAccountID holds the Cloudflare account id used to build the gateway URL.
	EnvAccountID = "CLOUDFLARE_ACCOUNT_ID"
	// EnvGatewayID holds the AI Gateway id; "default" is used when unset.
	EnvGatewayID = "CLOUDFLARE_AI_GATEWAY_ID"
	// DefaultGatewayID is the auto-created gateway.
	DefaultGatewayID = "default"
	// DefaultModel is the Workers AI binding id used when no model ID is provided.
	DefaultModel = "@cf/meta/llama-3.3-70b-instruct-fp8-fast"
	// GatewayURLTemplate is formatted with the account and gateway ids.
	GatewayURLTemplate = "https://gateway.ai.cloudflare.com/v1/%s/%s/compat"

	// ParamAccountID and ParamGatewayID are the ClientParams keys that override
	// the environment variables.
	ParamAccountID = "account_id"
	ParamGatewayID = "gateway_id"
)

// NormalizeModelID maps a Workers AI catalog binding ("@cf/...") to the gateway
// model string ("workers-ai/@cf/..."). Other ids are returned trimmed.
func NormalizeModelID(id string) string {
	mid := strings.TrimSpace(id)
	if mid == "" || strings.HasPrefix(strings.ToLower(mid), "workers-ai/") {
		return mid
	}
	if strings.HasPrefix(mid, "@cf/") {
		return "workers-ai/" + mid
	}
	return mid
}

// GatewayBaseURL builds the compat base URL for an account and gateway.
func GatewayBaseURL(accountID, gatewayID string) string {
	if gatewayID == "" {
		gatewayID = DefaultGatewayID
	}
	return fmt.Sprintf(GatewayURLTemplate, accountID, gatewayID)
}

// NewCloudflareChat creates a new Cloudflare AI Gateway chat model.
//
// Unless models.WithBaseURL is given, the base URL is built from the account id
// (ClientParams["account_id"] or CLOUDFLARE_ACCOUNT_ID) and gateway id
// (ClientParams["gateway_id"], CLOUDFLARE_AI_GATEWAY_ID or "default").
func NewCloudflareChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	co := like.ResolveClientOptions(options...)

	id := co.ID
	if id == "" {
		id = DefaultModel
	}
	id = NormalizeModelID(id)

	baseURL := co.BaseURL
	if baseURL == "" {
		account, _ := co.ClientParams[ParamAccountID].(string)
		if account == "" {
			account = os.Getenv(EnvAccountID)
		}
		if account == "" {
			return nil, fmt.Errorf("cloudflare: %s not set; set it, pass ClientParams[%q] or models.WithBaseURL", EnvAccountID, ParamAccountID)
		}
		gateway, _ := co.ClientParams[ParamGatewayID].(string)
		if gateway == "" {
			gateway = os.Getenv(EnvGatewayID)
		}
		baseURL = GatewayBaseURL(account, gateway)
	}

	apiKey := co.APIKey
	if apiKey == "" {
		apiKey = os.Getenv(EnvAPIKey)
	}
	if apiKey == "" {
		return nil, fmt.Errorf("cloudflare: %s not set; provide an API token with AI Gateway access", EnvAPIKey)
	}

	final := append([]models.OptionClient{}, options...)
	final = append(final, models.WithID(id), models.WithBaseURL(baseURL), models.WithAPIKey(apiKey))
	return like.NewPreset(like.Preset{DefaultModel: DefaultModel, BaseURL: baseURL}, final...)
}

// New is an alias of NewCloudflareChat.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewCloudflareChat(options...)
}
