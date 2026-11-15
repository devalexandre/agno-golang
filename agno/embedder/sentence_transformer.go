package embedder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// SentenceTransformerEmbedder is the Go counterpart of Agno Python's
// SentenceTransformerEmbedder. Python runs the model in-process; Go cannot, so
// this embedder calls a server hosting the same sentence-transformers model
// through the Hugging Face Text Embeddings Inference (TEI) API
// (POST {BaseURL}/embed), e.g.:
//
//	docker run -p 8080:80 ghcr.io/huggingface/text-embeddings-inference:cpu-1.5 \
//	    --model-id sentence-transformers/all-MiniLM-L6-v2
type SentenceTransformerEmbedder struct {
	BaseEmbedder
	// BaseURL of the TEI server; defaults to SENTENCE_TRANSFORMERS_BASE_URL,
	// then TEI_BASE_URL, then http://localhost:8080.
	BaseURL string
	// APIKey is sent as a Bearer token when set (TEI --api-key, HF endpoints).
	APIKey string
	// Prompt is prepended to every input, like sentence-transformers' prompt.
	Prompt string
	// NormalizeEmbeddings asks the server for L2-normalised vectors.
	NormalizeEmbeddings bool
	// Truncate asks the server to truncate inputs longer than the model limit.
	Truncate   bool
	HTTPClient *http.Client
}

// NewSentenceTransformerEmbedder creates the embedder. model defaults to
// sentence-transformers/all-MiniLM-L6-v2 (384 dimensions); it only labels the
// embedder, since a TEI server serves one model.
func NewSentenceTransformerEmbedder(baseURL, model string, dimensions int) *SentenceTransformerEmbedder {
	baseURL = firstNonEmpty(baseURL, os.Getenv("SENTENCE_TRANSFORMERS_BASE_URL"), os.Getenv("TEI_BASE_URL"), "http://localhost:8080")
	if model == "" {
		model = "sentence-transformers/all-MiniLM-L6-v2"
	}
	if dimensions <= 0 {
		dimensions = 384
	}
	return &SentenceTransformerEmbedder{
		BaseEmbedder: BaseEmbedder{ID: model, Dimensions: dimensions},
		BaseURL:      strings.TrimRight(baseURL, "/"),
		APIKey:       os.Getenv("TEI_API_KEY"),
		Truncate:     true,
		HTTPClient:   &http.Client{Timeout: 60 * time.Second},
	}
}

type teiEmbedRequest struct {
	Inputs    []string `json:"inputs"`
	Normalize bool     `json:"normalize"`
	Truncate  bool     `json:"truncate"`
}

// GetEmbedding implements Embedder.
func (e *SentenceTransformerEmbedder) GetEmbedding(text string) ([]float64, error) {
	if text == "" {
		return nil, ErrEmptyText
	}
	payload, err := json.Marshal(teiEmbedRequest{Inputs: []string{e.Prompt + text}, Normalize: e.NormalizeEmbeddings, Truncate: e.Truncate})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, e.BaseURL+"/embed", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.APIKey)
	}
	client := e.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sentence-transformer embedder: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sentence-transformer embedder: status %d: %s", resp.StatusCode, string(body))
	}
	var out [][]float64
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("sentence-transformer embedder: decode response: %w", err)
	}
	if len(out) == 0 || len(out[0]) == 0 {
		return nil, ErrInvalidResponse
	}
	return out[0], nil
}

// GetEmbeddingAndUsage implements Embedder (TEI reports no usage).
func (e *SentenceTransformerEmbedder) GetEmbeddingAndUsage(text string) ([]float64, map[string]interface{}, error) {
	emb, err := e.GetEmbedding(text)
	return emb, nil, err
}

var _ Embedder = (*SentenceTransformerEmbedder)(nil)
