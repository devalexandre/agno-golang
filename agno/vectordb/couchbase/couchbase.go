// Package couchbase implements the vectordb.VectorDB interface for Couchbase
// Server 7.6+ / Capella using only its REST services (no SDK dependency),
// mirroring Agno Python's agno.vectordb.couchbase.
//
//   - The Query service (N1QL, POST /query/service) creates the scope and
//     collection, upserts/deletes documents and answers count/existence
//     queries.
//   - The Search service (FTS) holds a scoped vector index on the "embedding"
//     field; vector search sends a "knn" request, keyword search a match query
//     on "content", and hybrid search both in one request (scores are summed).
//
// Documents are stored with the same shape as Python:
// {name, content, meta_data, embedding, content_id, content_hash, source}.
package couchbase

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

const (
	DefaultQueryURL  = "http://localhost:8093"
	DefaultSearchURL = "http://localhost:8094"
)

// Config configures a Couchbase vector DB.
type Config struct {
	QueryURL   string // Query service base URL (default http://localhost:8093)
	SearchURL  string // Search service base URL (default http://localhost:8094)
	Username   string
	Password   string
	Bucket     string
	Scope      string // default "_default"
	Collection string // default "_default"
	// SearchIndex is the FTS index name (default "<collection>_vector_index").
	SearchIndex string
	// SearchIndexDefinition overrides the generated FTS index definition.
	SearchIndexDefinition map[string]interface{}
	// Overwrite drops and recreates the collection and index on Create.
	Overwrite bool

	Embedder   embedder.Embedder
	Dimensions int
	// Distance maps to the FTS vector similarity: l2/euclidean -> l2_norm,
	// anything else -> dot_product.
	Distance   vectordb.Distance
	SearchType vectordb.SearchType

	HTTPClient *http.Client
}

// Couchbase implements vectordb.VectorDB over the Couchbase REST services.
type Couchbase struct {
	*vectordb.BaseVectorDB
	cfg    Config
	client *http.Client
}

var _ vectordb.VectorDB = (*Couchbase)(nil)

// New creates a Couchbase vector DB. It does not contact the cluster.
func New(cfg Config) (*Couchbase, error) {
	if cfg.Bucket == "" {
		return nil, errors.New("couchbase: bucket is required")
	}
	if cfg.Embedder == nil {
		return nil, errors.New("couchbase: embedder is required")
	}
	if cfg.QueryURL == "" {
		cfg.QueryURL = DefaultQueryURL
	}
	if cfg.SearchURL == "" {
		cfg.SearchURL = DefaultSearchURL
	}
	cfg.QueryURL = strings.TrimRight(cfg.QueryURL, "/")
	cfg.SearchURL = strings.TrimRight(cfg.SearchURL, "/")
	if cfg.Scope == "" {
		cfg.Scope = "_default"
	}
	if cfg.Collection == "" {
		cfg.Collection = "_default"
	}
	if cfg.SearchIndex == "" {
		cfg.SearchIndex = cfg.Collection + "_vector_index"
	}
	if cfg.SearchType == "" {
		cfg.SearchType = vectordb.SearchTypeVector
	}
	if cfg.Distance == "" {
		cfg.Distance = vectordb.DistanceCosine
	}
	base := vectordb.NewBaseVectorDB(cfg.Embedder, cfg.SearchType, cfg.Distance)
	if cfg.Dimensions > 0 {
		base.Dimensions = cfg.Dimensions
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Couchbase{BaseVectorDB: base, cfg: cfg, client: client}, nil
}

func quoteIdent(s string) string { return "`" + strings.ReplaceAll(s, "`", "``") + "`" }

func (c *Couchbase) keyspace() string {
	return quoteIdent(c.cfg.Bucket) + "." + quoteIdent(c.cfg.Scope) + "." + quoteIdent(c.cfg.Collection)
}

func (c *Couchbase) indexURL() string {
	return fmt.Sprintf("%s/api/bucket/%s/scope/%s/index/%s", c.cfg.SearchURL,
		url.PathEscape(c.cfg.Bucket), url.PathEscape(c.cfg.Scope), url.PathEscape(c.cfg.SearchIndex))
}

// do sends a JSON request and returns the status code and body.
func (c *Couchbase) do(ctx context.Context, method, u string, body interface{}) (int, []byte, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, r)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.cfg.Username != "" {
		req.SetBasicAuth(c.cfg.Username, c.cfg.Password)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return resp.StatusCode, data, err
}

type queryResponse struct {
	Status  string            `json:"status"`
	Results []json.RawMessage `json:"results"`
	Errors  []struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	} `json:"errors"`
}

// query runs a N1QL statement with named parameters (keys without the "$").
func (c *Couchbase) query(ctx context.Context, stmt string, params map[string]interface{}) ([]json.RawMessage, error) {
	body := map[string]interface{}{"statement": stmt, "scan_consistency": "request_plus"}
	for k, v := range params {
		body["$"+k] = v
	}
	status, data, err := c.do(ctx, http.MethodPost, c.cfg.QueryURL+"/query/service", body)
	if err != nil {
		return nil, fmt.Errorf("couchbase: query: %w", err)
	}
	var out queryResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("couchbase: query: status %d: %s", status, string(data))
	}
	if status != http.StatusOK || out.Status != "success" {
		msgs := make([]string, 0, len(out.Errors))
		for _, e := range out.Errors {
			msgs = append(msgs, fmt.Sprintf("%d: %s", e.Code, e.Msg))
		}
		return nil, fmt.Errorf("couchbase: query failed (status %d): %s", status, strings.Join(msgs, "; "))
	}
	return out.Results, nil
}

// Create ensures the scope, collection, a primary index and the FTS vector
// index exist (dropping them first when Overwrite is set).
func (c *Couchbase) Create(ctx context.Context) error {
	if _, err := c.query(ctx, fmt.Sprintf("CREATE SCOPE %s.%s IF NOT EXISTS", quoteIdent(c.cfg.Bucket), quoteIdent(c.cfg.Scope)), nil); err != nil {
		return err
	}
	if c.cfg.Overwrite {
		if err := c.Drop(ctx); err != nil {
			return err
		}
	}
	if _, err := c.query(ctx, fmt.Sprintf("CREATE COLLECTION %s IF NOT EXISTS", c.keyspace()), nil); err != nil {
		return err
	}
	if _, err := c.query(ctx, fmt.Sprintf("CREATE PRIMARY INDEX IF NOT EXISTS ON %s", c.keyspace()), nil); err != nil {
		return err
	}
	return c.createSearchIndex(ctx)
}

func (c *Couchbase) similarity() string {
	switch c.Distance {
	case vectordb.DistanceL2, vectordb.DistanceEuclidean:
		return "l2_norm"
	default:
		return "dot_product"
	}
}

// IndexDefinition returns the FTS index definition used by Create.
func (c *Couchbase) IndexDefinition() map[string]interface{} {
	if c.cfg.SearchIndexDefinition != nil {
		return c.cfg.SearchIndexDefinition
	}
	typeName := c.cfg.Scope + "." + c.cfg.Collection
	return map[string]interface{}{
		"type":       "fulltext-index",
		"name":       c.cfg.SearchIndex,
		"sourceType": "gocbcore",
		"sourceName": c.cfg.Bucket,
		"planParams": map[string]interface{}{"indexPartitions": 1},
		"params": map[string]interface{}{
			"doc_config": map[string]interface{}{"mode": "scope.collection.type_field", "type_field": "type"},
			"mapping": map[string]interface{}{
				"default_mapping": map[string]interface{}{"enabled": false},
				"types": map[string]interface{}{
					typeName: map[string]interface{}{
						"enabled": true,
						"dynamic": true,
						"properties": map[string]interface{}{
							"embedding": map[string]interface{}{"enabled": true, "fields": []interface{}{
								map[string]interface{}{"name": "embedding", "type": "vector", "dims": c.Dimensions, "similarity": c.similarity(), "index": true},
							}},
							"content": map[string]interface{}{"enabled": true, "fields": []interface{}{
								map[string]interface{}{"name": "content", "type": "text", "index": true, "store": true},
							}},
						},
					},
				},
			},
			"store": map[string]interface{}{"indexType": "scorch", "segmentVersion": 16},
		},
	}
}

func (c *Couchbase) createSearchIndex(ctx context.Context) error {
	status, _, err := c.do(ctx, http.MethodGet, c.indexURL(), nil)
	if err != nil {
		return fmt.Errorf("couchbase: get search index: %w", err)
	}
	if status == http.StatusOK && !c.cfg.Overwrite {
		return nil
	}
	if status == http.StatusOK {
		if _, _, err := c.do(ctx, http.MethodDelete, c.indexURL(), nil); err != nil {
			return fmt.Errorf("couchbase: drop search index: %w", err)
		}
	}
	if c.cfg.SearchIndexDefinition == nil && c.Dimensions <= 0 {
		return errors.New("couchbase: dimensions are required to create the vector index")
	}
	status, data, err := c.do(ctx, http.MethodPut, c.indexURL(), c.IndexDefinition())
	if err != nil {
		return fmt.Errorf("couchbase: create search index: %w", err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("couchbase: create search index: status %d: %s", status, string(data))
	}
	return nil
}

// Exists reports whether the collection exists.
func (c *Couchbase) Exists(ctx context.Context) (bool, error) {
	rows, err := c.query(ctx, "SELECT RAW name FROM system:keyspaces WHERE `bucket` = $bucket AND `scope` = $scope AND name = $collection",
		map[string]interface{}{"bucket": c.cfg.Bucket, "scope": c.cfg.Scope, "collection": c.cfg.Collection})
	if err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}

// Drop drops the collection (its FTS index goes with it).
func (c *Couchbase) Drop(ctx context.Context) error {
	_, err := c.query(ctx, fmt.Sprintf("DROP COLLECTION %s IF EXISTS", c.keyspace()), nil)
	return err
}

// Optimize is a no-op.
func (c *Couchbase) Optimize(ctx context.Context) error { return nil }

func docKey(d *document.Document) string {
	if d.ID != "" {
		return d.ID
	}
	sum := md5.Sum([]byte(strings.ReplaceAll(d.Content, "\x00", "�")))
	return hex.EncodeToString(sum[:])
}

func (c *Couchbase) prepareDoc(d *document.Document, filters map[string]interface{}) map[string]interface{} {
	meta := map[string]interface{}{}
	for k, v := range d.Metadata {
		meta[k] = v
	}
	for k, v := range filters {
		meta[k] = v
	}
	return map[string]interface{}{
		"name":       d.Name,
		"content":    strings.ReplaceAll(d.Content, "\x00", "�"),
		"meta_data":  meta,
		"embedding":  d.Embeddings,
		"content_id": d.ParentID,
		"source":     d.Source,
	}
}

// Insert embeds and upserts documents (one multi-row UPSERT statement).
func (c *Couchbase) Insert(ctx context.Context, docs []*document.Document, filters map[string]interface{}) error {
	if len(docs) == 0 {
		return nil
	}
	if err := c.EmbedDocuments(docs); err != nil {
		return err
	}
	params := map[string]interface{}{}
	values := make([]string, 0, len(docs))
	for i, d := range docs {
		if d == nil {
			continue
		}
		if d.Content == "" {
			return fmt.Errorf("couchbase: document %q has no content", d.Name)
		}
		params[fmt.Sprintf("k%d", i)] = docKey(d)
		params[fmt.Sprintf("v%d", i)] = c.prepareDoc(d, filters)
		values = append(values, fmt.Sprintf("VALUES ($k%d, $v%d)", i, i))
	}
	if len(values) == 0 {
		return nil
	}
	_, err := c.query(ctx, fmt.Sprintf("UPSERT INTO %s (KEY, VALUE) %s", c.keyspace(), strings.Join(values, ", ")), params)
	return err
}

// Upsert is equivalent to Insert (UPSERT overwrites).
func (c *Couchbase) Upsert(ctx context.Context, docs []*document.Document, filters map[string]interface{}) error {
	return c.Insert(ctx, docs, filters)
}

// Search dispatches on the configured SearchType.
func (c *Couchbase) Search(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	switch c.SearchType {
	case vectordb.SearchTypeKeyword:
		return c.KeywordSearch(ctx, query, limit, filters)
	case vectordb.SearchTypeHybrid:
		return c.HybridSearch(ctx, query, limit, filters)
	default:
		return c.VectorSearch(ctx, query, limit, filters)
	}
}

// filterQuery turns equality filters into an FTS conjunction on meta_data.*.
func filterQuery(filters map[string]interface{}) map[string]interface{} {
	if len(filters) == 0 {
		return nil
	}
	keys := make([]string, 0, len(filters))
	for k := range filters {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	conj := make([]interface{}, 0, len(keys))
	for _, k := range keys {
		field := "meta_data." + k
		switch v := filters[k].(type) {
		case int, int32, int64, float32, float64:
			conj = append(conj, map[string]interface{}{"field": field, "min": v, "max": v, "inclusive_min": true, "inclusive_max": true})
		case bool:
			conj = append(conj, map[string]interface{}{"field": field, "bool": v})
		default:
			conj = append(conj, map[string]interface{}{"field": field, "match": fmt.Sprint(v)})
		}
	}
	return map[string]interface{}{"conjuncts": conj}
}

func (c *Couchbase) knnClause(query string, limit int, filters map[string]interface{}) ([]interface{}, error) {
	vec, err := c.EmbedQuery(query)
	if err != nil {
		return nil, err
	}
	if len(vec) == 0 {
		return nil, errors.New("couchbase: empty query embedding")
	}
	knn := map[string]interface{}{"field": "embedding", "vector": vec, "k": limit}
	if f := filterQuery(filters); f != nil {
		knn["filter"] = f
	}
	return []interface{}{knn}, nil
}

func normLimit(limit int) int {
	if limit <= 0 {
		return 5
	}
	return limit
}

// VectorSearch runs an FTS kNN query.
func (c *Couchbase) VectorSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	limit = normLimit(limit)
	knn, err := c.knnClause(query, limit, filters)
	if err != nil {
		return nil, err
	}
	return c.searchFTS(ctx, map[string]interface{}{
		"query": map[string]interface{}{"match_none": map[string]interface{}{}},
		"knn":   knn,
		"size":  limit,
	})
}

func (c *Couchbase) keywordQuery(query string, filters map[string]interface{}) map[string]interface{} {
	q := map[string]interface{}{"match": query, "field": "content"}
	if f := filterQuery(filters); f != nil {
		f["conjuncts"] = append([]interface{}{q}, f["conjuncts"].([]interface{})...)
		return f
	}
	return q
}

// KeywordSearch runs an FTS match query on the content field.
func (c *Couchbase) KeywordSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	limit = normLimit(limit)
	return c.searchFTS(ctx, map[string]interface{}{"query": c.keywordQuery(query, filters), "size": limit})
}

// HybridSearch sends a match query and a kNN clause in one request; Couchbase
// sums the two scores.
func (c *Couchbase) HybridSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	limit = normLimit(limit)
	knn, err := c.knnClause(query, limit, filters)
	if err != nil {
		return nil, err
	}
	return c.searchFTS(ctx, map[string]interface{}{
		"query": c.keywordQuery(query, filters),
		"knn":   knn,
		"size":  limit,
	})
}

type ftsResponse struct {
	Hits []struct {
		ID    string  `json:"id"`
		Score float64 `json:"score"`
	} `json:"hits"`
}

// searchFTS runs an FTS request and loads the hit documents via N1QL USE KEYS
// (Python fetches them from KV with get_multi).
func (c *Couchbase) searchFTS(ctx context.Context, req map[string]interface{}) ([]*vectordb.SearchResult, error) {
	status, data, err := c.do(ctx, http.MethodPost, c.indexURL()+"/query", req)
	if err != nil {
		return nil, fmt.Errorf("couchbase: search: %w", err)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("couchbase: search: status %d: %s", status, string(data))
	}
	var out ftsResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("couchbase: decode search: %w", err)
	}
	if len(out.Hits) == 0 {
		return nil, nil
	}
	ids := make([]string, len(out.Hits))
	for i, h := range out.Hits {
		ids[i] = h.ID
	}
	docs, err := c.fetch(ctx, ids)
	if err != nil {
		return nil, err
	}
	results := make([]*vectordb.SearchResult, 0, len(out.Hits))
	for _, h := range out.Hits {
		d, ok := docs[h.ID]
		if !ok {
			continue
		}
		results = append(results, &vectordb.SearchResult{Document: d, Score: h.Score})
	}
	return results, nil
}

type storedDoc struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Content   string                 `json:"content"`
	MetaData  map[string]interface{} `json:"meta_data"`
	Embedding []float64              `json:"embedding"`
	ContentID string                 `json:"content_id"`
	Source    string                 `json:"source"`
}

func (c *Couchbase) fetch(ctx context.Context, ids []string) (map[string]*document.Document, error) {
	rows, err := c.query(ctx, fmt.Sprintf("SELECT META(d).id AS id, d.* FROM %s AS d USE KEYS $ids", c.keyspace()),
		map[string]interface{}{"ids": ids})
	if err != nil {
		return nil, err
	}
	out := make(map[string]*document.Document, len(rows))
	for _, r := range rows {
		var s storedDoc
		if err := json.Unmarshal(r, &s); err != nil {
			return nil, fmt.Errorf("couchbase: decode document: %w", err)
		}
		out[s.ID] = &document.Document{
			ID: s.ID, Name: s.Name, Content: s.Content, Metadata: s.MetaData,
			Embeddings: s.Embedding, Source: s.Source, ParentID: s.ContentID,
		}
	}
	return out, nil
}

func (c *Couchbase) countWhere(ctx context.Context, where string, params map[string]interface{}) (int64, error) {
	stmt := fmt.Sprintf("SELECT RAW COUNT(*) FROM %s", c.keyspace())
	if where != "" {
		stmt += " WHERE " + where
	}
	rows, err := c.query(ctx, stmt, params)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	var n int64
	if err := json.Unmarshal(rows[0], &n); err != nil {
		return 0, fmt.Errorf("couchbase: decode count: %w", err)
	}
	return n, nil
}

// GetCount returns the number of documents in the collection.
func (c *Couchbase) GetCount(ctx context.Context) (int64, error) {
	return c.countWhere(ctx, "", nil)
}

// DocExists checks existence by the document key.
func (c *Couchbase) DocExists(ctx context.Context, doc *document.Document) (bool, error) {
	if doc == nil {
		return false, nil
	}
	return c.IDExists(ctx, docKey(doc))
}

// NameExists checks whether a document with the given name exists.
func (c *Couchbase) NameExists(ctx context.Context, name string) (bool, error) {
	n, err := c.countWhere(ctx, "name = $name", map[string]interface{}{"name": name})
	return n > 0, err
}

// IDExists checks whether a document key exists.
func (c *Couchbase) IDExists(ctx context.Context, id string) (bool, error) {
	n, err := c.countWhere(ctx, "META().id = $id", map[string]interface{}{"id": id})
	return n > 0, err
}

// DeleteByID deletes one document.
func (c *Couchbase) DeleteByID(ctx context.Context, id string) error {
	_, err := c.query(ctx, fmt.Sprintf("DELETE FROM %s d USE KEYS $id", c.keyspace()), map[string]interface{}{"id": id})
	return err
}

// DeleteByName deletes every document with the given name.
func (c *Couchbase) DeleteByName(ctx context.Context, name string) error {
	_, err := c.query(ctx, fmt.Sprintf("DELETE FROM %s WHERE name = $name", c.keyspace()), map[string]interface{}{"name": name})
	return err
}
