package gemini

import (
	"net/http"
)

// ClientOptions represents the options for the Gemini API client
type ClientOptions struct {
	APIKey         string                 `json:"-"`                       // API key
	Organization   string                 `json:"-"`                       // Associated organization
	BaseURL        string                 `json:"-"`                       // Base API URL
	Timeout        int                    `json:"-"`                       // Request timeout
	MaxRetries     int                    `json:"-"`                       // Maximum number of retries
	DefaultHeaders http.Header            `json:"-"`                       // Default headers
	DefaultQuery   map[string]string      `json:"-"`                       // Default query parameters
	HTTPClient     *http.Client           `json:"-"`                       // Custom HTTP client
	ClientParams   map[string]interface{} `json:"client_params,omitempty"` // Additional client parameters
	// Additional fields for chat requests
	Model            string   // Model to be used
	Temperature      *float32 // Response temperature
	MaxTokens        *int     // Maximum number of tokens
	TopP             *float32 // Top-P parameter
	FrequencyPenalty *float32 // Frequency penalty
	PresencePenalty  *float32 // Presence penalty
}

// DefaultOptions returns the default options for the Gemini API client
func DefaultOptions() *ClientOptions {
	return &ClientOptions{
		Model:            "gemini-2.0-flash-lite",
		Temperature:      floatPtr(0.3),
		MaxTokens:        intPtr(1024),
		TopP:             floatPtr(1.0),
		FrequencyPenalty: floatPtr(0.0),
		PresencePenalty:  floatPtr(0.0),
	}
}

// WithModel sets the model to be used
func WithModel(model string) func(*ClientOptions) {
	return func(o *ClientOptions) {
		o.Model = model
	}
}

// WithAPIKey sets the API key for the client
func WithAPIKey(key string) func(*ClientOptions) {
	return func(o *ClientOptions) {
		o.APIKey = key
	}
}

// Helper functions to create pointers for optional fields
func boolPtr(b bool) *bool        { return &b }
func floatPtr(f float32) *float32 { return &f }
func intPtr(i int) *int           { return &i }
func strPtr(s string) *string     { return &s }
