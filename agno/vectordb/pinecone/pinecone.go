package pinecone

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

// PineconeDB implements the VectorDB interface for Pinecone
type PineconeDB struct {
	*vectordb.BaseVectorDB
	client    *http.Client
	apiKey    string
	indexURL  string
	namespace string
}

// PineconeOptions configuration options
type PineconeOptions struct {
	APIKey    string
	IndexURL  string // The full URL of your index (e.g., https://index-name-project.svc.pinecone.io)
	Namespace string
	Embedder  embedder.Embedder
}

// NewPineconeDB creates a new PineconeDB instance
func NewPineconeDB(opts PineconeOptions) *PineconeDB {
	return &PineconeDB{
		BaseVectorDB: vectordb.NewBaseVectorDB(opts.Embedder, vectordb.SearchTypeVector, vectordb.DistanceCosine),
		client:       &http.Client{},
		apiKey:       opts.APIKey,
		indexURL:     opts.IndexURL,
		namespace:    opts.Namespace,
	}
}

// Create is a no-op for Pinecone (indexes are managed via console/control plane API)
func (p *PineconeDB) Create(ctx context.Context) error {
	return nil
}

// Exists checks if we can connect to the index
func (p *PineconeDB) Exists(ctx context.Context) (bool, error) {
	url := fmt.Sprintf("%s/describe_index_stats", p.indexURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Api-Key", p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	return false, nil
}

// Drop is a no-op for Pinecone (safety)
func (p *PineconeDB) Drop(ctx context.Context) error {
	return nil
}

// Optimize is a no-op for Pinecone
func (p *PineconeDB) Optimize(ctx context.Context) error {
	return nil
}

// Insert inserts documents into the index
func (p *PineconeDB) Insert(ctx context.Context, documents []*document.Document, filters map[string]interface{}) error {
	if err := p.EmbedDocuments(documents); err != nil {
		return err
	}

	url := fmt.Sprintf("%s/vectors/upsert", p.indexURL)

	type Vector struct {
		ID       string                 `json:"id"`
		Values   []float64              `json:"values"`
		Metadata map[string]interface{} `json:"metadata"`
	}

	vectors := make([]Vector, len(documents))
	for i, doc := range documents {
		// Ensure metadata contains content for retrieval
		meta := doc.Metadata
		if meta == nil {
			meta = make(map[string]interface{})
		}
		meta["_content"] = doc.Content

		vectors[i] = Vector{
			ID:       doc.ID,
			Values:   doc.Embeddings,
			Metadata: meta,
		}
	}

	payload := map[string]interface{}{
		"vectors": vectors,
	}
	if p.namespace != "" {
		payload["namespace"] = p.namespace
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Api-Key", p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to insert vectors: %s", string(bodyBytes))
	}

	return nil
}

// Upsert is same as Insert for Pinecone
func (p *PineconeDB) Upsert(ctx context.Context, documents []*document.Document, filters map[string]interface{}) error {
	return p.Insert(ctx, documents, filters)
}

// Search performs a vector search
func (p *PineconeDB) Search(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return p.VectorSearch(ctx, query, limit, filters)
}

// VectorSearch performs a vector search
func (p *PineconeDB) VectorSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	queryEmbedding, err := p.EmbedQuery(query)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/query", p.indexURL)

	payload := map[string]interface{}{
		"vector":          queryEmbedding,
		"topK":            limit,
		"includeMetadata": true,
	}
	if p.namespace != "" {
		payload["namespace"] = p.namespace
	}
	if filters != nil {
		payload["filter"] = filters
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Api-Key", p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to search: %s", string(bodyBytes))
	}

	var result struct {
		Matches []struct {
			ID       string                 `json:"id"`
			Score    float64                `json:"score"`
			Metadata map[string]interface{} `json:"metadata"`
		} `json:"matches"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	results := make([]*vectordb.SearchResult, 0, len(result.Matches))
	for _, match := range result.Matches {
		content := ""
		if c, ok := match.Metadata["_content"].(string); ok {
			content = c
		}

		doc := &document.Document{
			ID:       match.ID,
			Content:  content,
			Metadata: match.Metadata,
		}

		results = append(results, &vectordb.SearchResult{
			Document: doc,
			Score:    match.Score,
			Distance: 0, // Pinecone returns score directly
		})
	}

	return results, nil
}

// KeywordSearch is not supported
func (p *PineconeDB) KeywordSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return nil, fmt.Errorf("keyword search not supported by Pinecone")
}

// HybridSearch is not supported in this basic implementation
func (p *PineconeDB) HybridSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return nil, fmt.Errorf("hybrid search not supported in this implementation")
}

// GetCount returns the number of documents
func (p *PineconeDB) GetCount(ctx context.Context) (int64, error) {
	url := fmt.Sprintf("%s/describe_index_stats", p.indexURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Api-Key", p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to get stats")
	}

	var result struct {
		TotalVectorCount int64 `json:"totalVectorCount"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	return result.TotalVectorCount, nil
}

// DocExists checks if a document exists
func (p *PineconeDB) DocExists(ctx context.Context, doc *document.Document) (bool, error) {
	return p.IDExists(ctx, doc.ID)
}

// NameExists checks if a document with name exists
func (p *PineconeDB) NameExists(ctx context.Context, name string) (bool, error) {
	// Not easily supported without fetching
	return false, fmt.Errorf("NameExists not supported by Pinecone")
}

// IDExists checks if a document with ID exists
func (p *PineconeDB) IDExists(ctx context.Context, id string) (bool, error) {
	url := fmt.Sprintf("%s/vectors/fetch?ids=%s", p.indexURL, id)
	if p.namespace != "" {
		url += fmt.Sprintf("&namespace=%s", p.namespace)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Api-Key", p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	var result struct {
		Vectors map[string]interface{} `json:"vectors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}

	_, exists := result.Vectors[id]
	return exists, nil
}
