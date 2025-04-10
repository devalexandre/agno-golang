package models

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/devalexandre/agno-golang/agno/tools"
)

// CallOptions defines common options that can be applied to both models (OpenAI and Gemini).
type CallOptions struct {
	Store               *bool                               `json:"store,omitempty"`
	ReasoningEffort     *string                             `json:"reasoning_effort,omitempty"`
	Metadata            map[string]interface{}              `json:"metadata,omitempty"`
	FrequencyPenalty    *float32                            `json:"frequency_penalty,omitempty"`
	LogitBias           map[string]float32                  `json:"logit_bias,omitempty"`
	Logprobs            *int                                `json:"logprobs,omitempty"`
	TopLogprobs         *int                                `json:"top_logprobs,omitempty"`
	MaxTokens           *int                                `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int                                `json:"max_completion_tokens,omitempty"`
	Modalities          []string                            `json:"modalities,omitempty"`
	Audio               map[string]interface{}              `json:"audio,omitempty"`
	PresencePenalty     *float32                            `json:"presence_penalty,omitempty"`
	ResponseFormat      interface{}                         `json:"response_format,omitempty"`
	Seed                *int                                `json:"seed,omitempty"`
	Stop                interface{}                         `json:"stop,omitempty"`
	Stream              *bool                               `json:"stream,omitempty"`
	Temperature         *float32                            `json:"temperature,omitempty"`
	TopP                *float32                            `json:"top_p,omitempty"`
	ExtraHeaders        map[string]string                   `json:"-"`
	ExtraQuery          map[string]string                   `json:"-"`
	RequestParams       map[string]interface{}              `json:"request_params,omitempty"`
	StreamingFunc       func(context.Context, []byte) error `json:"-"` // Callback function for streaming
	ToolCall            []tools.Tool                        `json:"-"` // Tools for function calls
	Tools               []tools.Tools                       `json:"tools,omitempty"`
}

// Option is a function that modifies CallOptions.
type Option func(*CallOptions)

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

// WithStore specifies if the output should be stored.
func WithStore(store bool) Option {
	return func(o *CallOptions) {
		o.Store = boolPtr(store)
	}
}

// WithStreamingFunc adds a callback function for processing streaming chunks.
func WithStreamingFunc(f func(context.Context, []byte) error) Option {
	return func(o *CallOptions) {
		o.StreamingFunc = f
		o.Stream = boolPtr(true)
	}
}

// WithTools adds tools to the request
func WithTools(tool []tools.Tool) Option {
	return func(o *CallOptions) {
		o.ToolCall = tool
	}
}

// Helper functions to create pointers for optional fields.
func boolPtr(b bool) *bool        { return &b }
func floatPtr(f float32) *float32 { return &f }
func intPtr(i int) *int           { return &i }
func strPtr(s string) *string     { return &s }

// WithReasoningEffort sets the reasoning effort
func WithReasoningEffort(reasoningEffort string) Option {
	return func(o *CallOptions) {
		o.ReasoningEffort = strPtr(reasoningEffort)
	}
}

// WithMetadata sets the additional metadata
func WithMetadata(metadata map[string]interface{}) Option {
	return func(o *CallOptions) {
		o.Metadata = metadata
	}
}

// WithFrequencyPenalty sets the frequency penalty
func WithFrequencyPenalty(penalty float32) Option {
	return func(o *CallOptions) {
		o.FrequencyPenalty = floatPtr(penalty)
	}
}

// WithLogitBias sets the token logit bias
func WithLogitBias(logitBias map[string]float32) Option {
	return func(o *CallOptions) {
		o.LogitBias = logitBias
	}
}

// WithLogprobs sets the maximum number of logprobs per token
func WithLogprobs(logprobs int) Option {
	return func(o *CallOptions) {
		o.Logprobs = intPtr(logprobs)
	}
}

// WithTopLogprobs sets the maximum number of top logprobs per token
func WithTopLogprobs(topLogprobs int) Option {
	return func(o *CallOptions) {
		o.TopLogprobs = intPtr(topLogprobs)
	}
}

// WithMaxTokens sets the maximum number of tokens in the response
func WithMaxTokens(tokens int) Option {
	return func(o *CallOptions) {
		o.MaxTokens = intPtr(tokens)
	}
}

// WithMaxCompletionTokens sets the maximum number of tokens in the completion
func WithMaxCompletionTokens(tokens int) Option {
	return func(o *CallOptions) {
		o.MaxCompletionTokens = intPtr(tokens)
	}
}

// WithModalities sets the supported modalities
func WithModalities(modalities []string) Option {
	return func(o *CallOptions) {
		o.Modalities = modalities
	}
}

// WithAudio sets the audio data
func WithAudio(audio map[string]interface{}) Option {
	return func(o *CallOptions) {
		o.Audio = audio
	}
}

// WithPresencePenalty sets the presence penalty
func WithPresencePenalty(penalty float32) Option {
	return func(o *CallOptions) {
		o.PresencePenalty = floatPtr(penalty)
	}
}

// WithResponseFormat sets the response format
func WithResponseFormat(format interface{}) Option {
	return func(o *CallOptions) {
		o.ResponseFormat = format
	}
}

// WithSeed sets the seed for reproducibility
func WithSeed(seed int) Option {
	return func(o *CallOptions) {
		o.Seed = intPtr(seed)
	}
}

// WithStop sets the stop sequences
func WithStop(stop interface{}) Option {
	return func(o *CallOptions) {
		o.Stop = stop
	}
}

// WithTemperature sets the response temperature
func WithTemperature(temp float32) Option {
	return func(o *CallOptions) {
		o.Temperature = floatPtr(temp)
	}
}

// WithTopP sets the Top-P parameter
func WithTopP(topP float32) Option {
	return func(o *CallOptions) {
		o.TopP = floatPtr(topP)
	}
}

// WithExtraHeaders sets additional headers
func WithExtraHeaders(headers http.Header) Option {
	return func(o *CallOptions) {
		o.ExtraHeaders = make(map[string]string)
		for k, v := range headers {
			if len(v) > 0 {
				o.ExtraHeaders[k] = v[0]
			}
		}
	}
}

// WithExtraQuery sets additional query parameters
func WithExtraQuery(query map[string]string) Option {
	return func(o *CallOptions) {
		o.ExtraQuery = query
	}
}

// WithRequestParams sets additional request parameters
func WithRequestParams(params map[string]interface{}) Option {
	return func(o *CallOptions) {
		o.RequestParams = params
	}
}

func (o *CallOptions) MarshalJSON() ([]byte, error) {
	type Alias CallOptions
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(o),
	})
}
