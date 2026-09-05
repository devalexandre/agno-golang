// Package huggingface provides integration with the Hugging Face Inference
// Router, which exposes an OpenAI-compatible API. It reuses the OpenAI-like
// implementation internally.
package huggingface

import (
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible Hugging Face router endpoint.
	DefaultBaseURL = "https://router.huggingface.co/v1"
	// EnvAPIKey is the primary environment variable for the HF token.
	EnvAPIKey = "HF_TOKEN"
	// EnvAPIKeyAlt is an alternative environment variable for the HF token.
	EnvAPIKeyAlt = "HUGGINGFACE_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "meta-llama/Llama-3.3-70B-Instruct"
)

// NewHuggingFaceChat creates a new Hugging Face router chat model.
//
//	chat, err := huggingface.NewHuggingFaceChat(
//	    models.WithID("meta-llama/Llama-3.3-70B-Instruct"),
//	    models.WithAPIKey("hf_..."), // or set HF_TOKEN / HUGGINGFACE_API_KEY
//	)
func NewHuggingFaceChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
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
	if apiKey == "" {
		apiKey = os.Getenv(EnvAPIKeyAlt)
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

// Popular Hugging Face model constants for convenience.
const (
	ModelLlama33_70B = "meta-llama/Llama-3.3-70B-Instruct"
	ModelQwen2_5_72B = "Qwen/Qwen2.5-72B-Instruct"
	ModelMixtral8x7B = "mistralai/Mixtral-8x7B-Instruct-v0.1"
)
