package openai

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/devalexandre/agno-golang/agno/tools"
)

// CallOptions represents the options for making a request to the OpenAI API.
type CallOptions struct {
	Store               *bool                               `json:"store,omitempty"`                 // Output storage.
	ReasoningEffort     *string                             `json:"reasoning_effort,omitempty"`      // Reasoning effort.
	Metadata            map[string]interface{}              `json:"metadata,omitempty"`              // Additional metadata.
	FrequencyPenalty    *float32                            `json:"frequency_penalty,omitempty"`     // Frequency penalty.
	LogitBias           map[string]float32                  `json:"logit_bias,omitempty"`            // Token logit bias.
	Logprobs            *int                                `json:"logprobs,omitempty"`              // Maximum number of logprobs per token.
	TopLogprobs         *int                                `json:"top_logprobs,omitempty"`          // Maximum number of top logprobs per token.
	MaxTokens           *int                                `json:"max_tokens,omitempty"`            // Maximum number of tokens in the response.
	MaxCompletionTokens *int                                `json:"max_completion_tokens,omitempty"` // Maximum number of tokens in the completion.
	Modalities          []string                            `json:"modalities,omitempty"`            // Supported modalities.
	Audio               map[string]interface{}              `json:"audio,omitempty"`                 // Audio data.
	PresencePenalty     *float32                            `json:"presence_penalty,omitempty"`      // Presence penalty.
	ResponseFormat      interface{}                         `json:"response_format,omitempty"`       // Response format.
	Seed                *int                                `json:"seed,omitempty"`                  // Seed for reproducibility.
	Stop                interface{}                         `json:"stop,omitempty"`                  // Stop sequences.
	Stream              *bool                               `json:"stream,omitempty"`                // Indicates if the request is streaming.
	Temperature         *float32                            `json:"temperature,omitempty"`           // Response temperature.
	TopP                *float32                            `json:"top_p,omitempty"`                 // Top-P parameter.
	ExtraHeaders        http.Header                         `json:"-"`                               // Additional headers.
	ExtraQuery          map[string]string                   `json:"-"`                               // Additional query parameters.
	RequestParams       map[string]interface{}              `json:"request_params,omitempty"`        // Additional request parameters.
	StreamingFunc       func(context.Context, []byte) error `json:"-"`                               // Callback function for streaming.
	Tools               []tools.Tools                       `json:"tools,omitempty"`                 // Tools for function calls.
	ToolCall            []tools.Tool                        `json:"-"`                               // Tools for function calls.
}

func WithTools(tool []tools.Tool) Option {
	var _tools []tools.Tools
	for _, t := range tool {
		toolConverted := tools.ConvertToTools(t)
		_tools = append(_tools, toolConverted)
	}

	return func(o *CallOptions) {
		o.ToolCall = tool
		o.Tools = _tools
	}
}

// WithStreamingFunc adds a callback function for processing streaming chunks.
// Setting this option will make the request be performed in streaming mode.
func WithStreamingFunc(f func(context.Context, []byte) error) Option {
	return func(o *CallOptions) {
		o.StreamingFunc = f
		o.Stream = boolPtr(true)
	}
}

// ClientOptions represents the options for the OpenAI API client.
type ClientOptions struct {
	APIKey         string                 `json:"-"`                       // API key.
	Organization   string                 `json:"-"`                       // Associated organization.
	BaseURL        string                 `json:"-"`                       // Base API URL.
	Timeout        int                    `json:"-"`                       // Request timeout.
	MaxRetries     int                    `json:"-"`                       // Maximum number of retries.
	DefaultHeaders http.Header            `json:"-"`                       // Default headers.
	DefaultQuery   map[string]string      `json:"-"`                       // Default query parameters.
	HTTPClient     *http.Client           `json:"-"`                       // Custom HTTP client.
	ClientParams   map[string]interface{} `json:"client_params,omitempty"` // Additional client parameters.
	// Additional fields for chat requests.
	Model            string   // Modelo a ser usado.
	Temperature      *float32 // Temperatura da resposta.
	MaxTokens        *int     // Número máximo de tokens.
	TopP             *float32 // Parâmetro Top-P.
	FrequencyPenalty *float32 // Penalidade de frequência.
	PresencePenalty  *float32 // Penalidade de presença.
}

// DefaultCallOptions returns the default options for the request.
func DefaultCallOptions() *CallOptions {
	return &CallOptions{
		Temperature:      floatPtr(0.7),
		MaxTokens:        intPtr(100),
		TopP:             floatPtr(1.0),
		FrequencyPenalty: floatPtr(0.0),
		PresencePenalty:  floatPtr(0.0),
	}
}

// DefaultOptions returns the default options for the OpenAI API client.
func DefaultOptions() *ClientOptions {
	return &ClientOptions{
		Model:            "gpt-3.5-turbo",
		Temperature:      floatPtr(0.7),
		MaxTokens:        intPtr(100),
		TopP:             floatPtr(1.0),
		FrequencyPenalty: floatPtr(0.0),
		PresencePenalty:  floatPtr(0.0),
	}
}

// WithModel sets the model to be used.
func WithModel(model string) func(*ClientOptions) {
	return func(o *ClientOptions) {
		o.Model = model
	}
}

// WithAPIKey sets the API key for the client.
func WithAPIKey(key string) func(*ClientOptions) {
	return func(o *ClientOptions) {
		o.APIKey = key
	}
}

// Option is a function that modifies the options.
type Option func(*CallOptions)

// WithStore specifies if the output should be stored.
func WithStore(store bool) Option {
	return func(o *CallOptions) {
		o.Store = boolPtr(store)
	}
}

// WithReasoningEffort sets the reasoning effort.
func WithReasoningEffort(reasoningEffort string) Option {
	return func(o *CallOptions) {
		o.ReasoningEffort = strPtr(reasoningEffort)
	}
}

// WithMetadata sets the additional metadata.
func WithMetadata(metadata map[string]interface{}) Option {
	return func(o *CallOptions) {
		o.Metadata = metadata
	}
}

// WithFrequencyPenalty sets the frequency penalty.
func WithFrequencyPenalty(penalty float32) Option {
	return func(o *CallOptions) {
		o.FrequencyPenalty = floatPtr(penalty)
	}
}

// WithLogitBias sets the token logit bias.
func WithLogitBias(logitBias map[string]float32) Option {
	return func(o *CallOptions) {
		o.LogitBias = logitBias
	}
}

// WithLogprobs sets the maximum number of logprobs per token.
func WithLogprobs(logprobs int) Option {
	return func(o *CallOptions) {
		o.Logprobs = intPtr(logprobs)
	}
}

// WithTopLogprobs sets the maximum number of top logprobs per token.
func WithTopLogprobs(topLogprobs int) Option {
	return func(o *CallOptions) {
		o.TopLogprobs = intPtr(topLogprobs)
	}
}

// WithMaxTokens sets the maximum number of tokens in the response.
func WithMaxTokens(tokens int) Option {
	return func(o *CallOptions) {
		o.MaxTokens = intPtr(tokens)
	}
}

// WithMaxCompletionTokens sets the maximum number of tokens in the completion.
func WithMaxCompletionTokens(tokens int) Option {
	return func(o *CallOptions) {
		o.MaxCompletionTokens = intPtr(tokens)
	}
}

// WithModalities sets the supported modalities.
func WithModalities(modalities []string) Option {
	return func(o *CallOptions) {
		o.Modalities = modalities
	}
}

// WithAudio sets the audio data.
func WithAudio(audio map[string]interface{}) Option {
	return func(o *CallOptions) {
		o.Audio = audio
	}
}

// WithPresencePenalty sets the presence penalty.
func WithPresencePenalty(penalty float32) Option {
	return func(o *CallOptions) {
		o.PresencePenalty = floatPtr(penalty)
	}
}

// WithResponseFormat sets the response format.
func WithResponseFormat(format interface{}) Option {
	return func(o *CallOptions) {
		o.ResponseFormat = format
	}
}

// WithSeed sets the seed for reproducibility.
func WithSeed(seed int) Option {
	return func(o *CallOptions) {
		o.Seed = intPtr(seed)
	}
}

// WithStop sets the stop sequences.
func WithStop(stop interface{}) Option {
	return func(o *CallOptions) {
		o.Stop = stop
	}
}

// WithTemperature sets the response temperature.
func WithTemperature(temp float32) Option {
	return func(o *CallOptions) {
		o.Temperature = floatPtr(temp)
	}
}

// WithTopP sets the Top-P parameter.
func WithTopP(topP float32) Option {
	return func(o *CallOptions) {
		o.TopP = floatPtr(topP)
	}
}

// WithExtraHeaders sets additional headers.
func WithExtraHeaders(headers http.Header) Option {
	return func(o *CallOptions) {
		o.ExtraHeaders = headers
	}
}

// WithExtraQuery sets additional query parameters.
func WithExtraQuery(query map[string]string) Option {
	return func(o *CallOptions) {
		o.ExtraQuery = query
	}
}

// WithRequestParams sets additional request parameters.
func WithRequestParams(params map[string]interface{}) Option {
	return func(o *CallOptions) {
		o.RequestParams = params
	}
}

// Helper functions to create pointers for optional fields.
func boolPtr(b bool) *bool        { return &b }
func floatPtr(f float32) *float32 { return &f }
func intPtr(i int) *int           { return &i }
func strPtr(s string) *string     { return &s }

// MarshalJSON implements custom serialization for CallOptions.
func (o *CallOptions) MarshalJSON() ([]byte, error) {
	type Alias CallOptions
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(o),
	})
}
