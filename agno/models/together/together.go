// Package together provides integration with the Together AI API.
// Together AI provides access to state-of-the-art open-source models
// through a unified API, compatible with the OpenAI API format.
//
// Since Together AI is fully compatible with the OpenAI API, this package
// uses the existing OpenAI-like implementation internally.
package together

import (
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the default base URL for the Together AI API
	DefaultBaseURL = "https://api.together.xyz/v1"
)

// NewTogetherChat creates a new instance of the integration with the Together AI API.
// Together AI is compatible with the OpenAI API, so this function uses the
// existing OpenAI-like implementation with Together AI's base URL.
//
// Example usage:
//
//	chat, err := together.NewTogetherChat(
//	    models.WithID("meta-llama/Meta-Llama-3.1-8B-Instruct-Turbo"),
//	    models.WithAPIKey("your-together-api-key"),
//	)
//
// If no API key is provided, it will look for the TOGETHER_API_KEY environment variable.
//
// Available models can be found at: https://docs.together.ai/docs/inference-models
func NewTogetherChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
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
		apiKey = os.Getenv("TOGETHER_API_KEY")
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

// Popular Together AI model constants for convenience
const (
	// Meta Llama 3.1 Models (Recommended for tool calling)
	ModelLlama318BInstruct     = "meta-llama/Meta-Llama-3.1-8B-Instruct-Turbo"
	ModelLlama3170BInstruct    = "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo"
	ModelLlama31405BInstruct   = "meta-llama/Meta-Llama-3.1-405B-Instruct-Turbo"
	ModelLlama3170BReference   = "meta-llama/Meta-Llama-3.1-70B-Instruct-Reference"
	ModelLlama31405BReference  = "meta-llama/Meta-Llama-3.1-405B-Instruct-Reference"

	// Meta Llama 3.2 Models
	ModelLlama323BInstruct     = "meta-llama/Llama-3.2-3B-Instruct-Turbo"
	ModelLlama3211BVisionInstruct = "meta-llama/Llama-3.2-11B-Vision-Instruct-Turbo"
	ModelLlama3290BVisionInstruct = "meta-llama/Llama-3.2-90B-Vision-Instruct-Turbo"

	// Meta Llama 3 Models
	ModelLlama38BInstruct      = "meta-llama/Llama-3-8b-chat-hf"
	ModelLlama370BInstruct     = "meta-llama/Llama-3-70b-chat-hf"

	// Qwen Models (Excellent for coding and tool calling)
	ModelQwen25Coder32BInstruct = "Qwen/Qwen2.5-Coder-32B-Instruct"
	ModelQwen257BInstruct      = "Qwen/Qwen2.5-7B-Instruct-Turbo"
	ModelQwen2572BInstruct     = "Qwen/Qwen2.5-72B-Instruct-Turbo"

	// Mistral Models
	ModelMistral7BInstruct     = "mistralai/Mistral-7B-Instruct-v0.3"
	ModelMixtral8x7BInstruct   = "mistralai/Mixtral-8x7B-Instruct-v0.1"
	ModelMixtral8x22BInstruct  = "mistralai/Mixtral-8x22B-Instruct-v0.1"

	// DeepSeek Models
	ModelDeepSeekCoderV2Instruct = "deepseek-ai/deepseek-coder-33b-instruct"
	ModelDeepSeekLLM67BChat      = "deepseek-ai/deepseek-llm-67b-chat"

	// Google Gemma Models
	ModelGemma2B               = "google/gemma-2b-it"
	ModelGemma7B               = "google/gemma-7b-it"
	ModelGemma29BInstruct      = "google/gemma-2-9b-it"
	ModelGemma227BInstruct     = "google/gemma-2-27b-it"

	// Microsoft Phi Models
	ModelPhi3Medium14BInstruct = "microsoft/Phi-3-medium-4k-instruct"

	// Databricks DBRX Model
	ModelDBRXInstruct          = "databricks/dbrx-instruct"

	// NousResearch Models
	ModelHermes2ProMistral7B   = "NousResearch/Nous-Hermes-2-Mistral-7B-DPO"
	ModelHermes2Mixtral8x7B    = "NousResearch/Nous-Hermes-2-Mixtral-8x7B-DPO"

	// WizardLM Models
	ModelWizardLM213B          = "WizardLMTeam/WizardLM-13B-V1.2"

	// OpenChat Models
	ModelOpenChat357B          = "openchat/openchat-3.5-0106"
)
