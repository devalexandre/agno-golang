package embedder

import "os"

// NewHuggingFaceEmbedder creates an embedder backed by the Hugging Face router's
// OpenAI-compatible /embeddings endpoint.
//
// When apiKey is empty it falls back to the HF_TOKEN environment variable.
// When model is empty the default below is used; set dimensions to match the
// chosen model (0 uses the default).
func NewHuggingFaceEmbedder(apiKey, model string, dimensions int) *OpenAIEmbedder {
	if apiKey == "" {
		apiKey = os.Getenv("HF_TOKEN")
	}
	if model == "" {
		model = "intfloat/multilingual-e5-large"
	}
	if dimensions <= 0 {
		dimensions = 1024
	}
	return NewOpenAILikeEmbedder("https://router.huggingface.co/v1", model, apiKey, dimensions)
}
