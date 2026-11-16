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

// GoogleEmbedder uses the Google Generative Language (Gemini) embeddings API.
type GoogleEmbedder struct {
	BaseEmbedder
	APIKey     string
	BaseURL    string
	Model      string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// NewGoogleEmbedder creates a Gemini embedder. apiKey falls back to
// GOOGLE_API_KEY then GEMINI_API_KEY. Default model is text-embedding-004 (768).
func NewGoogleEmbedder(apiKey, model string, dimensions int) *GoogleEmbedder {
	if apiKey == "" {
		apiKey = firstNonEmpty(os.Getenv("GOOGLE_API_KEY"), os.Getenv("GEMINI_API_KEY"))
	}
	if model == "" {
		model = "text-embedding-004"
	}
	if dimensions <= 0 {
		dimensions = 768
	}
	return &GoogleEmbedder{
		BaseEmbedder: BaseEmbedder{ID: model, Dimensions: dimensions},
		APIKey:       apiKey,
		BaseURL:      "https://generativelanguage.googleapis.com/v1beta",
		Model:        model,
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
		Timeout:      30 * time.Second,
	}
}

type googleEmbedRequest struct {
	Model   string `json:"model"`
	Content struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"content"`
}

type googleEmbedResponse struct {
	Embedding struct {
		Values []float64 `json:"values"`
	} `json:"embedding"`
}

func (e *GoogleEmbedder) embed(text string) ([]float64, error) {
	if text == "" {
		return nil, ErrEmptyText
	}
	if e.APIKey == "" {
		return nil, ErrAPIKeyMissing
	}

	modelPath := e.Model
	if !strings.HasPrefix(modelPath, "models/") {
		modelPath = "models/" + modelPath
	}

	var reqBody googleEmbedRequest
	reqBody.Model = modelPath
	reqBody.Content.Parts = []struct {
		Text string `json:"text"`
	}{{Text: text}}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/%s:embedContent?key=%s", e.BaseURL, modelPath, e.APIKey)
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var out googleEmbedResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	if len(out.Embedding.Values) == 0 {
		return nil, ErrInvalidResponse
	}
	return out.Embedding.Values, nil
}

// GetEmbedding implements Embedder.
func (e *GoogleEmbedder) GetEmbedding(text string) ([]float64, error) {
	return e.embed(text)
}

// GetEmbeddingAndUsage implements Embedder. Gemini's embedContent does not
// return token usage, so usage is nil.
func (e *GoogleEmbedder) GetEmbeddingAndUsage(text string) ([]float64, map[string]interface{}, error) {
	emb, err := e.embed(text)
	return emb, nil, err
}
