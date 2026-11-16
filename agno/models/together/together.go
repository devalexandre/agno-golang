// Package together provides integration with the Together AI API, which is
// compatible with the OpenAI API format. It reuses the OpenAI-like
// implementation internally.
package together

import (
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for Together AI.
	DefaultBaseURL = "https://api.together.xyz/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "TOGETHER_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "meta-llama/Llama-3.3-70B-Instruct-Turbo"
)

// NewTogetherChat creates a new Together AI chat model.
//
//	chat, err := together.NewTogetherChat(
//	    models.WithID("meta-llama/Llama-3.3-70B-Instruct-Turbo"),
//	    models.WithAPIKey("your-together-api-key"), // or set TOGETHER_API_KEY
//	)
func NewTogetherChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	opts := &models.ClientOptions{}
	for _, option := range options {
		option(opts)
	}

	model := opts.ID
	if model == "" {
		model = DefaultModel
	}
	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = os.Getenv(EnvAPIKey)
	}

	final := []models.OptionClient{
		models.WithBaseURL(DefaultBaseURL),
		models.WithID(model),
	}
	if apiKey != "" {
		final = append(final, models.WithAPIKey(apiKey))
	}
	final = append(final, options...)

	return like.NewLikeOpenAIChat(final...)
}

// Popular Together AI model constants for convenience.
const (
	ModelLlama33_70BTurbo = "meta-llama/Llama-3.3-70B-Instruct-Turbo"
	ModelLlama31_8BTurbo  = "meta-llama/Meta-Llama-3.1-8B-Instruct-Turbo"
	ModelMixtral8x7B      = "mistralai/Mixtral-8x7B-Instruct-v0.1"
	ModelQwen2_5_72BTurbo = "Qwen/Qwen2.5-72B-Instruct-Turbo"
)

// Additional Together model constants (kept from the previous API).
const (
	// DefaultBaseURL is the default base URL for the Together AI API
	// Meta Llama 3.1 Models (Recommended for tool calling)
	ModelLlama318BInstruct    = "meta-llama/Meta-Llama-3.1-8B-Instruct-Turbo"
	ModelLlama3170BInstruct   = "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo"
	ModelLlama31405BInstruct  = "meta-llama/Meta-Llama-3.1-405B-Instruct-Turbo"
	ModelLlama3170BReference  = "meta-llama/Meta-Llama-3.1-70B-Instruct-Reference"
	ModelLlama31405BReference = "meta-llama/Meta-Llama-3.1-405B-Instruct-Reference"
	// Meta Llama 3.2 Models
	ModelLlama323BInstruct        = "meta-llama/Llama-3.2-3B-Instruct-Turbo"
	ModelLlama3211BVisionInstruct = "meta-llama/Llama-3.2-11B-Vision-Instruct-Turbo"
	ModelLlama3290BVisionInstruct = "meta-llama/Llama-3.2-90B-Vision-Instruct-Turbo"
	// Meta Llama 3 Models
	ModelLlama38BInstruct  = "meta-llama/Llama-3-8b-chat-hf"
	ModelLlama370BInstruct = "meta-llama/Llama-3-70b-chat-hf"
	// Qwen Models (Excellent for coding and tool calling)
	ModelQwen25Coder32BInstruct = "Qwen/Qwen2.5-Coder-32B-Instruct"
	ModelQwen257BInstruct       = "Qwen/Qwen2.5-7B-Instruct-Turbo"
	ModelQwen2572BInstruct      = "Qwen/Qwen2.5-72B-Instruct-Turbo"
	// Mistral Models
	ModelMistral7BInstruct    = "mistralai/Mistral-7B-Instruct-v0.3"
	ModelMixtral8x7BInstruct  = "mistralai/Mixtral-8x7B-Instruct-v0.1"
	ModelMixtral8x22BInstruct = "mistralai/Mixtral-8x22B-Instruct-v0.1"
	// DeepSeek Models
	ModelDeepSeekCoderV2Instruct = "deepseek-ai/deepseek-coder-33b-instruct"
	ModelDeepSeekLLM67BChat      = "deepseek-ai/deepseek-llm-67b-chat"
	// Google Gemma Models
	ModelGemma2B           = "google/gemma-2b-it"
	ModelGemma7B           = "google/gemma-7b-it"
	ModelGemma29BInstruct  = "google/gemma-2-9b-it"
	ModelGemma227BInstruct = "google/gemma-2-27b-it"
	// Microsoft Phi Models
	ModelPhi3Medium14BInstruct = "microsoft/Phi-3-medium-4k-instruct"
	// Databricks DBRX Model
	ModelDBRXInstruct = "databricks/dbrx-instruct"
	// NousResearch Models
	ModelHermes2ProMistral7B = "NousResearch/Nous-Hermes-2-Mistral-7B-DPO"
	ModelHermes2Mixtral8x7B  = "NousResearch/Nous-Hermes-2-Mixtral-8x7B-DPO"
	// WizardLM Models
	ModelWizardLM213B = "WizardLMTeam/WizardLM-13B-V1.2"
	// OpenChat Models
	ModelOpenChat357B = "openchat/openchat-3.5-0106"
)
