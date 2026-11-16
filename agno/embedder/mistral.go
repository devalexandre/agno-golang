package embedder

import "os"

// NewMistralEmbedder creates an embedder backed by Mistral AI's
// OpenAI-compatible /embeddings endpoint.
//
// When apiKey is empty it falls back to the MISTRAL_API_KEY environment variable.
// When model is empty the default below is used; set dimensions to match the
// chosen model (0 uses the default).
func NewMistralEmbedder(apiKey, model string, dimensions int) *OpenAIEmbedder {
	if apiKey == "" {
		apiKey = os.Getenv("MISTRAL_API_KEY")
	}
	if model == "" {
		model = "mistral-embed"
	}
	if dimensions <= 0 {
		dimensions = 1024
	}
	return NewOpenAILikeEmbedder("https://api.mistral.ai/v1", model, apiKey, dimensions)
}
