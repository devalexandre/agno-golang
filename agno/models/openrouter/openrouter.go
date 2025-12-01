// Package openrouter provides integration with the OpenRouter API.
// OpenRouter is a unified API that provides access to multiple LLM providers
// through a single endpoint, compatible with the OpenAI API format.
//
// Since OpenRouter is fully compatible with the OpenAI API, this package
// uses the existing OpenAI-like implementation internally.
package openrouter

import (
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the default base URL for the OpenRouter API
	DefaultBaseURL = "https://openrouter.ai/api/v1"
)

// NewOpenRouterChat creates a new instance of the integration with the OpenRouter API.
// OpenRouter is compatible with the OpenAI API, so this function uses the
// existing OpenAI-like implementation with OpenRouter's base URL.
//
// Example usage:
//
//	chat, err := openrouter.NewOpenRouterChat(
//	    models.WithID("openai/gpt-4-turbo"),
//	    models.WithAPIKey("your-openrouter-api-key"),
//	)
//
// If no API key is provided, it will look for the OPENROUTER_API_KEY environment variable.
//
// Available models can be found at: https://openrouter.ai/models
func NewOpenRouterChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	// Collect options to check what's been set
	opts := models.DefaultOptions()
	for _, option := range options {
		option(opts)
	}

	// Build the final options list - order matters!
	// Start with BaseURL
	finalOptions := []models.OptionClient{
		models.WithBaseURL(DefaultBaseURL),
	}

	// Get API key from environment if not provided
	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENROUTER_API_KEY")
	}
	if apiKey != "" {
		finalOptions = append(finalOptions, models.WithAPIKey(apiKey))
	}

	// Add model ID if provided
	if opts.ID != "" {
		finalOptions = append(finalOptions, models.WithID(opts.ID))
	}

	// Add any other options that were passed, but skip BaseURL, APIKey, and ID
	// to avoid duplicates and ensure our values take precedence
	for _, option := range options {
		// We need to check if this option is one we've already set
		// For now, we'll just add all options and let the last one win
		// This is a limitation of the functional options pattern
		finalOptions = append(finalOptions, option)
	}

	// Use the existing OpenAI-like implementation
	return like.NewLikeOpenAIChat(finalOptions...)
}

// Popular OpenRouter model constants for convenience
const (
	// OpenAI Models
	ModelGPT4Turbo  = "openai/gpt-4-turbo"
	ModelGPT4       = "openai/gpt-4"
	ModelGPT4o      = "openai/gpt-4o"
	ModelGPT4oMini  = "openai/gpt-4o-mini"
	ModelGPT35Turbo = "openai/gpt-3.5-turbo"
	ModelO1Preview  = "openai/o1-preview"
	ModelO1Mini     = "openai/o1-mini"

	// Anthropic Models
	ModelClaude3Opus    = "anthropic/claude-3-opus"
	ModelClaude3Sonnet  = "anthropic/claude-3-sonnet"
	ModelClaude3Haiku   = "anthropic/claude-3-haiku"
	ModelClaude35Sonnet = "anthropic/claude-3.5-sonnet"

	// Google Models
	ModelGeminiPro     = "google/gemini-pro"
	ModelGemini15Pro   = "google/gemini-1.5-pro"
	ModelGemini15Flash = "google/gemini-1.5-flash"

	// Meta Models
	ModelLlama370B   = "meta-llama/llama-3-70b-instruct"
	ModelLlama38B    = "meta-llama/llama-3-8b-instruct"
	ModelLlama3170B  = "meta-llama/llama-3.1-70b-instruct"
	ModelLlama318B   = "meta-llama/llama-3.1-8b-instruct"
	ModelLlama31405B = "meta-llama/llama-3.1-405b-instruct"

	// Mistral Models
	ModelMistralLarge  = "mistralai/mistral-large"
	ModelMistralMedium = "mistralai/mistral-medium"
	ModelMistral7B     = "mistralai/mistral-7b-instruct"
	ModelMixtral8x7B   = "mistralai/mixtral-8x7b-instruct"
	ModelMixtral8x22B  = "mistralai/mixtral-8x22b-instruct"

	// Cohere Models
	ModelCommandR     = "cohere/command-r"
	ModelCommandRPlus = "cohere/command-r-plus"

	// DeepSeek Models
	ModelDeepSeekChat  = "deepseek/deepseek-chat"
	ModelDeepSeekCoder = "deepseek/deepseek-coder"

	// Qwen Models
	ModelQwen72B     = "qwen/qwen-72b-chat"
	ModelQwen25Coder = "qwen/qwen-2.5-coder-32b-instruct"
)
