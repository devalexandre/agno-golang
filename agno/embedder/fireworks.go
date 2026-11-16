package embedder

import "os"

// NewFireworksEmbedder creates an embedder backed by Fireworks AI's
// OpenAI-compatible /embeddings endpoint.
//
// When apiKey is empty it falls back to the FIREWORKS_API_KEY environment variable.
// When model is empty the default below is used; set dimensions to match the
// chosen model (0 uses the default).
func NewFireworksEmbedder(apiKey, model string, dimensions int) *OpenAIEmbedder {
	if apiKey == "" {
		apiKey = os.Getenv("FIREWORKS_API_KEY")
	}
	if model == "" {
		model = "nomic-ai/nomic-embed-text-v1.5"
	}
	if dimensions <= 0 {
		dimensions = 768
	}
	return NewOpenAILikeEmbedder("https://api.fireworks.ai/inference/v1", model, apiKey, dimensions)
}
