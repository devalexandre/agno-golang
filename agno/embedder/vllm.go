package embedder

import "os"

// NewVLLMEmbedder creates an embedder backed by a self-hosted vLLM server
// exposing an OpenAI-compatible /embeddings endpoint.
//
// baseURL defaults to the VLLM_BASE_URL env var, then http://localhost:8000/v1.
// apiKey falls back to VLLM_API_KEY (vLLM may not require one). Set model and
// dimensions to match your served embedding model.
func NewVLLMEmbedder(baseURL, model, apiKey string, dimensions int) *OpenAIEmbedder {
	baseURL = firstNonEmpty(baseURL, os.Getenv("VLLM_BASE_URL"), "http://localhost:8000/v1")
	if apiKey == "" {
		apiKey = os.Getenv("VLLM_API_KEY")
	}
	if model == "" {
		model = "BAAI/bge-small-en-v1.5"
	}
	if dimensions <= 0 {
		dimensions = 384
	}
	return NewOpenAILikeEmbedder(baseURL, model, apiKey, dimensions)
}
