package embedder

import (
	"net/http"
	"time"
)

// NewOpenAILikeEmbedder creates an embedder for any service exposing an
// OpenAI-compatible /embeddings endpoint. Many providers (Mistral, Jina,
// Together, Nebius, Fireworks, Hugging Face router, vLLM) are covered by this.
//
// dimensions must match the chosen model's output size, since it is reported by
// GetDimensions() and used to size vectors in the vector database.
func NewOpenAILikeEmbedder(baseURL, model, apiKey string, dimensions int) *OpenAIEmbedder {
	return &OpenAIEmbedder{
		BaseEmbedder: BaseEmbedder{
			ID:         model,
			Dimensions: dimensions,
		},
		APIKey:     apiKey,
		BaseURL:    baseURL,
		Model:      model,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		Timeout:    30 * time.Second,
	}
}

// firstNonEmpty returns the first non-empty string, or "" if all are empty.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
