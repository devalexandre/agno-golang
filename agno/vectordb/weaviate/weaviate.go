// Package weaviate implements the vectordb.VectorDB interface for Weaviate
// using its REST + GraphQL HTTP API (no external client dependency).
package weaviate

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

// WeaviateDB implements vectordb.VectorDB backed by Weaviate.
type WeaviateDB struct {
	*vectordb.BaseVectorDB
	client  *http.Client
	baseURL string
	apiKey  string
	class   string // Weaviate class name (collection)
}

// Options configures a WeaviateDB.
type Options struct {
	BaseURL  string // e.g. http://localhost:8080
	APIKey   string // optional (Weaviate Cloud)
	Class    string // class/collection name (required)
	Embedder embedder.Embedder
}

// New creates a new WeaviateDB.
func New(opts Options) *WeaviateDB {
	if opts.BaseURL == "" {
		opts.BaseURL = "http://localhost:8080"
	}
	opts.BaseURL = strings.TrimRight(opts.BaseURL, "/")
	if opts.Class == "" {
		opts.Class = "AgnoDocuments"
	}
	return &WeaviateDB{
		BaseVectorDB: vectordb.NewBaseVectorDB(opts.Embedder, vectordb.SearchTypeVector, vectordb.DistanceCosine),
		client:       &http.Client{},
		baseURL:      opts.BaseURL,
		apiKey:       opts.APIKey,
		class:        opts.Class,
	}
}

func (w *WeaviateDB) do(ctx context.Context, method, path string, payload interface{}) (*http.Response, error) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, w.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if w.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+w.apiKey)
	}
	return w.client.Do(req)
}

// Create creates the Weaviate class with a content property and no vectorizer
// (vectors are provided by the configured embedder).
func (w *WeaviateDB) Create(ctx context.Context) error {
	if exists, _ := w.Exists(ctx); exists {
		return nil
	}
	schema := map[string]interface{}{
		"class":      w.class,
		"vectorizer": "none",
		"properties": []map[string]interface{}{
			{"name": "content", "dataType": []string{"text"}},
			{"name": "name", "dataType": []string{"text"}},
			{"name": "source", "dataType": []string{"text"}},
		},
	}
	resp, err := w.do(ctx, "POST", "/v1/schema", schema)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("weaviate: create class: %s", string(b))
	}
	return nil
}

// Exists reports whether the class exists.
func (w *WeaviateDB) Exists(ctx context.Context) (bool, error) {
	resp, err := w.do(ctx, "GET", "/v1/schema/"+w.class, nil)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}

// Drop deletes the class.
func (w *WeaviateDB) Drop(ctx context.Context) error {
	resp, err := w.do(ctx, "DELETE", "/v1/schema/"+w.class, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("weaviate: drop: %s", string(b))
	}
	return nil
}

// Optimize is a no-op for Weaviate.
func (w *WeaviateDB) Optimize(ctx context.Context) error { return nil }

// Insert embeds and stores documents via the batch endpoint.
func (w *WeaviateDB) Insert(ctx context.Context, docs []*document.Document, _ map[string]interface{}) error {
	if err := w.EmbedDocuments(docs); err != nil {
		return err
	}
	objects := make([]map[string]interface{}, 0, len(docs))
	for _, d := range docs {
		obj := map[string]interface{}{
			"class": w.class,
			"properties": map[string]interface{}{
				"content": d.Content,
				"name":    d.Name,
				"source":  d.Source,
			},
			"vector": d.Embeddings,
		}
		if d.ID != "" {
			obj["id"] = toUUID(d.ID)
		}
		objects = append(objects, obj)
	}
	resp, err := w.do(ctx, "POST", "/v1/batch/objects", map[string]interface{}{"objects": objects})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("weaviate: insert: %s", string(b))
	}
	return nil
}

// Upsert is equivalent to Insert (batch import overwrites by id).
func (w *WeaviateDB) Upsert(ctx context.Context, docs []*document.Document, filters map[string]interface{}) error {
	return w.Insert(ctx, docs, filters)
}

// Search performs a vector search.
func (w *WeaviateDB) Search(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return w.VectorSearch(ctx, query, limit, filters)
}

// VectorSearch runs a nearVector GraphQL query.
func (w *WeaviateDB) VectorSearch(ctx context.Context, query string, limit int, _ map[string]interface{}) ([]*vectordb.SearchResult, error) {
	vec, err := w.EmbedQuery(query)
	if err != nil {
		return nil, err
	}
	vecJSON, _ := json.Marshal(vec)
	gql := fmt.Sprintf(`{ Get { %s(nearVector: {vector: %s}, limit: %d) { content name source _additional { id distance } } } }`,
		w.class, string(vecJSON), limit)

	resp, err := w.do(ctx, "POST", "/v1/graphql", map[string]string{"query": gql})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("weaviate: search: %s", string(b))
	}

	var out struct {
		Data struct {
			Get map[string][]struct {
				Content    string `json:"content"`
				Name       string `json:"name"`
				Source     string `json:"source"`
				Additional struct {
					ID       string  `json:"id"`
					Distance float64 `json:"distance"`
				} `json:"_additional"`
			} `json:"Get"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	var results []*vectordb.SearchResult
	for _, item := range out.Data.Get[w.class] {
		results = append(results, &vectordb.SearchResult{
			Document: &document.Document{
				ID:       item.Additional.ID,
				Content:  item.Content,
				Name:     item.Name,
				Source:   item.Source,
				Metadata: map[string]interface{}{"source": item.Source},
			},
			Score:    1 - item.Additional.Distance, // cosine distance -> similarity
			Distance: item.Additional.Distance,
		})
	}
	return results, nil
}

// KeywordSearch uses Weaviate's BM25 search.
func (w *WeaviateDB) KeywordSearch(ctx context.Context, query string, limit int, _ map[string]interface{}) ([]*vectordb.SearchResult, error) {
	q, _ := json.Marshal(query)
	gql := fmt.Sprintf(`{ Get { %s(bm25: {query: %s}, limit: %d) { content name source _additional { id score } } } }`,
		w.class, string(q), limit)

	resp, err := w.do(ctx, "POST", "/v1/graphql", map[string]string{"query": gql})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out struct {
		Data struct {
			Get map[string][]struct {
				Content    string `json:"content"`
				Name       string `json:"name"`
				Source     string `json:"source"`
				Additional struct {
					ID    string  `json:"id"`
					Score float64 `json:"score,string"`
				} `json:"_additional"`
			} `json:"Get"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	var results []*vectordb.SearchResult
	for _, item := range out.Data.Get[w.class] {
		results = append(results, &vectordb.SearchResult{
			Document: &document.Document{ID: item.Additional.ID, Content: item.Content, Name: item.Name, Source: item.Source},
			Score:    item.Additional.Score,
		})
	}
	return results, nil
}

// HybridSearch is not implemented (Weaviate hybrid requires a server-side vectorizer or fusion config).
func (w *WeaviateDB) HybridSearch(ctx context.Context, query string, limit int, _ map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return nil, fmt.Errorf("weaviate: hybrid search not implemented")
}

// GetCount returns the number of objects in the class via Aggregate.
func (w *WeaviateDB) GetCount(ctx context.Context) (int64, error) {
	gql := fmt.Sprintf(`{ Aggregate { %s { meta { count } } } }`, w.class)
	resp, err := w.do(ctx, "POST", "/v1/graphql", map[string]string{"query": gql})
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var out struct {
		Data struct {
			Aggregate map[string][]struct {
				Meta struct {
					Count int64 `json:"count"`
				} `json:"meta"`
			} `json:"Aggregate"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, err
	}
	if rows := out.Data.Aggregate[w.class]; len(rows) > 0 {
		return rows[0].Meta.Count, nil
	}
	return 0, nil
}

// DocExists checks existence by the document's ID.
func (w *WeaviateDB) DocExists(ctx context.Context, doc *document.Document) (bool, error) {
	return w.IDExists(ctx, doc.ID)
}

// NameExists is not supported without a filtered query.
func (w *WeaviateDB) NameExists(ctx context.Context, name string) (bool, error) {
	return false, fmt.Errorf("weaviate: NameExists not supported")
}

// IDExists checks whether an object with the given ID exists.
func (w *WeaviateDB) IDExists(ctx context.Context, id string) (bool, error) {
	resp, err := w.do(ctx, "GET", "/v1/objects/"+w.class+"/"+toUUID(id), nil)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}

var _ vectordb.VectorDB = (*WeaviateDB)(nil)
