package client

import (
	"context"
	"net/http"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools"
)

type OpenAIRequest struct {
	Model               string                 `json:"model"`                           // Model to be used.
	Messages            []models.Message       `json:"messages"`                        // Conversation history.
	Tools               []tools.Tools          `json:"tools,omitempty"`                 // External tool calls.
	ToolChoice          string                 `json:"tool_choice,omitempty"`           // External tool call.
	Store               *bool                  `json:"store,omitempty"`                 // Store the output.
	ReasoningEffort     *string                `json:"reasoning_effort,omitempty"`      // Reasoning effort.
	Verbosity           *string                `json:"verbosity,omitempty"`             // Verbosity level.
	Metadata            map[string]interface{} `json:"metadata,omitempty"`              // Additional metadata.
	FrequencyPenalty    *float32               `json:"frequency_penalty,omitempty"`     // Frequency penalty.
	LogitBias           map[string]float32     `json:"logit_bias,omitempty"`            // Token logits bias.
	Logprobs            *int                   `json:"logprobs,omitempty"`              // Maximum number of logprobs per token.
	TopLogprobs         *int                   `json:"top_logprobs,omitempty"`          // Maximum number of top logprobs per token.
	MaxTokens           *int                   `json:"max_tokens,omitempty"`            // Maximum number of tokens in the response.
	MaxCompletionTokens *int                   `json:"max_completion_tokens,omitempty"` // Maximum number of tokens in the completion.
	Modalities          []string               `json:"modalities,omitempty"`            // Supported modalities.
	Audio               map[string]interface{} `json:"audio,omitempty"`                 // Audio data.
	PresencePenalty     *float32               `json:"presence_penalty,omitempty"`      // Presence penalty.
	ResponseFormat      interface{}            `json:"response_format,omitempty"`       // Response format.
	Seed                *int                   `json:"seed,omitempty"`                  // Seed for reproducibility.
	Stop                interface{}            `json:"stop,omitempty"`                  // Stop sequences.
	Temperature         *float32               `json:"temperature,omitempty"`           // Response temperature.
	TopP                *float32               `json:"top_p,omitempty"`                 // Top-P parameter.
	ExtraHeaders        http.Header            `json:"-"`                               // Additional headers.
	ExtraQuery          map[string]string      `json:"-"`                               // Additional query parameters.
	RequestParams       map[string]interface{} `json:"request_params,omitempty"`        // Additional request parameters.
	Stream              *bool                  `json:"stream,omitempty"`                // Whether the request is streaming.
}

// New type definitions for chat completion.
type Choices struct {
	Index        int                    `json:"index"`
	Message      models.MessageResponse `json:"message"`
	Logprobs     interface{}            `json:"logprobs"`
	FinishReason string                 `json:"finish_reason"`
	Delta        models.MessageResponse `json:"delta"`
}
type CompletionChunk struct {
	ID                string    `json:"id"`
	Object            string    `json:"object"`
	Created           int64     `json:"created"`
	Model             string    `json:"model"`
	SystemFingerprint string    `json:"system_fingerprint"`
	Choices           []Choices `json:"choices"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type CompletionResponse struct {
	ID      string    `json:"id"`
	Object  string    `json:"object"`
	Created int64     `json:"created"`
	Model   string    `json:"model"`
	Choices []Choices `json:"choices"`
	Usage   Usage     `json:"usage"`
}


// ClientInterface defines the interface for communication with the OpenAI API.
type ClientInterface interface {
	CreateChatCompletion(ctx context.Context, messages []models.Message, options ...models.Option) (*CompletionResponse, error)
	StreamChatCompletion(ctx context.Context, messages []models.Message, options ...models.Option) error
}

type ChatCompletionMessage = models.Message
type ChatCompletionResponse = CompletionResponse
type ChatCompletionChunk = CompletionChunk
type ChatCompletionRequest = OpenAIRequest

type ChatCompletionChoice = Choices
