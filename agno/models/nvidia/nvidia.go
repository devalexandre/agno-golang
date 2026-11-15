// Package nvidia provides integration with the NVIDIA NIM API, which is
// compatible with the OpenAI API format. It reuses the OpenAI-like
// implementation internally.
package nvidia

import (
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the OpenAI-compatible endpoint for NVIDIA NIM.
	DefaultBaseURL = "https://integrate.api.nvidia.com/v1"
	// EnvAPIKey is the environment variable read when no API key is provided.
	EnvAPIKey = "NVIDIA_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "meta/llama-3.3-70b-instruct"
)

// NewNvidiaChat creates a new NVIDIA NIM chat model.
//
//	chat, err := nvidia.NewNvidiaChat(
//	    models.WithID("meta/llama-3.3-70b-instruct"),
//	    models.WithAPIKey("your-nvidia-api-key"), // or set NVIDIA_API_KEY
//	)
func NewNvidiaChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
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

// Popular NVIDIA NIM model constants for convenience.
const (
	ModelLlama33_70B = "meta/llama-3.3-70b-instruct"
	ModelLlama31_8B  = "meta/llama-3.1-8b-instruct"
	ModelMixtral8x7B = "mistralai/mixtral-8x7b-instruct-v0.1"
)
