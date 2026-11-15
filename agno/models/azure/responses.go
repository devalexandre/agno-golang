package azure

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/responses"
)

const (
	// EnvADToken is read when no API key is available (Microsoft Entra ID token).
	EnvADToken = "AZURE_OPENAI_AD_TOKEN"
	// EnvAPIVersion overrides DefaultResponsesAPIVersion.
	EnvAPIVersion = "OPENAI_API_VERSION"
	// DefaultResponsesAPIVersion is the api-version used for the Responses API.
	DefaultResponsesAPIVersion = "2025-04-01-preview"
)

// ResponsesConfig holds the Azure OpenAI Responses connection parameters.
type ResponsesConfig struct {
	// Deployment is the Azure deployment name, sent as the request "model" (required).
	Deployment string
	APIKey     string // falls back to AZURE_OPENAI_API_KEY; sent as the "api-key" header
	ADToken    string // falls back to AZURE_OPENAI_AD_TOKEN; sent as "Authorization: Bearer"
	Endpoint   string // e.g. https://my-resource.openai.azure.com (or AZURE_OPENAI_ENDPOINT)
	// BaseURL, when set, is used as-is instead of "{Endpoint}/openai".
	BaseURL    string
	APIVersion string // falls back to OPENAI_API_VERSION, then DefaultResponsesAPIVersion
	// IsReasoningModel overrides name-based detection, since deployment names
	// are arbitrary.
	IsReasoningModel *bool
	// Responses carries the remaining Responses request settings (store,
	// reasoning effort, include, ...). Its ID, APIKey, BaseURL, Headers and
	// Query fields are overwritten.
	Responses responses.Config
}

// NewAzureResponses creates an Azure OpenAI model backed by the Responses API
// (POST {endpoint}/openai/responses?api-version=...). Unlike chat completions,
// Azure selects the deployment from the request body's "model" field.
//
//	m, err := azure.NewAzureResponses(azure.ResponsesConfig{Deployment: "gpt-5-mini"})
func NewAzureResponses(cfg ResponsesConfig) (*responses.Responses, error) {
	if cfg.Deployment == "" {
		return nil, errors.New("azure: Responses requires Deployment (the Azure deployment name)")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		endpoint := cfg.Endpoint
		if endpoint == "" {
			endpoint = os.Getenv(EnvEndpoint)
		}
		if endpoint == "" {
			return nil, errors.New("azure: endpoint not set; set " + EnvEndpoint + " or ResponsesConfig.Endpoint")
		}
		baseURL = strings.TrimRight(endpoint, "/") + "/openai"
	}

	apiVersion := cfg.APIVersion
	if apiVersion == "" {
		apiVersion = os.Getenv(EnvAPIVersion)
	}
	if apiVersion == "" {
		apiVersion = DefaultResponsesAPIVersion
	}

	apiKey, adToken := cfg.APIKey, cfg.ADToken
	if apiKey == "" && adToken == "" {
		apiKey = os.Getenv(EnvAPIKey)
		if apiKey == "" {
			adToken = os.Getenv(EnvADToken)
		}
	}
	if apiKey == "" && adToken == "" {
		return nil, errors.New("azure: authentication not configured; provide an API key (" + EnvAPIKey + ") or AD token (" + EnvADToken + ")")
	}

	rc := cfg.Responses
	rc.ID = cfg.Deployment
	rc.BaseURL = baseURL
	headers := http.Header{}
	for k, v := range rc.Headers {
		headers[k] = append([]string(nil), v...)
	}
	if apiKey != "" {
		// Never mix authentication methods: api-key only.
		rc.APIKey = ""
		headers.Set("api-key", apiKey)
	} else {
		rc.APIKey = adToken
	}
	rc.Headers = headers
	query := map[string]string{}
	for k, v := range rc.Query {
		query[k] = v
	}
	query["api-version"] = apiVersion
	rc.Query = query
	if cfg.IsReasoningModel != nil {
		rc.IsReasoningModel = cfg.IsReasoningModel
	}
	return responses.New(rc)
}

// NewAzureResponsesFromOptions builds an Azure Responses model from client
// options: ID = deployment (or AZURE_OPENAI_DEPLOYMENT_NAME), BaseURL =
// endpoint, APIKey, ClientParams["api_version"], plus the Responses
// ClientParams documented in responses.NewOpenAIResponses.
func NewAzureResponsesFromOptions(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	co := &models.ClientOptions{}
	for _, o := range options {
		o(co)
	}
	deployment := co.ID
	if deployment == "" {
		deployment = os.Getenv(EnvDeployment)
	}
	apiVersion, _ := co.ClientParams["api_version"].(string)
	rc := responses.ConfigFromClientOptions(co)
	return NewAzureResponses(ResponsesConfig{
		Deployment:       deployment,
		APIKey:           co.APIKey,
		Endpoint:         co.BaseURL,
		APIVersion:       apiVersion,
		IsReasoningModel: rc.IsReasoningModel,
		Responses:        rc,
	})
}
