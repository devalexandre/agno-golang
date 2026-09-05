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

// VoyageEmbedder uses the Voyage AI embeddings API.
type VoyageEmbedder struct {
	BaseEmbedder
	APIKey     string
	BaseURL    string
	Model      string
	InputType  string // "query" | "document" | "" (none)
	HTTPClient *http.Client
	Timeout    time.Duration
}

// NewVoyageEmbedder creates a Voyage AI embedder. apiKey falls back to
// VOYAGE_API_KEY. Default model is voyage-3 (1024).
func NewVoyageEmbedder(apiKey, model string, dimensions int) *VoyageEmbedder {
	if apiKey == "" {
		apiKey = os.Getenv("VOYAGE_API_KEY")
	}
	if model == "" {
		model = "voyage-3"
	}
	if dimensions <= 0 {
		dimensions = 1024
	}
	return &VoyageEmbedder{
		BaseEmbedder: BaseEmbedder{ID: model, Dimensions: dimensions},
		APIKey:       apiKey,
		BaseURL:      "https://api.voyageai.com/v1",
		Model:        model,
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
		Timeout:      30 * time.Second,
	}
}

type voyageEmbedRequest struct {
	Input     []string `json:"input"`
	Model     string   `json:"model"`
	InputType string   `json:"input_type,omitempty"`
}

type voyageEmbedResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
	Model string `json:"model"`
}

func (e *VoyageEmbedder) embed(text string) ([]float64, map[string]interface{}, error) {
	if text == "" {
		return nil, nil, ErrEmptyText
	}
	if e.APIKey == "" {
		return nil, nil, ErrAPIKeyMissing
	}

	reqBody := voyageEmbedRequest{
		Input:     []string{text},
		Model:     e.Model,
		InputType: e.InputType,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", e.BaseURL+"/embeddings", bytes.NewBuffer(payload))
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

	var out voyageEmbedResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	if len(out.Data) == 0 {
		return nil, nil, ErrInvalidResponse
	}
	usage := map[string]interface{}{
		"total_tokens": out.Usage.TotalTokens,
		"model":        out.Model,
	}
	return out.Data[0].Embedding, usage, nil
}

// GetEmbedding implements Embedder.
func (e *VoyageEmbedder) GetEmbedding(text string) ([]float64, error) {
	emb, _, err := e.embed(text)
	return emb, err
}

// GetEmbeddingAndUsage implements Embedder.
func (e *VoyageEmbedder) GetEmbeddingAndUsage(text string) ([]float64, map[string]interface{}, error) {
	return e.embed(text)
}
