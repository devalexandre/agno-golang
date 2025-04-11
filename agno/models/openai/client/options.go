package client

import (
	"net/http"

	"github.com/devalexandre/agno-golang/agno/models"
)

// ClientOptions represents the options for the OpenAI API client.
type ClientOptions struct {
	APIKey         string            `json:"-"`                       // API key.
	Organization   string            `json:"-"`                       // Associated organization.
	BaseURL        string            `json:"-"`                       // Base API URL.
	Timeout        int               `json:"-"`                       // Request timeout.
	MaxRetries     int               `json:"-"`                       // Maximum number of retries.
	DefaultHeaders http.Header       `json:"-"`                       // Default headers.
	DefaultQuery   map[string]string `json:"-"`                       // Default query parameters.
	HTTPClient     *http.Client      `json:"-"`                       // Custom HTTP client.
	ClientParams   map[string]any    `json:"client_params,omitempty"` // Additional client parameters.
	// Additional fields for chat requests.
	ID               string   // Model to be used.
	Temperature      *float32 // Response temperature.
	MaxTokens        *int     // Maximum number of tokens.
	TopP             *float32 // Top-P parameter.
	FrequencyPenalty *float32 // Frequency penalty.
	PresencePenalty  *float32 // Presence penalty.
}

// DefaultOptions returns the default options for the OpenAI API client.
func DefaultOptions() *ClientOptions {
	return &ClientOptions{
		ID:               "gpt-3.5-turbo",
		Temperature:      floatPtr(0.7),
		MaxTokens:        intPtr(100),
		TopP:             floatPtr(1.0),
		FrequencyPenalty: floatPtr(0.0),
		PresencePenalty:  floatPtr(0.0),
	}
}

// DefaultCallOptions returns the default options for the request.
func DefaultCallOptions() *models.CallOptions {
	return &models.CallOptions{
		Temperature:      floatPtr(0.7),
		MaxTokens:        intPtr(100),
		TopP:             floatPtr(1.0),
		FrequencyPenalty: floatPtr(0.0),
		PresencePenalty:  floatPtr(0.0),
	}
}

// WithID sets the model to be used.
func WithID(id string) OptionClient {
	return func(o *ClientOptions) {
		o.ID = id
	}
}

// WithAPIKey sets the API key for the client.
func WithAPIKey(key string) OptionClient {
	return func(o *ClientOptions) {
		o.APIKey = key
	}
}

// Helper functions to create pointers for optional fields.
func boolPtr(b bool) *bool        { return &b }
func floatPtr(f float32) *float32 { return &f }
func intPtr(i int) *int           { return &i }
func strPtr(s string) *string     { return &s }
