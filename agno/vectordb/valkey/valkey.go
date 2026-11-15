// Package valkey implements vectordb.VectorDB on Valkey with the valkey-search
// module (FT.* commands), mirroring Agno Python's agno.vectordb.valkey.ValkeyDb.
//
// Valkey speaks the Redis protocol, so this package reuses the go-redis client
// already used by agno/vectordb/redis. The schema differs from the Redis Stack
// adapter: metadata used for filtering (id, name, source, status, category,
// tag, mode, linked_to, content_id, content_hash) is declared as TAG fields, the
// content is a TEXT field and documents live under the "<index>:" key prefix.
package valkey

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"

	goredis "github.com/redis/go-redis/v9"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

// FilterableTagFields are declared as TAG fields in the index schema and are
// therefore usable as server-side filters. Other filter keys are ignored.
var FilterableTagFields = map[string]bool{
	"id": true, "name": true, "content_hash": true, "content_id": true, "status": true,
	"category": true, "tag": true, "source": true, "mode": true, "linked_to": true,
}

// reservedHashFields are owned by the adapter; document metadata cannot overwrite them.
var reservedHashFields = map[string]bool{
	"id": true, "name": true, "content": true, "embedding": true, "content_hash": true, "content_id": true,
}

// Config configures a Valkey vector DB.
type Config struct {
	// IndexName is the FT index name; documents are stored under "<IndexName>:".
	IndexName string
	Addr      string // host:port, default localhost:6379
	Username  string
	Password  string
	DB        int
	// Client overrides Addr/Username/Password/DB.
	Client   *goredis.Client
	Embedder embedder.Embedder
	// Dimensions defaults to the embedder's.
	Dimensions int
	// Distance: cosine (default), l2 or max_inner_product.
	Distance   vectordb.Distance
	SearchType vectordb.SearchType
	// VectorAlgorithm is HNSW (default) or FLAT.
	VectorAlgorithm string
}

// Valkey implements vectordb.VectorDB backed by valkey-search.
type Valkey struct {
	*vectordb.BaseVectorDB
	client    *goredis.Client
	index     string
	prefix    string
	algorithm string
}

var _ vectordb.VectorDB = (*Valkey)(nil)

// New creates a Valkey vector DB. It does not contact the server.
func New(cfg Config) (*Valkey, error) {
	if cfg.IndexName == "" {
		return nil, errors.New("valkey: IndexName is required")
	}
	if cfg.Distance == "" {
		cfg.Distance = vectordb.DistanceCosine
	}
	if _, err := distanceMetric(cfg.Distance); err != nil {
		return nil, err
	}
	if cfg.SearchType == "" {
		cfg.SearchType = vectordb.SearchTypeVector
	}
	alg := strings.ToUpper(cfg.VectorAlgorithm)
	if alg == "" {
		alg = "HNSW"
	}
	if alg != "HNSW" && alg != "FLAT" {
		return nil, fmt.Errorf("valkey: unknown vector algorithm %q", cfg.VectorAlgorithm)
	}
	client := cfg.Client
	if client == nil {
		addr := cfg.Addr
		if addr == "" {
			addr = "localhost:6379"
		}
		client = goredis.NewClient(&goredis.Options{
			Addr: addr, Username: cfg.Username, Password: cfg.Password, DB: cfg.DB, Protocol: 2,
		})
	}
	base := vectordb.NewBaseVectorDB(cfg.Embedder, cfg.SearchType, cfg.Distance)
	if cfg.Dimensions > 0 {
		base.Dimensions = cfg.Dimensions
	}
	return &Valkey{BaseVectorDB: base, client: client, index: cfg.IndexName, prefix: cfg.IndexName + ":", algorithm: alg}, nil
}

func distanceMetric(d vectordb.Distance) (string, error) {
	switch d {
	case vectordb.DistanceCosine:
		return "COSINE", nil
	case vectordb.DistanceL2, vectordb.DistanceEuclidean:
		return "L2", nil
	case vectordb.DistanceMaxInnerProduct, vectordb.DistanceIP, vectordb.DistanceDot:
		return "IP", nil
	}
	return "", fmt.Errorf("valkey: unsupported distance %q", d)
}

// Client returns the underlying go-redis client.
func (v *Valkey) Client() *goredis.Client { return v.client }

// Create builds the FT index if it does not exist.
func (v *Valkey) Create(ctx context.Context) error {
	if ok, err := v.Exists(ctx); err != nil {
		return err
	} else if ok {
		return nil
	}
	if v.Dimensions <= 0 {
		return errors.New("valkey: dimensions are required to create the index")
	}
	metric, _ := distanceMetric(v.Distance)
	args := []interface{}{"FT.CREATE", v.index, "ON", "HASH", "PREFIX", "1", v.prefix, "SCHEMA"}
	for _, f := range []string{"id", "name", "content_hash", "content_id"} {
		args = append(args, f, "TAG")
	}
	args = append(args, "content", "TEXT")
	for _, f := range []string{"status", "category", "tag", "source", "mode", "linked_to"} {
		args = append(args, f, "TAG")
	}
	args = append(args, "embedding", "VECTOR", v.algorithm, "6",
		"TYPE", "FLOAT32", "DIM", v.Dimensions, "DISTANCE_METRIC", metric)
	return v.client.Do(ctx, args...).Err()
}

// Exists reports whether the index exists (FT.INFO).
func (v *Valkey) Exists(ctx context.Context) (bool, error) {
	err := v.client.Do(ctx, "FT.INFO", v.index).Err()
	if err == nil {
		return true, nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "not found") || strings.Contains(msg, "unknown index") || strings.Contains(msg, "no such index") {
		return false, nil
	}
	return false, err
}

// Drop removes the index and deletes every document under its prefix.
func (v *Valkey) Drop(ctx context.Context) error {
	if err := v.client.Do(ctx, "FT.DROPINDEX", v.index).Err(); err != nil {
		return err
	}
	iter := v.client.Scan(ctx, 0, v.prefix+"*", 500).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return err
	}
	if len(keys) > 0 {
		return v.client.Del(ctx, keys...).Err()
	}
	return nil
}

// Optimize is a no-op for Valkey.
func (v *Valkey) Optimize(context.Context) error { return nil }

func docID(d *document.Document) string {
	if d.ID != "" {
		return d.ID
	}
	sum := sha256.Sum256([]byte(d.Content))
	return hex.EncodeToString(sum[:])
}

func float32Bytes(vec []float64) []byte {
	b := make([]byte, 4*len(vec))
	for i, f := range vec {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(float32(f)))
	}
	return b
}

func toString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	case nil:
		return ""
	case int, int64, float64, bool:
		return fmt.Sprint(t)
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprint(t)
		}
		return string(b)
	}
}

func (v *Valkey) hash(d *document.Document, filters map[string]interface{}) (map[string]interface{}, error) {
	if len(d.Embeddings) != v.Dimensions && v.Dimensions > 0 {
		return nil, fmt.Errorf("valkey: document %s embedding has %d dimensions, expected %d", d.ID, len(d.Embeddings), v.Dimensions)
	}
	for _, f := range d.Embeddings {
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return nil, fmt.Errorf("valkey: document %s embedding contains non-finite values", d.ID)
		}
	}
	h := map[string]interface{}{}
	merge := func(m map[string]interface{}) {
		for k, val := range m {
			if reservedHashFields[k] || val == nil {
				continue
			}
			h[k] = toString(val)
		}
	}
	merge(d.Metadata)
	merge(filters)
	h["id"] = docID(d)
	h["content"] = d.Content
	h["embedding"] = float32Bytes(d.Embeddings)
	if d.Name != "" {
		h["name"] = d.Name
	}
	if d.Source != "" {
		h["source"] = d.Source
	}
	if d.ParentID != "" {
		h["content_id"] = d.ParentID
	}
	if len(d.Metadata) > 0 {
		if b, err := json.Marshal(d.Metadata); err == nil {
			h["meta_data"] = string(b)
		}
	}
	return h, nil
}

// Insert embeds documents and stores them as hashes. filters are stored as
// extra hash fields so they can be used as TAG filters later.
func (v *Valkey) Insert(ctx context.Context, docs []*document.Document, filters map[string]interface{}) error {
	if err := v.EmbedDocuments(docs); err != nil {
		return err
	}
	pipe := v.client.Pipeline()
	for _, d := range docs {
		h, err := v.hash(d, filters)
		if err != nil {
			return err
		}
		pipe.HSet(ctx, v.prefix+docID(d), h)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// Upsert is equivalent to Insert (HSET overwrites the hash).
func (v *Valkey) Upsert(ctx context.Context, docs []*document.Document, filters map[string]interface{}) error {
	return v.Insert(ctx, docs, filters)
}

// EscapeTag escapes TAG special characters so value matches a single literal tag.
func EscapeTag(value string) string {
	var b strings.Builder
	for _, c := range value {
		if unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_' {
			b.WriteRune(c)
		} else {
			b.WriteRune('\\')
			b.WriteRune(c)
		}
	}
	return b.String()
}

func escapeQueryText(q string) string {
	var toks []string
	for _, tok := range strings.Fields(q) {
		t := strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
				return r
			}
			return -1
		}, tok)
		if t != "" {
			toks = append(toks, t)
		}
	}
	return strings.Join(toks, " ")
}

// BuildFilter turns metadata filters into an FT.SEARCH TAG expression. Only
// FilterableTagFields are used; list values match any of their elements.
func BuildFilter(filters map[string]interface{}) string {
	keys := make([]string, 0, len(filters))
	for k := range filters {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var clauses []string
	for _, k := range keys {
		if !FilterableTagFields[k] || filters[k] == nil {
			continue
		}
		var tag string
		switch val := filters[k].(type) {
		case []string:
			parts := make([]string, len(val))
			for i, s := range val {
				parts[i] = EscapeTag(s)
			}
			tag = strings.Join(parts, "|")
		case []interface{}:
			parts := make([]string, len(val))
			for i, s := range val {
				parts[i] = EscapeTag(toString(s))
			}
			tag = strings.Join(parts, "|")
		default:
			tag = EscapeTag(toString(val))
		}
		if tag == "" {
			continue
		}
		clauses = append(clauses, fmt.Sprintf("@%s:{%s}", k, tag))
	}
	return strings.Join(clauses, " ")
}

// Search dispatches on the configured search type.
func (v *Valkey) Search(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	switch v.SearchType {
	case vectordb.SearchTypeKeyword:
		return v.KeywordSearch(ctx, query, limit, filters)
	case vectordb.SearchTypeHybrid:
		return v.HybridSearch(ctx, query, limit, filters)
	default:
		return v.VectorSearch(ctx, query, limit, filters)
	}
}

// VectorSearch runs a KNN query with the TAG filters as pre-filter.
func (v *Valkey) VectorSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}
	vec, err := v.EmbedQuery(query)
	if err != nil {
		return nil, err
	}
	if len(vec) == 0 {
		return nil, errors.New("valkey: empty query embedding (no embedder configured?)")
	}
	pre := "*"
	if f := BuildFilter(filters); f != "" {
		pre = "(" + f + ")"
	}
	q := fmt.Sprintf("%s=>[KNN %d @embedding $query_vector AS score]", pre, limit)
	res, err := v.client.Do(ctx, "FT.SEARCH", v.index, q,
		"PARAMS", "2", "query_vector", float32Bytes(vec),
		"RETURN", "6", "id", "name", "content", "source", "meta_data", "score",
		"LIMIT", "0", limit,
		"DIALECT", "2",
	).Result()
	if err != nil {
		return nil, err
	}
	out := parseSearch(res, v.prefix)
	for _, r := range out {
		r.Score = v.scoreFromDistance(r.Distance)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out, nil
}

func (v *Valkey) scoreFromDistance(d float64) float64 {
	switch v.Distance {
	case vectordb.DistanceL2, vectordb.DistanceEuclidean:
		return 1 / (1 + d)
	default: // COSINE and IP distances are 1 - similarity
		return 1 - d
	}
}

// KeywordSearch runs a full-text query on the content field.
func (v *Valkey) KeywordSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}
	clauses := []string{}
	if t := escapeQueryText(query); t != "" {
		clauses = append(clauses, "@content:("+t+")")
	}
	if f := BuildFilter(filters); f != "" {
		clauses = append(clauses, f)
	}
	q := strings.Join(clauses, " ")
	if q == "" {
		q = "*"
	}
	res, err := v.client.Do(ctx, "FT.SEARCH", v.index, q,
		"RETURN", "5", "id", "name", "content", "source", "meta_data",
		"LIMIT", "0", limit,
		"DIALECT", "2",
	).Result()
	if err != nil {
		return nil, err
	}
	out := parseSearch(res, v.prefix)
	for i, r := range out {
		r.Score = 1 / float64(i+1)
	}
	return out, nil
}

// HybridSearch fuses vector and keyword results with reciprocal rank fusion
// (k=60), deduplicating by document ID.
func (v *Valkey) HybridSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}
	vecRes, err := v.VectorSearch(ctx, query, limit, filters)
	if err != nil {
		return nil, err
	}
	kwRes, err := v.KeywordSearch(ctx, query, limit, filters)
	if err != nil {
		return nil, err
	}
	const k = 60.0
	scores := map[string]float64{}
	byID := map[string]*vectordb.SearchResult{}
	var order []string
	for _, list := range [][]*vectordb.SearchResult{vecRes, kwRes} {
		for rank, r := range list {
			id := r.Document.ID
			if _, ok := byID[id]; !ok {
				byID[id] = r
				order = append(order, id)
			}
			scores[id] += 1 / (k + float64(rank+1))
		}
	}
	out := make([]*vectordb.SearchResult, 0, len(order))
	for _, id := range order {
		r := byID[id]
		r.Score = scores[id]
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// parseSearch decodes a RESP2 FT.SEARCH reply: [count, key, [f, v, ...], ...].
func parseSearch(res interface{}, prefix string) []*vectordb.SearchResult {
	arr, ok := res.([]interface{})
	if !ok || len(arr) < 1 {
		return nil
	}
	var out []*vectordb.SearchResult
	for i := 1; i+1 < len(arr); i += 2 {
		key := toString(arr[i])
		fields, _ := arr[i+1].([]interface{})
		doc := &document.Document{ID: strings.TrimPrefix(key, prefix)}
		var dist float64
		for j := 0; j+1 < len(fields); j += 2 {
			val := toString(fields[j+1])
			switch toString(fields[j]) {
			case "id":
				doc.ID = val
			case "name":
				doc.Name = val
			case "content":
				doc.Content = val
			case "source":
				doc.Source = val
			case "meta_data":
				_ = json.Unmarshal([]byte(val), &doc.Metadata)
			case "score", "__embedding_score":
				dist, _ = strconv.ParseFloat(val, 64)
			}
		}
		out = append(out, &vectordb.SearchResult{Document: doc, Distance: dist})
	}
	return out
}

func (v *Valkey) countQuery(ctx context.Context, q string) (int64, error) {
	res, err := v.client.Do(ctx, "FT.SEARCH", v.index, q, "LIMIT", "0", "0", "DIALECT", "2").Result()
	if err != nil {
		return 0, err
	}
	arr, ok := res.([]interface{})
	if !ok || len(arr) == 0 {
		return 0, nil
	}
	switch n := arr[0].(type) {
	case int64:
		return n, nil
	default:
		return strconv.ParseInt(toString(n), 10, 64)
	}
}

// GetCount returns the number of indexed documents.
func (v *Valkey) GetCount(ctx context.Context) (int64, error) { return v.countQuery(ctx, "*") }

// DocExists checks existence by the document's ID (or content hash).
func (v *Valkey) DocExists(ctx context.Context, doc *document.Document) (bool, error) {
	return v.IDExists(ctx, docID(doc))
}

// NameExists checks whether any document has the given name.
func (v *Valkey) NameExists(ctx context.Context, name string) (bool, error) {
	n, err := v.countQuery(ctx, fmt.Sprintf("@name:{%s}", EscapeTag(name)))
	return n > 0, err
}

// IDExists checks whether the document hash exists.
func (v *Valkey) IDExists(ctx context.Context, id string) (bool, error) {
	n, err := v.client.Exists(ctx, v.prefix+id).Result()
	return n > 0, err
}

// DeleteByID removes a document.
func (v *Valkey) DeleteByID(ctx context.Context, id string) error {
	return v.client.Del(ctx, v.prefix+id).Err()
}

// Close closes the client.
func (v *Valkey) Close() error { return v.client.Close() }
