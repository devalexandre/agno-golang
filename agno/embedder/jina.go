package embedder

import "os"

// NewJinaEmbedder creates an embedder backed by Jina AI's
// OpenAI-compatible /embeddings endpoint.
//
// When apiKey is empty it falls back to the JINA_API_KEY environment variable.
// When model is empty the default below is used; set dimensions to match the
// chosen model (0 uses the default).
func NewJinaEmbedder(apiKey, model string, dimensions int) *OpenAIEmbedder {
	if apiKey == "" {
		apiKey = os.Getenv("JINA_API_KEY")
	}
	if model == "" {
		model = "jina-embeddings-v3"
	}
	if dimensions <= 0 {
		dimensions = 1024
	}
	return NewOpenAILikeEmbedder("https://api.jina.ai/v1", model, apiKey, dimensions)
}
