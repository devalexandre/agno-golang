package chroma

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

// ChromaDB implements the VectorDB interface for Chroma
type ChromaDB struct {
	*vectordb.BaseVectorDB
	client     *http.Client
	baseURL    string
	collection string
	tenant     string
	database   string
}

// ChromaOptions configuration options
type ChromaOptions struct {
	Host       string
	Port       int
	Collection string
	Tenant     string
	Database   string
	Embedder   embedder.Embedder
}

// NewChromaDB creates a new ChromaDB instance
func NewChromaDB(opts ChromaOptions) *ChromaDB {
	if opts.Host == "" {
		opts.Host = "localhost"
	}
	if opts.Port == 0 {
		opts.Port = 8000
	}
	if opts.Collection == "" {
		opts.Collection = "agno_collection"
	}
	if opts.Tenant == "" {
		opts.Tenant = "default_tenant"
	}
	if opts.Database == "" {
		opts.Database = "default_database"
	}

	baseURL := fmt.Sprintf("http://%s:%d/api/v1", opts.Host, opts.Port)

	return &ChromaDB{
		BaseVectorDB: vectordb.NewBaseVectorDB(opts.Embedder, vectordb.SearchTypeVector, vectordb.DistanceCosine),
		client:       &http.Client{},
		baseURL:      baseURL,
		collection:   opts.Collection,
		tenant:       opts.Tenant,
		database:     opts.Database,
	}
}

// Create creates the collection
func (c *ChromaDB) Create(ctx context.Context) error {
	// Check if exists first
	exists, err := c.Exists(ctx)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	url := fmt.Sprintf("%s/collections?tenant=%s&database=%s", c.baseURL, c.tenant, c.database)

	payload := map[string]interface{}{
		"name": c.collection,
		"metadata": map[string]interface{}{
			"hnsw:space": "cosine",
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create collection: %s", string(bodyBytes))
	}

	return nil
}

// Exists checks if the collection exists
func (c *ChromaDB) Exists(ctx context.Context) (bool, error) {
	url := fmt.Sprintf("%s/collections/%s?tenant=%s&database=%s", c.baseURL, c.collection, c.tenant, c.database)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusInternalServerError { // Chroma sometimes returns 500 for not found
		return false, nil
	}

	return false, fmt.Errorf("failed to check collection existence: status %d", resp.StatusCode)
}

// Drop deletes the collection
func (c *ChromaDB) Drop(ctx context.Context) error {
	url := fmt.Sprintf("%s/collections/%s?tenant=%s&database=%s", c.baseURL, c.collection, c.tenant, c.database)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound { // Ignore if not found
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to drop collection: %s", string(bodyBytes))
	}

	return nil
}

// Optimize is a no-op for Chroma
func (c *ChromaDB) Optimize(ctx context.Context) error {
	return nil
}

// Insert inserts documents into the collection
func (c *ChromaDB) Insert(ctx context.Context, documents []*document.Document, filters map[string]interface{}) error {
	if err := c.EmbedDocuments(documents); err != nil {
		return err
	}

	collectionID, err := c.getCollectionID(ctx)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/collections/%s/add", c.baseURL, collectionID)

	ids := make([]string, len(documents))
	embeddings := make([][]float64, len(documents))
	metadatas := make([]map[string]interface{}, len(documents))
	documentsContent := make([]string, len(documents))

	for i, doc := range documents {
		ids[i] = doc.ID
		embeddings[i] = doc.Embeddings
		metadatas[i] = doc.Metadata
		documentsContent[i] = doc.Content
	}

	payload := map[string]interface{}{
		"ids":        ids,
		"embeddings": embeddings,
		"metadatas":  metadatas,
		"documents":  documentsContent,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to insert documents: %s", string(bodyBytes))
	}

	return nil
}

// Upsert is same as Insert for Chroma (it handles updates)
func (c *ChromaDB) Upsert(ctx context.Context, documents []*document.Document, filters map[string]interface{}) error {
	return c.Insert(ctx, documents, filters)
}

// Search performs a vector search
func (c *ChromaDB) Search(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return c.VectorSearch(ctx, query, limit, filters)
}

// VectorSearch performs a vector search
func (c *ChromaDB) VectorSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	queryEmbedding, err := c.EmbedQuery(query)
	if err != nil {
		return nil, err
	}

	collectionID, err := c.getCollectionID(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/collections/%s/query", c.baseURL, collectionID)

	payload := map[string]interface{}{
		"query_embeddings": [][]float64{queryEmbedding},
		"n_results":        limit,
	}

	if filters != nil {
		payload["where"] = filters
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to search: %s", string(bodyBytes))
	}

	var result struct {
		Ids       [][]string                 `json:"ids"`
		Distances [][]float64                `json:"distances"`
		Metadatas [][]map[string]interface{} `json:"metadatas"`
		Documents [][]string                 `json:"documents"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Ids) == 0 {
		return []*vectordb.SearchResult{}, nil
	}

	results := make([]*vectordb.SearchResult, 0)
	for i := range result.Ids[0] {
		doc := &document.Document{
			ID:       result.Ids[0][i],
			Content:  result.Documents[0][i],
			Metadata: result.Metadatas[0][i],
		}

		score := 1.0 - result.Distances[0][i] // Convert distance to similarity score approx

		results = append(results, &vectordb.SearchResult{
			Document: doc,
			Score:    score,
			Distance: result.Distances[0][i],
		})
	}

	return results, nil
}

// KeywordSearch is not supported natively by Chroma
func (c *ChromaDB) KeywordSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return nil, fmt.Errorf("keyword search not supported by ChromaDB")
}

// HybridSearch is not supported natively by Chroma
func (c *ChromaDB) HybridSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return nil, fmt.Errorf("hybrid search not supported by ChromaDB")
}

// GetCount returns the number of documents
func (c *ChromaDB) GetCount(ctx context.Context) (int64, error) {
	collectionID, err := c.getCollectionID(ctx)
	if err != nil {
		return 0, err
	}

	url := fmt.Sprintf("%s/collections/%s/count", c.baseURL, collectionID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to get count")
	}

	var count int64
	if err := json.NewDecoder(resp.Body).Decode(&count); err != nil {
		return 0, err
	}

	return count, nil
}

// Helper to get collection ID
func (c *ChromaDB) getCollectionID(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/collections/%s?tenant=%s&database=%s", c.baseURL, c.collection, c.tenant, c.database)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get collection ID")
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if id, ok := result["id"].(string); ok {
		return id, nil
	}

	return "", fmt.Errorf("collection ID not found in response")
}

// DocExists checks if a document exists
func (c *ChromaDB) DocExists(ctx context.Context, doc *document.Document) (bool, error) {
	return c.IDExists(ctx, doc.ID)
}

// NameExists checks if a document with name exists (using filter)
func (c *ChromaDB) NameExists(ctx context.Context, name string) (bool, error) {
	collectionID, err := c.getCollectionID(ctx)
	if err != nil {
		return false, err
	}

	url := fmt.Sprintf("%s/collections/%s/get", c.baseURL, collectionID)

	payload := map[string]interface{}{
		"where": map[string]interface{}{
			"name": name,
		},
		"limit": 1,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return false, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return false, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var result struct {
		Ids []string `json:"ids"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}

	return len(result.Ids) > 0, nil
}

// IDExists checks if a document with ID exists
func (c *ChromaDB) IDExists(ctx context.Context, id string) (bool, error) {
	collectionID, err := c.getCollectionID(ctx)
	if err != nil {
		return false, err
	}

	url := fmt.Sprintf("%s/collections/%s/get", c.baseURL, collectionID)

	payload := map[string]interface{}{
		"ids": []string{id},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return false, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return false, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var result struct {
		Ids []string `json:"ids"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}

	return len(result.Ids) > 0, nil
}
