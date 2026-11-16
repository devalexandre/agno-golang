package embedder

import "os"

// NewNebiusEmbedder creates an embedder backed by Nebius AI Studio's
// OpenAI-compatible /embeddings endpoint.
//
// When apiKey is empty it falls back to the NEBIUS_API_KEY environment variable.
// When model is empty the default below is used; set dimensions to match the
// chosen model (0 uses the default).
func NewNebiusEmbedder(apiKey, model string, dimensions int) *OpenAIEmbedder {
	if apiKey == "" {
		apiKey = os.Getenv("NEBIUS_API_KEY")
	}
	if model == "" {
		model = "BAAI/bge-en-icl"
	}
	if dimensions <= 0 {
		dimensions = 4096
	}
	return NewOpenAILikeEmbedder("https://api.studio.nebius.com/v1", model, apiKey, dimensions)
}
