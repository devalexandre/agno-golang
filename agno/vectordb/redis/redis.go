// Package redis implements the vectordb.VectorDB interface using Redis with the
// RediSearch module (Redis Stack), via github.com/redis/go-redis/v9. Documents
// are stored as hashes and queried with a KNN vector search.
package redis

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"strings"

	goredis "github.com/redis/go-redis/v9"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

// RedisDB implements vectordb.VectorDB backed by Redis Stack (RediSearch).
type RedisDB struct {
	*vectordb.BaseVectorDB
	client    *goredis.Client
	index     string // RediSearch index name
	prefix    string // key prefix for documents
	dimension int
}

// Options configures a RedisDB.
type Options struct {
	Addr      string // e.g. localhost:6379
	Password  string
	DB        int
	Client    *goredis.Client
	Index     string // index name (defaults to "agno_idx")
	Prefix    string // key prefix (defaults to "agno:")
	Dimension int
	Embedder  embedder.Embedder
}

// New creates a RedisDB. Provide either Addr or an existing Client.
func New(opts Options) *RedisDB {
	if opts.Index == "" {
		opts.Index = "agno_idx"
	}
	if opts.Prefix == "" {
		opts.Prefix = "agno:"
	}
	dim := opts.Dimension
	if dim <= 0 && opts.Embedder != nil {
		dim = opts.Embedder.GetDimensions()
	}
	client := opts.Client
	if client == nil {
		client = goredis.NewClient(&goredis.Options{Addr: opts.Addr, Password: opts.Password, DB: opts.DB})
	}
	return &RedisDB{
		BaseVectorDB: vectordb.NewBaseVectorDB(opts.Embedder, vectordb.SearchTypeVector, vectordb.DistanceCosine),
		client:       client,
		index:        opts.Index,
		prefix:       opts.Prefix,
		dimension:    dim,
	}
}

// float32Bytes encodes a vector as little-endian float32 bytes (RediSearch format).
func float32Bytes(v []float64) []byte {
	b := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(float32(f)))
	}
	return b
}

// Create builds the RediSearch index over the document hashes.
func (r *RedisDB) Create(ctx context.Context) error {
	if exists, _ := r.Exists(ctx); exists {
		return nil
	}
	if r.dimension <= 0 {
		return fmt.Errorf("redis: dimension is required to create the index")
	}
	// FT.CREATE idx ON HASH PREFIX 1 agno: SCHEMA content TEXT embedding VECTOR HNSW 6 TYPE FLOAT32 DIM d DISTANCE_METRIC COSINE
	args := []interface{}{
		"FT.CREATE", r.index, "ON", "HASH", "PREFIX", "1", r.prefix, "SCHEMA",
		"content", "TEXT",
		"name", "TEXT",
		"source", "TAG",
		"embedding", "VECTOR", "HNSW", "6", "TYPE", "FLOAT32", "DIM", r.dimension, "DISTANCE_METRIC", "COSINE",
	}
	return r.client.Do(ctx, args...).Err()
}

// Exists reports whether the index exists.
func (r *RedisDB) Exists(ctx context.Context) (bool, error) {
	err := r.client.Do(ctx, "FT.INFO", r.index).Err()
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unknown index") || strings.Contains(err.Error(), "no such index") {
			return false, nil
		}
		return false, nil
	}
	return true, nil
}

// Drop deletes the index (documents are kept unless DD is desired).
func (r *RedisDB) Drop(ctx context.Context) error {
	return r.client.Do(ctx, "FT.DROPINDEX", r.index, "DD").Err()
}

// Optimize is a no-op for Redis.
func (r *RedisDB) Optimize(ctx context.Context) error { return nil }

func (r *RedisDB) key(id string) string { return r.prefix + id }

// Insert embeds and writes documents as hashes.
func (r *RedisDB) Insert(ctx context.Context, docs []*document.Document, _ map[string]interface{}) error {
	if err := r.EmbedDocuments(docs); err != nil {
		return err
	}
	pipe := r.client.Pipeline()
	for _, d := range docs {
		pipe.HSet(ctx, r.key(d.ID), map[string]interface{}{
			"content":   d.Content,
			"name":      d.Name,
			"source":    d.Source,
			"embedding": float32Bytes(d.Embeddings),
		})
	}
	_, err := pipe.Exec(ctx)
	return err
}

// Upsert is equivalent to Insert (HSET overwrites).
func (r *RedisDB) Upsert(ctx context.Context, docs []*document.Document, filters map[string]interface{}) error {
	return r.Insert(ctx, docs, filters)
}

// Search performs a vector search.
func (r *RedisDB) Search(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return r.VectorSearch(ctx, query, limit, filters)
}

// VectorSearch runs a KNN search via FT.SEARCH.
func (r *RedisDB) VectorSearch(ctx context.Context, query string, limit int, _ map[string]interface{}) ([]*vectordb.SearchResult, error) {
	vec, err := r.EmbedQuery(query)
	if err != nil {
		return nil, err
	}
	q := fmt.Sprintf("*=>[KNN %d @embedding $vec AS score]", limit)
	res, err := r.client.Do(ctx,
		"FT.SEARCH", r.index, q,
		"PARAMS", "2", "vec", float32Bytes(vec),
		"SORTBY", "score",
		"RETURN", "4", "content", "name", "source", "score",
		"DIALECT", "2",
	).Result()
	if err != nil {
		return nil, err
	}
	return parseSearch(res), nil
}

// parseSearch decodes the FT.SEARCH reply (RESP map/array) into SearchResults.
func parseSearch(res interface{}) []*vectordb.SearchResult {
	arr, ok := res.([]interface{})
	if !ok || len(arr) < 1 {
		// RESP3 may return a map with a "results" key.
		if m, ok := res.(map[interface{}]interface{}); ok {
			if results, ok := m["results"].([]interface{}); ok {
				return parseRESP3Results(results)
			}
		}
		return nil
	}
	// RESP2: [count, key1, [field, val, ...], key2, [...], ...]
	var out []*vectordb.SearchResult
	for i := 1; i+1 < len(arr); i += 2 {
		id, _ := arr[i].(string)
		fields, _ := arr[i+1].([]interface{})
		doc := &document.Document{ID: strings.TrimPrefix(id, prefixOf(id))}
		var score float64
		for j := 0; j+1 < len(fields); j += 2 {
			k, _ := fields[j].(string)
			v, _ := fields[j+1].(string)
			switch k {
			case "content":
				doc.Content = v
			case "name":
				doc.Name = v
			case "source":
				doc.Source = v
			case "score":
				fmt.Sscanf(v, "%g", &score)
			}
		}
		out = append(out, &vectordb.SearchResult{Document: doc, Score: 1 - score, Distance: score})
	}
	return out
}

func parseRESP3Results(results []interface{}) []*vectordb.SearchResult {
	var out []*vectordb.SearchResult
	for _, item := range results {
		m, ok := item.(map[interface{}]interface{})
		if !ok {
			continue
		}
		doc := &document.Document{}
		if id, ok := m["id"].(string); ok {
			doc.ID = id
		}
		var score float64
		if attrs, ok := m["extra_attributes"].(map[interface{}]interface{}); ok {
			if c, ok := attrs["content"].(string); ok {
				doc.Content = c
			}
			if n, ok := attrs["name"].(string); ok {
				doc.Name = n
			}
			if s, ok := attrs["source"].(string); ok {
				doc.Source = s
			}
			if sc, ok := attrs["score"].(string); ok {
				fmt.Sscanf(sc, "%g", &score)
			}
		}
		out = append(out, &vectordb.SearchResult{Document: doc, Score: 1 - score, Distance: score})
	}
	return out
}

func prefixOf(key string) string {
	if i := strings.Index(key, ":"); i >= 0 {
		return key[:i+1]
	}
	return ""
}

// KeywordSearch runs a full-text search on the content field.
func (r *RedisDB) KeywordSearch(ctx context.Context, query string, limit int, _ map[string]interface{}) ([]*vectordb.SearchResult, error) {
	res, err := r.client.Do(ctx,
		"FT.SEARCH", r.index, query,
		"LIMIT", "0", limit,
		"RETURN", "3", "content", "name", "source",
		"DIALECT", "2",
	).Result()
	if err != nil {
		return nil, err
	}
	return parseSearch(res), nil
}

// HybridSearch is not implemented.
func (r *RedisDB) HybridSearch(ctx context.Context, query string, limit int, _ map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return nil, fmt.Errorf("redis: hybrid search not implemented")
}

// GetCount returns the number of indexed documents.
func (r *RedisDB) GetCount(ctx context.Context) (int64, error) {
	keys, err := r.client.Keys(ctx, r.prefix+"*").Result()
	if err != nil {
		return 0, err
	}
	return int64(len(keys)), nil
}

// DocExists checks existence by the document's ID.
func (r *RedisDB) DocExists(ctx context.Context, doc *document.Document) (bool, error) {
	return r.IDExists(ctx, doc.ID)
}

// NameExists is not supported efficiently.
func (r *RedisDB) NameExists(ctx context.Context, name string) (bool, error) {
	return false, fmt.Errorf("redis: NameExists not supported")
}

// IDExists checks whether the document hash exists.
func (r *RedisDB) IDExists(ctx context.Context, id string) (bool, error) {
	n, err := r.client.Exists(ctx, r.key(id)).Result()
	return n > 0, err
}

// Close closes the Redis client.
func (r *RedisDB) Close() error { return r.client.Close() }

var _ vectordb.VectorDB = (*RedisDB)(nil)
