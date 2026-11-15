// Package surrealdb implements the vectordb.VectorDB interface for SurrealDB
// using its HTTP /sql endpoint (no external client dependency).
package surrealdb

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

// SurrealDB implements vectordb.VectorDB backed by SurrealDB.
type SurrealDB struct {
	*vectordb.BaseVectorDB
	client    *http.Client
	baseURL   string
	namespace string
	database  string
	user      string
	pass      string
	table     string
	dimension int
}

// Options configures a SurrealDB.
type Options struct {
	BaseURL   string // e.g. http://localhost:8000
	Namespace string // required
	Database  string // required
	User      string // root user (optional if no auth)
	Pass      string
	Table     string // table name (collection)
	Dimension int    // vector dimension (defaults to embedder's)
	Embedder  embedder.Embedder
}

// New creates a new SurrealDB.
func New(opts Options) *SurrealDB {
	if opts.BaseURL == "" {
		opts.BaseURL = "http://localhost:8000"
	}
	opts.BaseURL = strings.TrimRight(opts.BaseURL, "/")
	if opts.Namespace == "" {
		opts.Namespace = "agno"
	}
	if opts.Database == "" {
		opts.Database = "agno"
	}
	if opts.Table == "" {
		opts.Table = "documents"
	}
	dim := opts.Dimension
	if dim <= 0 && opts.Embedder != nil {
		dim = opts.Embedder.GetDimensions()
	}
	return &SurrealDB{
		BaseVectorDB: vectordb.NewBaseVectorDB(opts.Embedder, vectordb.SearchTypeVector, vectordb.DistanceCosine),
		client:       &http.Client{},
		baseURL:      opts.BaseURL,
		namespace:    opts.Namespace,
		database:     opts.Database,
		user:         opts.User,
		pass:         opts.Pass,
		table:        opts.Table,
		dimension:    dim,
	}
}

// sql executes a SurrealQL statement in the configured namespace/database.
func (s *SurrealDB) sql(ctx context.Context, query string) ([]surrealResult, error) {
	return s.exec(ctx, query, true)
}

// exec runs a query; when withContext is false the NS/DB headers are omitted
// (used to bootstrap the namespace/database, which must exist before USE).
func (s *SurrealDB) exec(ctx context.Context, query string, withContext bool) ([]surrealResult, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/sql", bytes.NewReader([]byte(query)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "text/plain")
	if withContext {
		req.Header.Set("surreal-ns", s.namespace)
		req.Header.Set("surreal-db", s.database)
		req.Header.Set("NS", s.namespace)
		req.Header.Set("DB", s.database)
	}
	if s.user != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(s.user + ":" + s.pass))
		req.Header.Set("Authorization", "Basic "+auth)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("surrealdb: %s", string(body))
	}
	var results []surrealResult
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("surrealdb: decode: %w (body: %s)", err, string(body))
	}
	for _, r := range results {
		if r.Status != "OK" {
			msg := r.Detail
			if msg == "" {
				msg = string(r.Result)
			}
			return nil, fmt.Errorf("surrealdb: query error: %s", msg)
		}
	}
	return results, nil
}

type surrealResult struct {
	Status string          `json:"status"`
	Detail string          `json:"detail"`
	Result json.RawMessage `json:"result"`
}

// Create defines the table. Vector search uses brute-force cosine similarity
// (vector::similarity::cosine), which needs no index and is portable across
// SurrealDB versions. For large collections, define an MTREE/HNSW index manually.
func (s *SurrealDB) Create(ctx context.Context) error {
	// Bootstrap NS/DB (without USE headers, which would fail if they don't exist).
	bootstrap := fmt.Sprintf(
		"DEFINE NAMESPACE IF NOT EXISTS %s; USE NS %s; DEFINE DATABASE IF NOT EXISTS %s;",
		s.namespace, s.namespace, s.database)
	if _, err := s.exec(ctx, bootstrap, false); err != nil {
		return err
	}
	_, err := s.sql(ctx, fmt.Sprintf("DEFINE TABLE IF NOT EXISTS %s SCHEMALESS;", s.table))
	return err
}

// Exists reports whether the table exists (by querying INFO FOR DB).
func (s *SurrealDB) Exists(ctx context.Context) (bool, error) {
	results, err := s.sql(ctx, "INFO FOR DB;")
	if err != nil {
		return false, err
	}
	if len(results) == 0 {
		return false, nil
	}
	var info struct {
		Tables map[string]interface{} `json:"tables"`
	}
	if err := json.Unmarshal(results[0].Result, &info); err != nil {
		return false, nil
	}
	_, ok := info.Tables[s.table]
	return ok, nil
}

// Drop removes the table.
func (s *SurrealDB) Drop(ctx context.Context) error {
	_, err := s.sql(ctx, fmt.Sprintf("REMOVE TABLE IF EXISTS %s;", s.table))
	return err
}

// Optimize is a no-op for SurrealDB.
func (s *SurrealDB) Optimize(ctx context.Context) error { return nil }

// Insert embeds and stores documents.
func (s *SurrealDB) Insert(ctx context.Context, docs []*document.Document, _ map[string]interface{}) error {
	if err := s.EmbedDocuments(docs); err != nil {
		return err
	}
	var b strings.Builder
	for _, d := range docs {
		rec := map[string]interface{}{
			"content":   d.Content,
			"name":      d.Name,
			"source":    d.Source,
			"embedding": d.Embeddings,
		}
		data, _ := json.Marshal(rec)
		id := d.ID
		if id == "" {
			fmt.Fprintf(&b, "CREATE %s CONTENT %s; ", s.table, string(data))
		} else {
			fmt.Fprintf(&b, "CREATE %s:%s CONTENT %s; ", s.table, surrealID(id), string(data))
		}
	}
	_, err := s.sql(ctx, b.String())
	return err
}

// Upsert stores documents, overwriting by id.
func (s *SurrealDB) Upsert(ctx context.Context, docs []*document.Document, _ map[string]interface{}) error {
	if err := s.EmbedDocuments(docs); err != nil {
		return err
	}
	var b strings.Builder
	for _, d := range docs {
		rec := map[string]interface{}{
			"content":   d.Content,
			"name":      d.Name,
			"source":    d.Source,
			"embedding": d.Embeddings,
		}
		data, _ := json.Marshal(rec)
		fmt.Fprintf(&b, "UPSERT %s:%s CONTENT %s; ", s.table, surrealID(d.ID), string(data))
	}
	_, err := s.sql(ctx, b.String())
	return err
}

// Search performs a vector search.
func (s *SurrealDB) Search(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return s.VectorSearch(ctx, query, limit, filters)
}

// VectorSearch runs a KNN search using the MTREE index.
func (s *SurrealDB) VectorSearch(ctx context.Context, query string, limit int, _ map[string]interface{}) ([]*vectordb.SearchResult, error) {
	vec, err := s.EmbedQuery(query)
	if err != nil {
		return nil, err
	}
	vecJSON, _ := json.Marshal(vec)
	q := fmt.Sprintf(
		"SELECT id, content, name, source, vector::similarity::cosine(embedding, %s) AS score "+
			"FROM %s ORDER BY score DESC LIMIT %d;",
		string(vecJSON), s.table, limit)

	results, err := s.sql(ctx, q)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	var rows []struct {
		ID      string  `json:"id"`
		Content string  `json:"content"`
		Name    string  `json:"name"`
		Source  string  `json:"source"`
		Score   float64 `json:"score"`
	}
	if err := json.Unmarshal(results[len(results)-1].Result, &rows); err != nil {
		return nil, err
	}
	out := make([]*vectordb.SearchResult, 0, len(rows))
	for _, r := range rows {
		out = append(out, &vectordb.SearchResult{
			Document: &document.Document{ID: r.ID, Content: r.Content, Name: r.Name, Source: r.Source},
			Score:    r.Score,
			Distance: 1 - r.Score,
		})
	}
	return out, nil
}

// KeywordSearch is not supported.
func (s *SurrealDB) KeywordSearch(ctx context.Context, query string, limit int, _ map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return nil, fmt.Errorf("surrealdb: keyword search not supported")
}

// HybridSearch is not supported.
func (s *SurrealDB) HybridSearch(ctx context.Context, query string, limit int, _ map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return nil, fmt.Errorf("surrealdb: hybrid search not supported")
}

// GetCount returns the number of records in the table.
func (s *SurrealDB) GetCount(ctx context.Context) (int64, error) {
	results, err := s.sql(ctx, fmt.Sprintf("SELECT count() AS c FROM %s GROUP ALL;", s.table))
	if err != nil {
		return 0, err
	}
	if len(results) == 0 {
		return 0, nil
	}
	var rows []struct {
		C int64 `json:"c"`
	}
	if err := json.Unmarshal(results[0].Result, &rows); err != nil || len(rows) == 0 {
		return 0, nil
	}
	return rows[0].C, nil
}

// DocExists checks existence by the document's ID.
func (s *SurrealDB) DocExists(ctx context.Context, doc *document.Document) (bool, error) {
	return s.IDExists(ctx, doc.ID)
}

// NameExists is not supported.
func (s *SurrealDB) NameExists(ctx context.Context, name string) (bool, error) {
	return false, fmt.Errorf("surrealdb: NameExists not supported")
}

// IDExists checks whether a record with the given id exists.
func (s *SurrealDB) IDExists(ctx context.Context, id string) (bool, error) {
	results, err := s.sql(ctx, fmt.Sprintf("SELECT id FROM %s:%s;", s.table, surrealID(id)))
	if err != nil {
		return false, err
	}
	if len(results) == 0 {
		return false, nil
	}
	var rows []map[string]interface{}
	if err := json.Unmarshal(results[0].Result, &rows); err != nil {
		return false, nil
	}
	return len(rows) > 0, nil
}

// surrealID escapes a record id so it is safe to embed in a query.
func surrealID(id string) string {
	if id == "" {
		return "⟨⟩"
	}
	return "⟨" + strings.ReplaceAll(id, "⟩", "") + "⟩"
}

var _ vectordb.VectorDB = (*SurrealDB)(nil)
