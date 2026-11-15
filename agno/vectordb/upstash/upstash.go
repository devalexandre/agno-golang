// Package upstash implements the vectordb.VectorDB interface for Upstash
// Vector using its REST API directly (no client dependency), mirroring Agno
// Python's agno.vectordb.upstashdb.
//
// Two embedding modes are supported:
//   - client-side: an embedder is configured and vectors are sent to /upsert
//     and /query;
//   - Upstash-hosted: no embedder (or UseUpstashEmbeddings), raw text is sent
//     to /upsert-data and /query-data and the index embeds it server-side.
//
// All operations are scoped to Config.Namespace ("" is the default namespace).
// Indexes are created and deleted in the Upstash console, so Create is a
// no-op and Drop resets (empties) the namespace.
package upstash

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
	"os"
	"sort"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

// Config configures an Upstash Vector DB.
type Config struct {
	// URL is the index REST URL; falls back to UPSTASH_VECTOR_REST_URL.
	URL string
	// Token is the REST token; falls back to UPSTASH_VECTOR_REST_TOKEN.
	Token     string
	Namespace string

	// Embedder computes vectors client-side. When nil, Upstash-hosted
	// embeddings are used (UseUpstashEmbeddings is forced to true).
	Embedder             embedder.Embedder
	UseUpstashEmbeddings bool

	HTTPClient *http.Client
	Timeout    time.Duration // default 30s for the built-in client
}

// Upstash implements vectordb.VectorDB over the Upstash Vector REST API.
type Upstash struct {
	*vectordb.BaseVectorDB
	cfg    Config
	client *http.Client
}

var _ vectordb.VectorDB = (*Upstash)(nil)

// New creates an Upstash Vector DB. It does not contact the service.
func New(cfg Config) (*Upstash, error) {
	if cfg.URL == "" {
		cfg.URL = os.Getenv("UPSTASH_VECTOR_REST_URL")
	}
	if cfg.Token == "" {
		cfg.Token = os.Getenv("UPSTASH_VECTOR_REST_TOKEN")
	}
	if cfg.URL == "" || cfg.Token == "" {
		return nil, errors.New("upstash: URL and Token are required (or UPSTASH_VECTOR_REST_URL / UPSTASH_VECTOR_REST_TOKEN)")
	}
	cfg.URL = strings.TrimRight(cfg.URL, "/")
	if cfg.Embedder == nil {
		cfg.UseUpstashEmbeddings = true
	}
	client := cfg.HTTPClient
	if client == nil {
		t := cfg.Timeout
		if t <= 0 {
			t = 30 * time.Second
		}
		client = &http.Client{Timeout: t}
	}
	return &Upstash{
		BaseVectorDB: vectordb.NewBaseVectorDB(cfg.Embedder, vectordb.SearchTypeVector, vectordb.DistanceCosine),
		cfg:          cfg,
		client:       client,
	}, nil
}

// ---------------------------------------------------------------- transport

type apiError struct {
	Status int
	Body   string
}

func (e *apiError) Error() string { return fmt.Sprintf("upstash: HTTP %d: %s", e.Status, e.Body) }

func (u *Upstash) path(op string) string {
	if u.cfg.Namespace == "" {
		return "/" + op
	}
	return "/" + op + "/" + url.PathEscape(u.cfg.Namespace)
}

// call sends body as JSON and decodes the "result" field of the response into out.
func (u *Upstash) call(ctx context.Context, method, path string, body, out interface{}) error {
	var rdr io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.cfg.URL+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+u.cfg.Token)
	if rdr != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := u.client.Do(req)
	if err != nil {
		return fmt.Errorf("upstash: request failed: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &apiError{Status: resp.StatusCode, Body: string(data)}
	}
	var env struct {
		Result json.RawMessage `json:"result"`
		Error  string          `json:"error"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("upstash: decode response: %w", err)
	}
	if env.Error != "" {
		return fmt.Errorf("upstash: %s", env.Error)
	}
	if out != nil && len(env.Result) > 0 {
		return json.Unmarshal(env.Result, out)
	}
	return nil
}

// ---------------------------------------------------------------- lifecycle

// Info is the subset of /info used by this package.
type Info struct {
	VectorCount        int64  `json:"vectorCount"`
	PendingVectorCount int64  `json:"pendingVectorCount"`
	Dimension          int    `json:"dimension"`
	SimilarityFunction string `json:"similarityFunction"`
	Namespaces         map[string]struct {
		VectorCount        int64 `json:"vectorCount"`
		PendingVectorCount int64 `json:"pendingVectorCount"`
	} `json:"namespaces"`
}

// Info returns the index info.
func (u *Upstash) Info(ctx context.Context) (*Info, error) {
	var info Info
	if err := u.call(ctx, http.MethodGet, "/info", nil, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// Create is a no-op: Upstash indexes are created in the console or developer API.
func (u *Upstash) Create(ctx context.Context) error { return nil }

// Exists reports whether the index is reachable (/info succeeds).
func (u *Upstash) Exists(ctx context.Context) (bool, error) {
	if _, err := u.Info(ctx); err != nil {
		var ae *apiError
		if errors.As(err, &ae) && (ae.Status == http.StatusNotFound || ae.Status == http.StatusUnauthorized) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Drop resets (deletes every vector of) the configured namespace.
func (u *Upstash) Drop(ctx context.Context) error {
	return u.call(ctx, http.MethodDelete, u.path("reset"), nil, nil)
}

// Optimize is a no-op for Upstash.
func (u *Upstash) Optimize(ctx context.Context) error { return nil }

// GetCount returns the vector count of the configured namespace.
func (u *Upstash) GetCount(ctx context.Context) (int64, error) {
	info, err := u.Info(ctx)
	if err != nil {
		return 0, err
	}
	if ns, ok := info.Namespaces[u.cfg.Namespace]; ok {
		return ns.VectorCount, nil
	}
	if u.cfg.Namespace == "" {
		return info.VectorCount, nil
	}
	return 0, nil
}

// ---------------------------------------------------------------- writes

// RecordID returns the Upstash id used for doc: its ID or the MD5 of its content.
func RecordID(doc *document.Document) string {
	if doc.ID != "" {
		return doc.ID
	}
	sum := md5.Sum([]byte(doc.Content))
	return hex.EncodeToString(sum[:])
}

type vectorRecord struct {
	ID       string                 `json:"id"`
	Vector   []float64              `json:"vector,omitempty"`
	Data     string                 `json:"data,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

func buildMetadata(doc *document.Document, filters map[string]interface{}) map[string]interface{} {
	meta := make(map[string]interface{}, len(doc.Metadata)+len(filters)+3)
	for k, v := range doc.Metadata {
		meta[k] = v
	}
	for k, v := range filters {
		meta[k] = v
	}
	meta["text"] = doc.Content
	if doc.Name != "" {
		meta["name"] = doc.Name
	}
	if doc.Source != "" {
		meta["source"] = doc.Source
	}
	return meta
}

// Upsert writes documents (Upstash upserts are idempotent by id). Filters are
// merged into each record's metadata.
func (u *Upstash) Upsert(ctx context.Context, docs []*document.Document, filters map[string]interface{}) error {
	if len(docs) == 0 {
		return nil
	}
	if !u.cfg.UseUpstashEmbeddings {
		if err := u.EmbedDocuments(docs); err != nil {
			return err
		}
	}
	records := make([]vectorRecord, 0, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		id := RecordID(doc)
		if doc.ID == "" {
			doc.ID = id
		}
		rec := vectorRecord{ID: id, Data: doc.Content, Metadata: buildMetadata(doc, filters)}
		if !u.cfg.UseUpstashEmbeddings {
			if len(doc.Embeddings) == 0 {
				return fmt.Errorf("upstash: document %q has no embedding", doc.ID)
			}
			rec.Vector = doc.Embeddings
		}
		records = append(records, rec)
	}
	op := "upsert"
	if u.cfg.UseUpstashEmbeddings {
		op = "upsert-data"
	}
	return u.call(ctx, http.MethodPost, u.path(op), records, nil)
}

// Insert is equivalent to Upsert.
func (u *Upstash) Insert(ctx context.Context, docs []*document.Document, filters map[string]interface{}) error {
	return u.Upsert(ctx, docs, filters)
}

// DeleteByIDs removes the given ids from the namespace.
func (u *Upstash) DeleteByIDs(ctx context.Context, ids []string) error {
	return u.call(ctx, http.MethodDelete, u.path("delete"), map[string]interface{}{"ids": ids}, nil)
}

// DeleteByFilter removes every vector matching the metadata filters.
func (u *Upstash) DeleteByFilter(ctx context.Context, filters map[string]interface{}) error {
	f := BuildFilter(filters)
	if f == "" {
		return errors.New("upstash: empty delete filter")
	}
	return u.call(ctx, http.MethodDelete, u.path("delete"), map[string]interface{}{"filter": f}, nil)
}

// ---------------------------------------------------------------- existence

// IDExists fetches the id and reports whether it is present.
func (u *Upstash) IDExists(ctx context.Context, id string) (bool, error) {
	if id == "" {
		return false, nil
	}
	var out []json.RawMessage
	if err := u.call(ctx, http.MethodPost, u.path("fetch"), map[string]interface{}{"ids": []string{id}}, &out); err != nil {
		return false, err
	}
	for _, r := range out {
		if len(r) > 0 && string(r) != "null" {
			return true, nil
		}
	}
	return false, nil
}

// DocExists checks the document's record id.
func (u *Upstash) DocExists(ctx context.Context, doc *document.Document) (bool, error) {
	if doc == nil {
		return false, nil
	}
	return u.IDExists(ctx, RecordID(doc))
}

// NameExists runs a top-1 filtered query on metadata name.
func (u *Upstash) NameExists(ctx context.Context, name string) (bool, error) {
	res, err := u.query(ctx, name, 1, BuildFilter(map[string]interface{}{"name": name}), false)
	if err != nil {
		return false, err
	}
	return len(res) > 0, nil
}

// ---------------------------------------------------------------- search

type queryHit struct {
	ID       string                 `json:"id"`
	Score    float64                `json:"score"`
	Vector   []float64              `json:"vector"`
	Data     string                 `json:"data"`
	Metadata map[string]interface{} `json:"metadata"`
}

func (u *Upstash) query(ctx context.Context, query string, limit int, filter string, full bool) ([]*vectordb.SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}
	body := map[string]interface{}{
		"topK":            limit,
		"includeMetadata": true,
		"includeData":     true,
		"includeVectors":  full,
	}
	if filter != "" {
		body["filter"] = filter
	}
	op := "query"
	if u.cfg.UseUpstashEmbeddings {
		op = "query-data"
		body["data"] = query
	} else {
		vec, err := u.EmbedQuery(query)
		if err != nil {
			return nil, err
		}
		if len(vec) == 0 {
			return nil, errors.New("upstash: empty query embedding")
		}
		body["vector"] = vec
	}
	var hits []queryHit
	if err := u.call(ctx, http.MethodPost, u.path(op), body, &hits); err != nil {
		return nil, err
	}
	out := make([]*vectordb.SearchResult, 0, len(hits))
	for _, h := range hits {
		meta := h.Metadata
		if meta == nil {
			meta = map[string]interface{}{}
		}
		content := h.Data
		if content == "" {
			content, _ = meta["text"].(string)
		}
		name, _ := meta["name"].(string)
		source, _ := meta["source"].(string)
		out = append(out, &vectordb.SearchResult{
			Document: &document.Document{ID: h.ID, Content: content, Name: name, Source: source, Metadata: meta, Embeddings: h.Vector},
			Score:    h.Score,
			Distance: 1 - h.Score,
		})
	}
	return out, nil
}

// Search runs a vector search (dense, or server-side fused on hybrid indexes
// when Upstash-hosted embeddings are used).
func (u *Upstash) Search(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return u.VectorSearch(ctx, query, limit, filters)
}

// VectorSearch queries /query (client embeddings) or /query-data (hosted).
func (u *Upstash) VectorSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return u.query(ctx, query, limit, BuildFilter(filters), true)
}

// KeywordSearch is not supported by Upstash Vector.
func (u *Upstash) KeywordSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return nil, errors.New("upstash: keyword search is not supported")
}

// HybridSearch is available with Upstash-hosted embeddings on a hybrid index,
// where /query-data fuses dense and sparse scores server-side.
func (u *Upstash) HybridSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	if !u.cfg.UseUpstashEmbeddings {
		return nil, errors.New("upstash: hybrid search requires Upstash-hosted embeddings on a hybrid index")
	}
	return u.VectorSearch(ctx, query, limit, filters)
}

// ---------------------------------------------------------------- filters

// quote renders s as an Upstash string literal (no escape processing, so the
// quote char not contained in s is used); ok is false if s has both.
func quote(s string) (string, bool) {
	if !strings.Contains(s, `"`) {
		return `"` + s + `"`, true
	}
	if !strings.Contains(s, "'") {
		return "'" + s + "'", true
	}
	return "", false
}

func literal(v interface{}) (string, bool) {
	switch x := v.(type) {
	case string:
		return quote(x)
	case bool:
		return fmt.Sprintf("%t", x), true
	case int, int32, int64, float32, float64, uint, uint32, uint64:
		return fmt.Sprintf("%v", x), true
	}
	return "", false
}

func toList(v interface{}) ([]interface{}, bool) {
	switch l := v.(type) {
	case []interface{}:
		return l, true
	case []string:
		out := make([]interface{}, len(l))
		for i, s := range l {
			out[i] = s
		}
		return out, true
	case []int:
		out := make([]interface{}, len(l))
		for i, s := range l {
			out[i] = s
		}
		return out, true
	case []float64:
		out := make([]interface{}, len(l))
		for i, s := range l {
			out[i] = s
		}
		return out, true
	}
	return nil, false
}

func alwaysFalse(key string) string {
	return fmt.Sprintf("(HAS FIELD %s AND HAS NOT FIELD %s)", key, key)
}

func inClause(key string, list []interface{}) string {
	parts := make([]string, 0, len(list))
	for _, item := range list {
		lit, ok := literal(item)
		if !ok {
			continue
		}
		parts = append(parts, lit)
	}
	if len(parts) == 0 {
		return alwaysFalse(key)
	}
	return fmt.Sprintf("%s IN (%s)", key, strings.Join(parts, ", "))
}

var rangeOps = []struct{ key, op string }{
	{"gt", ">"}, {"$gt", ">"}, {"gte", ">="}, {"$gte", ">="},
	{"lt", "<"}, {"$lt", "<"}, {"lte", "<="}, {"$lte", "<="},
	{"ne", "!="}, {"$ne", "!="},
}

// BuildFilter converts agno metadata filters into an Upstash filter string
// (clauses joined with AND, keys sorted for determinism):
//   - scalar                      -> key = literal
//   - list / {"$in": [...]}       -> key IN (...)
//   - {"gt","gte","lt","lte","ne"} -> comparison operators
//
// A string value containing both quote characters has no literal form and
// becomes an always-false predicate (fail closed).
func BuildFilter(filters map[string]interface{}) string {
	if len(filters) == 0 {
		return ""
	}
	keys := make([]string, 0, len(filters))
	for k := range filters {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var clauses []string
	for _, key := range keys {
		value := filters[key]
		if m, ok := value.(map[string]interface{}); ok {
			if in, ok := m["$in"]; ok {
				if l, ok := toList(in); ok {
					clauses = append(clauses, inClause(key, l))
				}
			} else if in, ok := m["in"]; ok {
				if l, ok := toList(in); ok {
					clauses = append(clauses, inClause(key, l))
				}
			}
			for _, r := range rangeOps {
				if v, ok := m[r.key]; ok {
					if lit, ok := literal(v); ok {
						clauses = append(clauses, fmt.Sprintf("%s %s %s", key, r.op, lit))
					}
				}
			}
			continue
		}
		if l, ok := toList(value); ok {
			clauses = append(clauses, inClause(key, l))
			continue
		}
		if lit, ok := literal(value); ok {
			clauses = append(clauses, fmt.Sprintf("%s = %s", key, lit))
		} else {
			clauses = append(clauses, alwaysFalse(key))
		}
	}
	return strings.Join(clauses, " AND ")
}
