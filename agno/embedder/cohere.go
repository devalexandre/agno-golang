package embedder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// CohereEmbedder uses the Cohere Embed API.
type CohereEmbedder struct {
	BaseEmbedder
	APIKey     string
	BaseURL    string
	Model      string
	InputType  string // search_document | search_query | classification | clustering
	HTTPClient *http.Client
	Timeout    time.Duration
}

// NewCohereEmbedder creates a Cohere embedder. apiKey falls back to
// COHERE_API_KEY. Default model is embed-english-v3.0 (1024).
func NewCohereEmbedder(apiKey, model string, dimensions int) *CohereEmbedder {
	if apiKey == "" {
		apiKey = os.Getenv("COHERE_API_KEY")
	}
	if model == "" {
		model = "embed-english-v3.0"
	}
	if dimensions <= 0 {
		dimensions = 1024
	}
	return &CohereEmbedder{
		BaseEmbedder: BaseEmbedder{ID: model, Dimensions: dimensions},
		APIKey:       apiKey,
		BaseURL:      "https://api.cohere.ai/v1",
		Model:        model,
		InputType:    "search_document",
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
		Timeout:      30 * time.Second,
	}
}

type cohereEmbedRequest struct {
	Texts          []string `json:"texts"`
	Model          string   `json:"model"`
	InputType      string   `json:"input_type"`
	EmbeddingTypes []string `json:"embedding_types,omitempty"`
}

type cohereEmbedResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
	Meta       struct {
		BilledUnits struct {
			InputTokens int `json:"input_tokens"`
		} `json:"billed_units"`
	} `json:"meta"`
}

func (e *CohereEmbedder) embed(text string) ([]float64, map[string]interface{}, error) {
	if text == "" {
		return nil, nil, ErrEmptyText
	}
	if e.APIKey == "" {
		return nil, nil, ErrAPIKeyMissing
	}

	reqBody := cohereEmbedRequest{
		Texts:     []string{text},
		Model:     e.Model,
		InputType: e.InputType,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", e.BaseURL+"/embed", bytes.NewBuffer(payload))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.APIKey)

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var out cohereEmbedResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	if len(out.Embeddings) == 0 {
		return nil, nil, ErrInvalidResponse
	}
	usage := map[string]interface{}{
		"input_tokens": out.Meta.BilledUnits.InputTokens,
		"model":        e.Model,
	}
	return out.Embeddings[0], usage, nil
}

// GetEmbedding implements Embedder.
func (e *CohereEmbedder) GetEmbedding(text string) ([]float64, error) {
	emb, _, err := e.embed(text)
	return emb, err
}

// GetEmbeddingAndUsage implements Embedder.
func (e *CohereEmbedder) GetEmbeddingAndUsage(text string) ([]float64, map[string]interface{}, error) {
	return e.embed(text)
}
