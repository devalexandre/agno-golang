package openai

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
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
	ToolCall            []toolkit.Tool                      `json:"-"`                               // Tools for function calls.
}

func WithTools(tool []toolkit.Tool) Option {
	var _tools []tools.Tools
	for _, t := range tool {
		for methodName := range t.GetMethods() {
			toolConverted := tools.ConvertToTools(t, methodName)
			_tools = append(_tools, toolConverted)
		}
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

// DefaultCallOptions returns the default options for the request.
func DefaultCallOptions() *models.CallOptions {
	return &models.CallOptions{
		Temperature:         floatPtr(0.7),
		MaxTokens:           nil, // Default to no limit
		MaxCompletionTokens: nil, // Default to no limit
		TopP:                floatPtr(1.0),
		FrequencyPenalty:    floatPtr(0.0),
		PresencePenalty:     floatPtr(0.0),
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
