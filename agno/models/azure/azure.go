// Package azure provides integration with Azure OpenAI. It reuses the
// OpenAI-compatible client, adapting authentication (api-key header) and the
// deployment-based URL layout (api-version query) that Azure requires.
package azure

import (
	"net/http"
	"os"
	"strings"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// EnvAPIKey is the environment variable for the Azure OpenAI key.
	EnvAPIKey = "AZURE_OPENAI_API_KEY"
	// EnvEndpoint is the environment variable for the Azure resource endpoint.
	EnvEndpoint = "AZURE_OPENAI_ENDPOINT"
	// DefaultAPIVersion is used when no api-version is provided.
	DefaultAPIVersion = "2024-10-21"
)

// Config holds the Azure OpenAI connection parameters.
type Config struct {
	APIKey     string // falls back to AZURE_OPENAI_API_KEY
	Endpoint   string // e.g. https://my-resource.openai.azure.com (or AZURE_OPENAI_ENDPOINT)
	Deployment string // the chat model deployment name (required)
	APIVersion string // defaults to DefaultAPIVersion
}

// NewAzureChat creates a new Azure OpenAI chat model.
//
//	chat, err := azure.NewAzureChat(azure.Config{
//	    Deployment: "gpt-4o",        // your Azure deployment name
//	    // APIKey/Endpoint read from AZURE_OPENAI_API_KEY / AZURE_OPENAI_ENDPOINT
//	})
//
// Extra models.OptionClient values (temperature, etc.) may be appended.
func NewAzureChat(cfg Config, options ...models.OptionClient) (models.AgnoModelInterface, error) {
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv(EnvAPIKey)
	}
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = os.Getenv(EnvEndpoint)
	}
	endpoint = strings.TrimRight(endpoint, "/")

	apiVersion := cfg.APIVersion
	if apiVersion == "" {
		apiVersion = DefaultAPIVersion
	}

	// Azure routes by deployment in the URL; the SDK appends /chat/completions.
	baseURL := endpoint + "/openai/deployments/" + cfg.Deployment

	headers := http.Header{}
	headers.Set("api-key", apiKey)

	final := []models.OptionClient{
		models.WithBaseURL(baseURL),
		models.WithID(cfg.Deployment),
		models.WithAPIKey(apiKey), // satisfies the client; Azure uses the api-key header
		models.WithDefaultHeaders(headers),
		models.WithDefaultQuery(map[string]string{"api-version": apiVersion}),
	}
	final = append(final, options...)

	return like.NewLikeOpenAIChat(final...)
}

// EnvDeployment is read by New when no model ID is given.
const EnvDeployment = "AZURE_OPENAI_DEPLOYMENT_NAME"

// New is kept for backward compatibility. It builds a Config from the client
// options (ID = deployment, BaseURL = endpoint, APIKey) and falls back to the
// AZURE_OPENAI_* environment variables.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	opts := &models.ClientOptions{}
	for _, option := range options {
		option(opts)
	}
	deployment := opts.ID
	if deployment == "" {
		deployment = os.Getenv(EnvDeployment)
	}
	return NewAzureChat(Config{APIKey: opts.APIKey, Endpoint: opts.BaseURL, Deployment: deployment}, options...)
}
