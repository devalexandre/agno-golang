// Package milvus implements the vectordb.VectorDB interface for Milvus / Zilliz
// Cloud using the Milvus v2 REST API (no external client dependency).
package milvus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

// MilvusDB implements vectordb.VectorDB backed by Milvus.
type MilvusDB struct {
	*vectordb.BaseVectorDB
	client     *http.Client
	baseURL    string
	token      string
	collection string
	dimension  int
}

// Options configures a MilvusDB.
type Options struct {
	BaseURL    string // e.g. http://localhost:19530 or your Zilliz Cloud endpoint
	Token      string // "user:password" or API key (optional for local)
	Collection string // collection name (required)
	Dimension  int    // vector dimension (defaults to the embedder's)
	Embedder   embedder.Embedder
}

// New creates a new MilvusDB.
func New(opts Options) *MilvusDB {
	if opts.BaseURL == "" {
		opts.BaseURL = "http://localhost:19530"
	}
	opts.BaseURL = strings.TrimRight(opts.BaseURL, "/")
	if opts.Collection == "" {
		opts.Collection = "agno_documents"
	}
	dim := opts.Dimension
	if dim <= 0 && opts.Embedder != nil {
		dim = opts.Embedder.GetDimensions()
	}
	return &MilvusDB{
		BaseVectorDB: vectordb.NewBaseVectorDB(opts.Embedder, vectordb.SearchTypeVector, vectordb.DistanceCosine),
		client:       &http.Client{},
		baseURL:      opts.BaseURL,
		token:        opts.Token,
		collection:   opts.Collection,
		dimension:    dim,
	}
}

// post calls a Milvus v2 REST endpoint and decodes the standard {code,data} envelope.
func (m *MilvusDB) post(ctx context.Context, path string, payload interface{}, out interface{}) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", m.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if m.token != "" {
		req.Header.Set("Authorization", "Bearer "+m.token)
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("milvus: %s: %s", path, string(body))
	}
	// Milvus wraps errors in {"code": N, "message": "..."} even on HTTP 200.
	var env struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return err
	}
	if env.Code != 0 {
		return fmt.Errorf("milvus: %s: code %d: %s", path, env.Code, env.Message)
	}
	if out != nil && len(env.Data) > 0 {
		return json.Unmarshal(env.Data, out)
	}
	return nil
}

// Create creates the collection (id, vector, content) if it does not exist.
func (m *MilvusDB) Create(ctx context.Context) error {
	if exists, _ := m.Exists(ctx); exists {
		return nil
	}
	if m.dimension <= 0 {
		return fmt.Errorf("milvus: dimension is required to create a collection")
	}
	payload := map[string]interface{}{
		"collectionName": m.collection,
		"dimension":      m.dimension,
		"metricType":     "COSINE",
		"primaryFieldName": "id",
		"vectorFieldName":  "vector",
		"idType":           "VarChar",
		"params":           map[string]interface{}{"max_length": 512},
	}
	return m.post(ctx, "/v2/vectordb/collections/create", payload, nil)
}

// Exists reports whether the collection exists.
func (m *MilvusDB) Exists(ctx context.Context) (bool, error) {
	var names []string
	if err := m.post(ctx, "/v2/vectordb/collections/list", map[string]interface{}{}, &names); err != nil {
		return false, err
	}
	for _, n := range names {
		if n == m.collection {
			return true, nil
		}
	}
	return false, nil
}

// Drop deletes the collection.
func (m *MilvusDB) Drop(ctx context.Context) error {
	return m.post(ctx, "/v2/vectordb/collections/drop", map[string]interface{}{"collectionName": m.collection}, nil)
}

// Optimize is a no-op for Milvus.
func (m *MilvusDB) Optimize(ctx context.Context) error { return nil }

// Insert embeds and inserts documents.
func (m *MilvusDB) Insert(ctx context.Context, docs []*document.Document, _ map[string]interface{}) error {
	if err := m.EmbedDocuments(docs); err != nil {
		return err
	}
	rows := make([]map[string]interface{}, 0, len(docs))
	for _, d := range docs {
		rows = append(rows, map[string]interface{}{
			"id":      d.ID,
			"vector":  d.Embeddings,
			"content": d.Content,
		})
	}
	return m.post(ctx, "/v2/vectordb/entities/insert",
		map[string]interface{}{"collectionName": m.collection, "data": rows}, nil)
}

// Upsert embeds and upserts documents.
func (m *MilvusDB) Upsert(ctx context.Context, docs []*document.Document, _ map[string]interface{}) error {
	if err := m.EmbedDocuments(docs); err != nil {
		return err
	}
	rows := make([]map[string]interface{}, 0, len(docs))
	for _, d := range docs {
		rows = append(rows, map[string]interface{}{
			"id":      d.ID,
			"vector":  d.Embeddings,
			"content": d.Content,
		})
	}
	return m.post(ctx, "/v2/vectordb/entities/upsert",
		map[string]interface{}{"collectionName": m.collection, "data": rows}, nil)
}

// Search performs a vector search.
func (m *MilvusDB) Search(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return m.VectorSearch(ctx, query, limit, filters)
}

// VectorSearch runs an ANN search over the collection.
func (m *MilvusDB) VectorSearch(ctx context.Context, query string, limit int, _ map[string]interface{}) ([]*vectordb.SearchResult, error) {
	vec, err := m.EmbedQuery(query)
	if err != nil {
		return nil, err
	}
	payload := map[string]interface{}{
		"collectionName": m.collection,
		"data":           []interface{}{vec},
		"limit":          limit,
		"outputFields":   []string{"content"},
		"annsField":      "vector",
	}
	var rows []struct {
		ID       string  `json:"id"`
		Distance float64 `json:"distance"`
		Content  string  `json:"content"`
	}
	if err := m.post(ctx, "/v2/vectordb/entities/search", payload, &rows); err != nil {
		return nil, err
	}
	results := make([]*vectordb.SearchResult, 0, len(rows))
	for _, r := range rows {
		results = append(results, &vectordb.SearchResult{
			Document: &document.Document{ID: r.ID, Content: r.Content},
			Score:    r.Distance, // COSINE: higher is more similar
			Distance: r.Distance,
		})
	}
	return results, nil
}

// KeywordSearch is not supported by this Milvus implementation.
func (m *MilvusDB) KeywordSearch(ctx context.Context, query string, limit int, _ map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return nil, fmt.Errorf("milvus: keyword search not supported")
}

// HybridSearch is not supported by this Milvus implementation.
func (m *MilvusDB) HybridSearch(ctx context.Context, query string, limit int, _ map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return nil, fmt.Errorf("milvus: hybrid search not supported")
}

// GetCount returns the number of entities in the collection.
func (m *MilvusDB) GetCount(ctx context.Context) (int64, error) {
	var rows []map[string]interface{}
	err := m.post(ctx, "/v2/vectordb/entities/query", map[string]interface{}{
		"collectionName": m.collection,
		"filter":         "",
		"outputFields":   []string{"count(*)"},
	}, &rows)
	if err != nil {
		return 0, err
	}
	if len(rows) > 0 {
		if c, ok := rows[0]["count(*)"].(float64); ok {
			return int64(c), nil
		}
	}
	return 0, nil
}

// DocExists checks existence by the document's ID.
func (m *MilvusDB) DocExists(ctx context.Context, doc *document.Document) (bool, error) {
	return m.IDExists(ctx, doc.ID)
}

// NameExists is not supported.
func (m *MilvusDB) NameExists(ctx context.Context, name string) (bool, error) {
	return false, fmt.Errorf("milvus: NameExists not supported")
}

// IDExists checks whether an entity with the given primary key exists.
func (m *MilvusDB) IDExists(ctx context.Context, id string) (bool, error) {
	var rows []map[string]interface{}
	err := m.post(ctx, "/v2/vectordb/entities/get", map[string]interface{}{
		"collectionName": m.collection,
		"id":             []string{id},
		"outputFields":   []string{"id"},
	}, &rows)
	if err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}

var _ vectordb.VectorDB = (*MilvusDB)(nil)
