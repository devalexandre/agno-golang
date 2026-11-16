// Package elasticsearch implements the vectordb.VectorDB interface for
// Elasticsearch using its REST API directly (no client dependency).
//
// Documents are stored with a dense_vector "embedding" field and searched with
// native kNN (pre-filtered), multi_match keyword queries, or a hybrid of both
// (boosted sum by default, reciprocal rank fusion opt-in), mirroring Agno
// Python's agno.vectordb.elasticsearch.
package elasticsearch

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

// Similarity is the similarity function of the dense_vector field.
type Similarity string

const (
	SimilarityCosine          Similarity = "cosine"
	SimilarityL2Norm          Similarity = "l2_norm"
	SimilarityDotProduct      Similarity = "dot_product"
	SimilarityMaxInnerProduct Similarity = "max_inner_product"
)

// HybridStrategy selects how hybrid search combines its vector and keyword halves.
type HybridStrategy string

const (
	// HybridBoost sums knn (boost 0.7) and multi_match (boost 0.3) scores. Works
	// on every distribution/licence.
	HybridBoost HybridStrategy = "boost"
	// HybridRRF uses reciprocal rank fusion (requires a licensed cluster). On a
	// licence refusal the DB falls back to HybridBoost for the rest of its life.
	HybridRRF HybridStrategy = "rrf"
)

const (
	// DefaultAddress is used when Config.Addresses is empty.
	DefaultAddress = "http://localhost:9200"
	// DefaultIndexName is used when Config.IndexName is empty.
	DefaultIndexName = "agno_documents"

	numCandidatesMultiplier = 10
	minNumCandidates        = 50
	maxNumCandidates        = 10000
	rrfMinRankWindowSize    = 10
	vectorBoost             = 0.7
	keywordBoost            = 0.3
)

// Config configures an Elasticsearch vector DB.
type Config struct {
	// Addresses of cluster nodes; tried in order on transport errors.
	// Credentials embedded in the URL (https://user:pass@host) are honoured.
	Addresses []string
	IndexName string
	// APIKey is sent as "Authorization: ApiKey <APIKey>" (the encoded key).
	APIKey string
	// Username/Password enable HTTP basic auth.
	Username string
	Password string

	Embedder embedder.Embedder
	// Dimensions of the dense_vector field; defaults to the embedder's.
	Dimensions int
	// Distance is mapped to a Similarity when Similarity is empty
	// (l2 -> l2_norm, max_inner_product -> max_inner_product, else cosine).
	Distance       vectordb.Distance
	Similarity     Similarity
	SearchType     vectordb.SearchType
	HybridStrategy HybridStrategy
	// NumCandidates per shard for kNN; default max(limit*10, 50), capped at 10000.
	NumCandidates int
	// IndexSettings override the default {"index":{"number_of_replicas":0}}.
	IndexSettings map[string]interface{}

	HTTPClient *http.Client
	// Timeout for the default HTTP client (default 30s).
	Timeout time.Duration
}

// Elasticsearch implements vectordb.VectorDB over the Elasticsearch REST API.
type Elasticsearch struct {
	*vectordb.BaseVectorDB
	cfg        Config
	client     *http.Client
	similarity Similarity

	mu             sync.Mutex
	hybridStrategy HybridStrategy
}

var _ vectordb.VectorDB = (*Elasticsearch)(nil)

// New creates an Elasticsearch vector DB. It does not contact the cluster.
func New(cfg Config) (*Elasticsearch, error) {
	if len(cfg.Addresses) == 0 {
		cfg.Addresses = []string{DefaultAddress}
	}
	for i, a := range cfg.Addresses {
		cfg.Addresses[i] = strings.TrimRight(a, "/")
	}
	if cfg.IndexName == "" {
		cfg.IndexName = DefaultIndexName
	}
	if cfg.SearchType == "" {
		cfg.SearchType = vectordb.SearchTypeVector
	}
	if cfg.Distance == "" {
		cfg.Distance = vectordb.DistanceCosine
	}
	if cfg.HybridStrategy == "" {
		cfg.HybridStrategy = HybridBoost
	}
	if cfg.HybridStrategy != HybridBoost && cfg.HybridStrategy != HybridRRF {
		return nil, fmt.Errorf("elasticsearch: unknown hybrid strategy %q", cfg.HybridStrategy)
	}
	base := vectordb.NewBaseVectorDB(cfg.Embedder, cfg.SearchType, cfg.Distance)
	if cfg.Dimensions > 0 {
		base.Dimensions = cfg.Dimensions
	}
	sim := cfg.Similarity
	if sim == "" {
		sim = similarityForDistance(cfg.Distance)
	}
	client := cfg.HTTPClient
	if client == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	return &Elasticsearch{
		BaseVectorDB:   base,
		cfg:            cfg,
		client:         client,
		similarity:     sim,
		hybridStrategy: cfg.HybridStrategy,
	}, nil
}

func similarityForDistance(d vectordb.Distance) Similarity {
	switch d {
	case vectordb.DistanceL2, vectordb.DistanceEuclidean:
		return SimilarityL2Norm
	case vectordb.DistanceMaxInnerProduct, vectordb.DistanceIP:
		return SimilarityMaxInnerProduct
	case vectordb.DistanceDot:
		return SimilarityDotProduct
	default:
		return SimilarityCosine
	}
}

// ---------------------------------------------------------------- transport

type apiError struct {
	Status int
	Body   string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("elasticsearch: HTTP %d: %s", e.Status, e.Body)
}

// do sends a request, failing over across addresses on transport errors. body
// may be nil, a []byte (sent as-is with contentType) or a value encoded as JSON.
func (e *Elasticsearch) do(ctx context.Context, method, path string, body interface{}, contentType string) (int, []byte, error) {
	var payload []byte
	switch b := body.(type) {
	case nil:
	case []byte:
		payload = b
	default:
		var err error
		if payload, err = json.Marshal(b); err != nil {
			return 0, nil, err
		}
	}
	if contentType == "" {
		contentType = "application/json"
	}

	var lastErr error
	for _, addr := range e.cfg.Addresses {
		var rdr io.Reader
		if payload != nil {
			rdr = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, addr+path, rdr)
		if err != nil {
			return 0, nil, err
		}
		if payload != nil {
			req.Header.Set("Content-Type", contentType)
		}
		req.Header.Set("Accept", "application/json")
		switch {
		case e.cfg.APIKey != "":
			req.Header.Set("Authorization", "ApiKey "+e.cfg.APIKey)
		case e.cfg.Username != "":
			req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(e.cfg.Username+":"+e.cfg.Password)))
		}
		resp, err := e.client.Do(req)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				break
			}
			continue
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		return resp.StatusCode, data, err
	}
	return 0, nil, fmt.Errorf("elasticsearch: request failed: %w", lastErr)
}

// doJSON performs a request and decodes a 2xx JSON response into out.
func (e *Elasticsearch) doJSON(ctx context.Context, method, path string, body interface{}, out interface{}) error {
	status, data, err := e.do(ctx, method, path, body, "")
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return &apiError{Status: status, Body: string(data)}
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}

func (e *Elasticsearch) indexPath() string { return "/" + url.PathEscape(e.cfg.IndexName) }

// ---------------------------------------------------------------- index lifecycle

// Mappings returns the index mappings used by Create.
func (e *Elasticsearch) Mappings() map[string]interface{} {
	keywordSub := map[string]interface{}{"keyword": map[string]interface{}{"type": "keyword", "ignore_above": 256}}
	return map[string]interface{}{
		// Keep date-shaped strings as text so .keyword equality filters work.
		"date_detection": false,
		"dynamic_templates": []interface{}{
			map[string]interface{}{
				"meta_data_strings": map[string]interface{}{
					"path_match":         "meta_data.*",
					"match_mapping_type": "string",
					"mapping":            map[string]interface{}{"type": "text", "fields": keywordSub},
				},
			},
		},
		"properties": map[string]interface{}{
			"embedding": map[string]interface{}{
				"type":       "dense_vector",
				"dims":       e.Dimensions,
				"index":      true,
				"similarity": string(e.similarity),
			},
			"content":      map[string]interface{}{"type": "text", "analyzer": "standard"},
			"name":         map[string]interface{}{"type": "text", "fields": keywordSub},
			"meta_data":    map[string]interface{}{"type": "object", "dynamic": true},
			"source":       map[string]interface{}{"type": "keyword"},
			"content_hash": map[string]interface{}{"type": "keyword"},
		},
	}
}

func (e *Elasticsearch) settings() map[string]interface{} {
	if e.cfg.IndexSettings != nil {
		return e.cfg.IndexSettings
	}
	// No replica keeps single-node clusters green.
	return map[string]interface{}{"index": map[string]interface{}{"number_of_replicas": 0}}
}

// Create creates the index with a dense_vector mapping if it does not exist.
func (e *Elasticsearch) Create(ctx context.Context) error {
	exists, err := e.Exists(ctx)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if e.Dimensions <= 0 {
		return errors.New("elasticsearch: vector dimensions unknown; set Config.Dimensions or an embedder")
	}
	body := map[string]interface{}{"mappings": e.Mappings(), "settings": e.settings()}
	err = e.doJSON(ctx, http.MethodPut, e.indexPath(), body, nil)
	var ae *apiError
	if errors.As(err, &ae) {
		if strings.Contains(ae.Body, "resource_already_exists_exception") {
			return nil
		}
		// Serverless projects reject shard/replica settings: retry without them.
		if e.cfg.IndexSettings == nil && ae.Status == http.StatusBadRequest && strings.Contains(ae.Body, "number_of_replicas") {
			delete(body, "settings")
			err = e.doJSON(ctx, http.MethodPut, e.indexPath(), body, nil)
		}
	}
	return err
}

// Exists reports whether the index exists.
func (e *Elasticsearch) Exists(ctx context.Context) (bool, error) {
	status, data, err := e.do(ctx, http.MethodHead, e.indexPath(), nil, "")
	if err != nil {
		return false, err
	}
	switch status {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, &apiError{Status: status, Body: string(data)}
	}
}

// Drop deletes the index (a missing index is not an error).
func (e *Elasticsearch) Drop(ctx context.Context) error {
	status, data, err := e.do(ctx, http.MethodDelete, e.indexPath(), nil, "")
	if err != nil {
		return err
	}
	if status == http.StatusNotFound || (status >= 200 && status < 300) {
		return nil
	}
	return &apiError{Status: status, Body: string(data)}
}

// Optimize force-merges the index into a single segment.
func (e *Elasticsearch) Optimize(ctx context.Context) error {
	return e.doJSON(ctx, http.MethodPost, e.indexPath()+"/_forcemerge?max_num_segments=1", nil, nil)
}

// GetCount returns the number of documents in the index (0 if missing).
func (e *Elasticsearch) GetCount(ctx context.Context) (int64, error) {
	var out struct {
		Count int64 `json:"count"`
	}
	err := e.doJSON(ctx, http.MethodGet, e.indexPath()+"/_count", nil, &out)
	var ae *apiError
	if errors.As(err, &ae) && ae.Status == http.StatusNotFound {
		return 0, nil
	}
	return out.Count, err
}

// ---------------------------------------------------------------- writes

// DocumentID returns the Elasticsearch _id used for doc: its ID, or the MD5 of
// its content when the ID is empty.
func DocumentID(doc *document.Document) string {
	if doc.ID != "" {
		return doc.ID
	}
	sum := md5.Sum([]byte(doc.Content))
	return hex.EncodeToString(sum[:])
}

func (e *Elasticsearch) buildIndexDoc(doc *document.Document, filters map[string]interface{}) map[string]interface{} {
	meta := make(map[string]interface{}, len(doc.Metadata)+len(filters))
	for k, v := range doc.Metadata {
		meta[k] = v
	}
	for k, v := range filters {
		meta[k] = v
	}
	out := map[string]interface{}{
		"embedding": doc.Embeddings,
		"content":   doc.Content,
		"meta_data": meta,
	}
	if doc.Name != "" {
		out["name"] = doc.Name
	}
	if doc.Source != "" {
		out["source"] = doc.Source
	}
	if h, ok := meta["content_hash"]; ok && h != nil {
		out["content_hash"] = h
	}
	return out
}

func (e *Elasticsearch) bulk(ctx context.Context, docs []*document.Document, filters map[string]interface{}, upsert bool) error {
	if len(docs) == 0 {
		return nil
	}
	if err := e.EmbedDocuments(docs); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		if len(doc.Embeddings) == 0 {
			return fmt.Errorf("elasticsearch: document %q has no embedding (configure an embedder)", doc.ID)
		}
		if e.Dimensions > 0 && len(doc.Embeddings) != e.Dimensions {
			return fmt.Errorf("elasticsearch: document %q embedding has %d dims, index expects %d", doc.ID, len(doc.Embeddings), e.Dimensions)
		}
		id := DocumentID(doc)
		if doc.ID == "" {
			doc.ID = id
		}
		src := e.buildIndexDoc(doc, filters)
		action, payload := "index", interface{}(src)
		if upsert {
			action = "update"
			payload = map[string]interface{}{"doc": src, "doc_as_upsert": true}
		}
		if err := enc.Encode(map[string]interface{}{action: map[string]interface{}{"_index": e.cfg.IndexName, "_id": id}}); err != nil {
			return err
		}
		if err := enc.Encode(payload); err != nil {
			return err
		}
	}
	status, data, err := e.do(ctx, http.MethodPost, "/_bulk?refresh=true", buf.Bytes(), "application/x-ndjson")
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return &apiError{Status: status, Body: string(data)}
	}
	var resp struct {
		Errors bool                                `json:"errors"`
		Items  []map[string]map[string]interface{} `json:"items"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("elasticsearch: decode bulk response: %w", err)
	}
	if !resp.Errors {
		return nil
	}
	var failed int
	var samples []string
	for _, item := range resp.Items {
		for _, res := range item {
			if res["error"] != nil {
				failed++
				if len(samples) < 3 {
					b, _ := json.Marshal(res["error"])
					samples = append(samples, fmt.Sprintf("%v: %s", res["_id"], b))
				}
			}
		}
	}
	return fmt.Errorf("elasticsearch: bulk %s failed for %d/%d documents: %s", opName(upsert), failed, len(resp.Items), strings.Join(samples, "; "))
}

func opName(upsert bool) string {
	if upsert {
		return "upsert"
	}
	return "insert"
}

// Insert embeds and indexes documents (bulk "index", overwriting same _id).
// Filters are merged into each document's meta_data.
func (e *Elasticsearch) Insert(ctx context.Context, docs []*document.Document, filters map[string]interface{}) error {
	return e.bulk(ctx, docs, filters, false)
}

// Upsert embeds and upserts documents by _id (bulk "update" with doc_as_upsert).
func (e *Elasticsearch) Upsert(ctx context.Context, docs []*document.Document, filters map[string]interface{}) error {
	return e.bulk(ctx, docs, filters, true)
}

// ---------------------------------------------------------------- existence

// IDExists reports whether a document with the given _id exists.
func (e *Elasticsearch) IDExists(ctx context.Context, id string) (bool, error) {
	status, data, err := e.do(ctx, http.MethodHead, e.indexPath()+"/_doc/"+url.PathEscape(id), nil, "")
	if err != nil {
		return false, err
	}
	switch status {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, &apiError{Status: status, Body: string(data)}
	}
}

// DocExists reports whether doc (by DocumentID) is indexed.
func (e *Elasticsearch) DocExists(ctx context.Context, doc *document.Document) (bool, error) {
	if doc == nil {
		return false, nil
	}
	return e.IDExists(ctx, DocumentID(doc))
}

// NameExists reports whether any document has the exact name.
func (e *Elasticsearch) NameExists(ctx context.Context, name string) (bool, error) {
	body := map[string]interface{}{
		"size":  0,
		"query": map[string]interface{}{"term": map[string]interface{}{"name.keyword": name}},
	}
	var out struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
		} `json:"hits"`
	}
	err := e.doJSON(ctx, http.MethodPost, e.indexPath()+"/_search", body, &out)
	var ae *apiError
	if errors.As(err, &ae) && ae.Status == http.StatusNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return out.Hits.Total.Value > 0, nil
}

// ---------------------------------------------------------------- search

// Search dispatches to the configured search type.
func (e *Elasticsearch) Search(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	switch e.SearchType {
	case vectordb.SearchTypeKeyword:
		return e.KeywordSearch(ctx, query, limit, filters)
	case vectordb.SearchTypeHybrid:
		return e.HybridSearch(ctx, query, limit, filters)
	default:
		return e.VectorSearch(ctx, query, limit, filters)
	}
}

func (e *Elasticsearch) queryVector(query string) ([]float64, error) {
	if e.Embedder == nil {
		return nil, errors.New("elasticsearch: no embedder configured for vector search")
	}
	vec, err := e.EmbedQuery(query)
	if err != nil {
		return nil, err
	}
	if len(vec) == 0 {
		return nil, errors.New("elasticsearch: empty query embedding")
	}
	return vec, nil
}

func (e *Elasticsearch) numCandidates(limit int) int {
	n := limit * numCandidatesMultiplier
	if e.cfg.NumCandidates > 0 {
		n = e.cfg.NumCandidates
	} else if n < minNumCandidates {
		n = minNumCandidates
	}
	if n < limit {
		n = limit
	}
	if n > maxNumCandidates {
		n = maxNumCandidates
	}
	return n
}

func (e *Elasticsearch) knnClause(vec []float64, limit int, filter []interface{}) map[string]interface{} {
	knn := map[string]interface{}{
		"field":          "embedding",
		"query_vector":   vec,
		"k":              limit,
		"num_candidates": e.numCandidates(limit),
	}
	if len(filter) > 0 {
		knn["filter"] = filter // pre-filtering
	}
	return knn
}

func keywordClause(query string, boost float64) map[string]interface{} {
	mm := map[string]interface{}{"query": query, "fields": []string{"content", "name"}, "type": "best_fields"}
	if boost > 0 {
		mm["boost"] = boost
	}
	return map[string]interface{}{"multi_match": mm}
}

// VectorSearch runs a pre-filtered kNN query on the embedding field.
func (e *Elasticsearch) VectorSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	limit = normLimit(limit)
	vec, err := e.queryVector(query)
	if err != nil {
		return nil, err
	}
	body := map[string]interface{}{"size": limit, "knn": e.knnClause(vec, limit, BuildFilterConditions(filters))}
	return e.runSearch(ctx, body)
}

// KeywordSearch runs a multi_match query over content and name.
func (e *Elasticsearch) KeywordSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	limit = normLimit(limit)
	var q interface{} = keywordClause(query, 0)
	if fc := BuildFilterConditions(filters); len(fc) > 0 {
		q = map[string]interface{}{"bool": map[string]interface{}{"must": []interface{}{q}, "filter": fc}}
	}
	return e.runSearch(ctx, map[string]interface{}{"size": limit, "query": q})
}

// HybridSearch combines kNN and multi_match: boosted sum (0.7/0.3) by default,
// or an rrf retriever with HybridRRF (falling back to boost on licence errors).
func (e *Elasticsearch) HybridSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	limit = normLimit(limit)
	vec, err := e.queryVector(query)
	if err != nil {
		return nil, err
	}
	body := e.hybridBody(query, vec, limit, BuildFilterConditions(filters))
	results, err := e.runSearch(ctx, body)
	if err != nil && e.currentStrategy() == HybridRRF && isUnlicensedRRF(err) {
		e.mu.Lock()
		e.hybridStrategy = HybridBoost
		e.mu.Unlock()
		return e.runSearch(ctx, e.hybridBody(query, vec, limit, BuildFilterConditions(filters)))
	}
	return results, err
}

func (e *Elasticsearch) currentStrategy() HybridStrategy {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.hybridStrategy
}

func (e *Elasticsearch) hybridBody(query string, vec []float64, limit int, filter []interface{}) map[string]interface{} {
	if e.currentStrategy() == HybridRRF {
		var kw interface{} = keywordClause(query, 0)
		if len(filter) > 0 {
			kw = map[string]interface{}{"bool": map[string]interface{}{"must": []interface{}{kw}, "filter": filter}}
		}
		window := limit
		if window < rrfMinRankWindowSize {
			window = rrfMinRankWindowSize
		}
		return map[string]interface{}{
			"size": limit,
			"retriever": map[string]interface{}{
				"rrf": map[string]interface{}{
					"rank_window_size": window,
					"retrievers": []interface{}{
						map[string]interface{}{"standard": map[string]interface{}{"query": kw}},
						map[string]interface{}{"knn": e.knnClause(vec, limit, filter)},
					},
				},
			},
		}
	}
	knn := e.knnClause(vec, limit, filter)
	knn["boost"] = vectorBoost
	var kw interface{} = keywordClause(query, keywordBoost)
	if len(filter) > 0 {
		kw = map[string]interface{}{"bool": map[string]interface{}{
			"must":   []interface{}{keywordClause(query, 0)},
			"filter": filter,
			"boost":  keywordBoost,
		}}
	}
	return map[string]interface{}{"size": limit, "knn": knn, "query": kw}
}

func isUnlicensedRRF(err error) bool {
	msg := strings.ToLower(err.Error())
	msg = strings.ReplaceAll(msg, "reciprocal rank fusion", "rrf")
	return strings.Contains(msg, "license") && strings.Contains(msg, "rrf")
}

func normLimit(limit int) int {
	if limit <= 0 {
		return 5
	}
	return limit
}

type searchResponse struct {
	Hits struct {
		Hits []struct {
			ID     string   `json:"_id"`
			Score  *float64 `json:"_score"`
			Source struct {
				Content   string                 `json:"content"`
				Name      string                 `json:"name"`
				Source    string                 `json:"source"`
				MetaData  map[string]interface{} `json:"meta_data"`
				Embedding []float64              `json:"embedding"`
			} `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

func (e *Elasticsearch) runSearch(ctx context.Context, body map[string]interface{}) ([]*vectordb.SearchResult, error) {
	var out searchResponse
	err := e.doJSON(ctx, http.MethodPost, e.indexPath()+"/_search", body, &out)
	var ae *apiError
	if errors.As(err, &ae) && ae.Status == http.StatusNotFound && strings.Contains(ae.Body, "index_not_found_exception") {
		return []*vectordb.SearchResult{}, nil
	}
	if err != nil {
		return nil, err
	}
	results := make([]*vectordb.SearchResult, 0, len(out.Hits.Hits))
	for _, h := range out.Hits.Hits {
		meta := make(map[string]interface{}, len(h.Source.MetaData)+1)
		for k, v := range h.Source.MetaData {
			meta[k] = v
		}
		score := 0.0
		if h.Score != nil {
			score = *h.Score
			meta["search_score"] = score
		}
		results = append(results, &vectordb.SearchResult{
			Document: &document.Document{
				ID:         h.ID,
				Content:    h.Source.Content,
				Name:       h.Source.Name,
				Source:     h.Source.Source,
				Metadata:   meta,
				Embeddings: h.Source.Embedding,
			},
			Score: score,
		})
	}
	return results, nil
}

// ---------------------------------------------------------------- filters

// BuildFilterConditions converts agno filters into Elasticsearch filter clauses
// on meta_data fields:
//   - scalar      -> term   (strings target the .keyword subfield)
//   - list        -> terms
//   - {"$in": []} -> terms  (also "in")
//   - {"gt"/"gte"/"lt"/"lte": v} -> range
func BuildFilterConditions(filters map[string]interface{}) []interface{} {
	if len(filters) == 0 {
		return nil
	}
	var out []interface{}
	for key, value := range filters {
		if c := buildFilterCondition(key, value); c != nil {
			out = append(out, c)
		}
	}
	return out
}

func matchField(key string, sample interface{}) string {
	if _, ok := sample.(string); ok {
		return "meta_data." + key + ".keyword"
	}
	return "meta_data." + key
}

func firstOf(list []interface{}) interface{} {
	if len(list) > 0 {
		return list[0]
	}
	return nil
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

func buildFilterCondition(key string, value interface{}) map[string]interface{} {
	if m, ok := value.(map[string]interface{}); ok {
		inVal, hasIn := m["$in"]
		if !hasIn {
			inVal, hasIn = m["in"]
		}
		if hasIn {
			list, ok := toList(inVal)
			if !ok {
				return nil
			}
			return map[string]interface{}{"terms": map[string]interface{}{matchField(key, firstOf(list)): list}}
		}
		rng := map[string]interface{}{}
		var bound interface{}
		for _, op := range []string{"gt", "gte", "lt", "lte"} {
			if v, ok := m[op]; ok {
				rng[op] = v
				if bound == nil {
					bound = v
				}
			}
		}
		if len(rng) > 0 {
			return map[string]interface{}{"range": map[string]interface{}{matchField(key, bound): rng}}
		}
		return nil
	}
	if list, ok := toList(value); ok {
		return map[string]interface{}{"terms": map[string]interface{}{matchField(key, firstOf(list)): list}}
	}
	return map[string]interface{}{"term": map[string]interface{}{matchField(key, value): value}}
}
