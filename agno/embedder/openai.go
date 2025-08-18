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

// OpenAIEmbedder embedder usando OpenAI API
type OpenAIEmbedder struct {
	BaseEmbedder
	APIKey       string
	BaseURL      string
	Organization string
	Model        string
	User         string
	HTTPClient   *http.Client
	Timeout      time.Duration
}

// OpenAIEmbeddingRequest request structure for OpenAI
type OpenAIEmbeddingRequest struct {
	Input          string `json:"input"`
	Model          string `json:"model"`
	EncodingFormat string `json:"encoding_format,omitempty"`
	Dimensions     *int   `json:"dimensions,omitempty"`
	User           string `json:"user,omitempty"`
}

// OpenAIEmbeddingResponse estrutura da resposta da OpenAI
type OpenAIEmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// NewOpenAIEmbedder cria um novo embedder OpenAI
func NewOpenAIEmbedder(options ...func(*OpenAIEmbedder)) *OpenAIEmbedder {
	embedder := &OpenAIEmbedder{
		BaseEmbedder: BaseEmbedder{
			ID:         "text-embedding-3-small",
			Dimensions: 1536,
		},
		APIKey:     os.Getenv("OPENAI_API_KEY"),
		BaseURL:    "https://api.openai.com/v1",
		Model:      "text-embedding-3-small",
		HTTPClient: &http.Client{},
		Timeout:    30 * time.Second,
	}

	// Apply options
	for _, option := range options {
		option(embedder)
	}

	// Configurar timeout no client HTTP
	embedder.HTTPClient.Timeout = embedder.Timeout

	// Adjust dimensions based on model
	if embedder.Model == "text-embedding-3-large" {
		embedder.Dimensions = 3072
	}

	return embedder
}

// WithAPIKey configura a API key
func WithAPIKey(apiKey string) func(*OpenAIEmbedder) {
	return func(e *OpenAIEmbedder) {
		e.APIKey = apiKey
	}
}

// WithModel configura o modelo
func WithModel(model string) func(*OpenAIEmbedder) {
	return func(e *OpenAIEmbedder) {
		e.Model = model
		e.ID = model
		// Adjust dimensions
		if model == "text-embedding-3-large" {
			e.Dimensions = 3072
		} else {
			e.Dimensions = 1536
		}
	}
}

// WithDimensions configures dimensions (only for text-embedding-3 models)
func WithDimensions(dimensions int) func(*OpenAIEmbedder) {
	return func(e *OpenAIEmbedder) {
		e.Dimensions = dimensions
	}
}

// WithBaseURL configura a URL base
func WithBaseURL(baseURL string) func(*OpenAIEmbedder) {
	return func(e *OpenAIEmbedder) {
		e.BaseURL = baseURL
	}
}

// WithOrganization configures the organization
func WithOrganization(organization string) func(*OpenAIEmbedder) {
	return func(e *OpenAIEmbedder) {
		e.Organization = organization
	}
}

// WithTimeout configura o timeout
func WithTimeout(timeout time.Duration) func(*OpenAIEmbedder) {
	return func(e *OpenAIEmbedder) {
		e.Timeout = timeout
	}
}

// GetEmbedding gets embedding for a text
func (e *OpenAIEmbedder) GetEmbedding(text string) ([]float64, error) {
	if text == "" {
		return nil, ErrEmptyText
	}

	if e.APIKey == "" {
		return nil, ErrAPIKeyMissing
	}

	request := OpenAIEmbeddingRequest{
		Input:          text,
		Model:          e.Model,
		EncodingFormat: "float",
		User:           e.User,
	}

	// Add dimensions only for text-embedding-3 models
	if strings.HasPrefix(e.Model, "text-embedding-3") {
		request.Dimensions = &e.Dimensions
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/embeddings", e.BaseURL)
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.APIKey))
	if e.Organization != "" {
		req.Header.Set("OpenAI-Organization", e.Organization)
	}

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

	var response OpenAIEmbeddingResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(response.Data) == 0 {
		return nil, ErrInvalidResponse
	}

	return response.Data[0].Embedding, nil
}

// GetEmbeddingAndUsage gets embedding and usage information
func (e *OpenAIEmbedder) GetEmbeddingAndUsage(text string) ([]float64, map[string]interface{}, error) {
	request := OpenAIEmbeddingRequest{
		Input:          text,
		Model:          e.Model,
		EncodingFormat: "float",
		User:           e.User,
	}

	// Add dimensions only for text-embedding-3 models
	if strings.HasPrefix(e.Model, "text-embedding-3") {
		request.Dimensions = &e.Dimensions
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/embeddings", e.BaseURL)
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.APIKey))
	if e.Organization != "" {
		req.Header.Set("OpenAI-Organization", e.Organization)
	}

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

	var response OpenAIEmbeddingResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(response.Data) == 0 {
		return nil, nil, ErrInvalidResponse
	}

	usage := map[string]interface{}{
		"prompt_tokens": response.Usage.PromptTokens,
		"total_tokens":  response.Usage.TotalTokens,
		"model":         response.Model,
	}

	return response.Data[0].Embedding, usage, nil
}
