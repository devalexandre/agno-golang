package embedder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaEmbedder embedder usando Ollama (local)
type OllamaEmbedder struct {
	BaseEmbedder
	Host       string
	Model      string
	HTTPClient *http.Client
	Timeout    time.Duration
	Options    map[string]interface{}
}

// OllamaEmbeddingRequest request structure for Ollama
type OllamaEmbeddingRequest struct {
	Model   string                 `json:"model"`
	Input   string                 `json:"input"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// OllamaEmbeddingResponse estrutura da resposta do Ollama
type OllamaEmbeddingResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
	Model      string      `json:"model"`
}

// NewOllamaEmbedder cria um novo embedder Ollama
func NewOllamaEmbedder(options ...func(*OllamaEmbedder)) *OllamaEmbedder {
	embedder := &OllamaEmbedder{
		BaseEmbedder: BaseEmbedder{
			ID:         "nomic-embed-text",
			Dimensions: 768,
		},
		Host:       "http://localhost:11434",
		Model:      "nomic-embed-text",
		HTTPClient: &http.Client{},
		Timeout:    60 * time.Second, // Embeddings podem demorar mais
	}

	// Apply options
	for _, option := range options {
		option(embedder)
	}

	// Configurar timeout no client HTTP
	embedder.HTTPClient.Timeout = embedder.Timeout

	return embedder
}

// WithOllamaHost configura o host do Ollama
func WithOllamaHost(host string) func(*OllamaEmbedder) {
	return func(e *OllamaEmbedder) {
		e.Host = host
	}
}

// WithOllamaModel configura o modelo
func WithOllamaModel(model string, dimensions int) func(*OllamaEmbedder) {
	return func(e *OllamaEmbedder) {
		e.Model = model
		e.ID = model
		e.Dimensions = dimensions
	}
}

// WithOllamaTimeout configura o timeout
func WithOllamaTimeout(timeout time.Duration) func(*OllamaEmbedder) {
	return func(e *OllamaEmbedder) {
		e.Timeout = timeout
	}
}

// WithOllamaOptions configures additional options
func WithOllamaOptions(options map[string]interface{}) func(*OllamaEmbedder) {
	return func(e *OllamaEmbedder) {
		e.Options = options
	}
}

// GetEmbedding gets embedding for a text
func (e *OllamaEmbedder) GetEmbedding(text string) ([]float64, error) {
	if text == "" {
		return nil, ErrEmptyText
	}

	request := OllamaEmbeddingRequest{
		Model:   e.Model,
		Input:   text,
		Options: e.Options,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/embed", e.Host)
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewBuffer(requestBody))
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

	// Log the raw response for debugging
	// fmt.Printf("[DEBUG] Ollama raw response: %s\n", string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response OllamaEmbeddingResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(response.Embeddings) == 0 {
		return nil, ErrInvalidResponse
	}

	embedding := response.Embeddings[0]

	// Validar dimensões
	if len(embedding) != e.Dimensions {
		return nil, fmt.Errorf("%w: expected %d, got %d", ErrInvalidDimension, e.Dimensions, len(embedding))
	}

	return embedding, nil
}

// GetEmbeddingAndUsage gets embedding and usage information
func (e *OllamaEmbedder) GetEmbeddingAndUsage(text string) ([]float64, map[string]interface{}, error) {
	embedding, err := e.GetEmbedding(text)
	if err != nil {
		return nil, nil, err
	}

	// Ollama não fornece informações de uso detalhadas
	usage := map[string]interface{}{
		"model":      e.Model,
		"dimensions": len(embedding),
	}

	return embedding, usage, nil
}
