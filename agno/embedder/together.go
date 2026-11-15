package embedder

import "os"

// NewTogetherEmbedder creates an embedder backed by Together AI's
// OpenAI-compatible /embeddings endpoint.
//
// When apiKey is empty it falls back to the TOGETHER_API_KEY environment variable.
// When model is empty the default below is used; set dimensions to match the
// chosen model (0 uses the default).
func NewTogetherEmbedder(apiKey, model string, dimensions int) *OpenAIEmbedder {
	if apiKey == "" {
		apiKey = os.Getenv("TOGETHER_API_KEY")
	}
	if model == "" {
		model = "togethercomputer/m2-bert-80M-8k-retrieval"
	}
	if dimensions <= 0 {
		dimensions = 768
	}
	return NewOpenAILikeEmbedder("https://api.together.xyz/v1", model, apiKey, dimensions)
}
