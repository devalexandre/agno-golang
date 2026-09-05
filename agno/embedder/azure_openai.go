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

// AzureOpenAIEmbedder uses Azure OpenAI's deployment-based embeddings endpoint.
// It differs from OpenAI in auth (api-key header) and URL layout (deployment +
// api-version), but returns the same response shape.
type AzureOpenAIEmbedder struct {
	BaseEmbedder
	APIKey     string
	Endpoint   string // e.g. https://my-resource.openai.azure.com
	Deployment string // the embeddings deployment name
	APIVersion string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// NewAzureOpenAIEmbedder creates an Azure OpenAI embedder.
//
// apiKey     -> AZURE_OPENAI_API_KEY
// endpoint   -> AZURE_OPENAI_ENDPOINT
// deployment -> the deployment name of your embeddings model
// apiVersion -> defaults to 2023-05-15
// dimensions -> defaults to 1536 (text-embedding-3-small / ada-002)
func NewAzureOpenAIEmbedder(apiKey, endpoint, deployment, apiVersion string, dimensions int) *AzureOpenAIEmbedder {
	if apiKey == "" {
		apiKey = os.Getenv("AZURE_OPENAI_API_KEY")
	}
	if endpoint == "" {
		endpoint = os.Getenv("AZURE_OPENAI_ENDPOINT")
	}
	endpoint = strings.TrimRight(endpoint, "/")
	if apiVersion == "" {
		apiVersion = "2023-05-15"
	}
	if dimensions <= 0 {
		dimensions = 1536
	}
	return &AzureOpenAIEmbedder{
		BaseEmbedder: BaseEmbedder{ID: deployment, Dimensions: dimensions},
		APIKey:       apiKey,
		Endpoint:     endpoint,
		Deployment:   deployment,
		APIVersion:   apiVersion,
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
		Timeout:      30 * time.Second,
	}
}

func (e *AzureOpenAIEmbedder) embed(text string) ([]float64, map[string]interface{}, error) {
	if text == "" {
		return nil, nil, ErrEmptyText
	}
	if e.APIKey == "" {
		return nil, nil, ErrAPIKeyMissing
	}
	if e.Endpoint == "" || e.Deployment == "" {
		return nil, nil, fmt.Errorf("azure: endpoint and deployment are required")
	}

	payload, err := json.Marshal(OpenAIEmbeddingRequest{Input: text, EncodingFormat: "float"})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/openai/deployments/%s/embeddings?api-version=%s", e.Endpoint, e.Deployment, e.APIVersion)
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", e.APIKey)

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

	var out OpenAIEmbeddingResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	if len(out.Data) == 0 {
		return nil, nil, ErrInvalidResponse
	}
	usage := map[string]interface{}{
		"prompt_tokens": out.Usage.PromptTokens,
		"total_tokens":  out.Usage.TotalTokens,
		"model":         out.Model,
	}
	return out.Data[0].Embedding, usage, nil
}

// GetEmbedding implements Embedder.
func (e *AzureOpenAIEmbedder) GetEmbedding(text string) ([]float64, error) {
	emb, _, err := e.embed(text)
	return emb, err
}

// GetEmbeddingAndUsage implements Embedder.
func (e *AzureOpenAIEmbedder) GetEmbeddingAndUsage(text string) ([]float64, map[string]interface{}, error) {
	return e.embed(text)
}
