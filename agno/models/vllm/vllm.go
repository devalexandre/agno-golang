package vllm

import (
	"os"
	"github.com/devalexandre/agno-golang/agno/models"
	likeopenai "github.com/devalexandre/agno-golang/agno/models/openai/like"
)


const (
	// DefaultBaseURL é o endpoint padrão para vLLM
	DefaultBaseURL = "http://localhost:8000/v1"
)


// NewVLLMProvider creates a new vLLM provider instance
// NewVLLMProvider cria uma instância do provider vLLM
// Exemplo de uso:
//
//  vllm, err := vllm.NewVLLMProvider(
//      models.WithID("EssentialAI/rnj-1-instruct"),
//      models.WithAPIKey("your-vllm-api-key"),
//      models.WithBaseURL("https://z2bg1juojbhurv-8000.proxy.runpod.net/v1"),
//  )
//
// Se nenhuma API key for fornecida, será usada a variável de ambiente VLLM_API_KEY.
func NewVLLMProvider(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	opts := models.DefaultOptions()
	for _, option := range options {
		option(opts)
	}

	finalOptions := []models.OptionClient{
		models.WithBaseURL(DefaultBaseURL),
	}

	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("VLLM_API_KEY")
	}
	if apiKey != "" {
		finalOptions = append(finalOptions, models.WithAPIKey(apiKey))
	}

	if opts.ID != "" {
		finalOptions = append(finalOptions, models.WithID(opts.ID))
	}

	for _, option := range options {
		finalOptions = append(finalOptions, option)
	}

	return likeopenai.NewLikeOpenAIChat(finalOptions...)
}

// Modelos populares vLLM
const (
	ModelEssentialAI_RNJ1 = "EssentialAI/rnj-1-instruct"
)
