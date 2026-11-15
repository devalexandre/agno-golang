// Package opensearch implements the vectordb.VectorDB interface for OpenSearch
// using its REST API directly (no client dependency).
//
// Documents are stored in a k-NN enabled index ("index.knn": true) with a
// knn_vector "embedding" field (HNSW, lucene/faiss/nmslib engine) and searched
// with knn queries, multi_match keyword queries, or a hybrid bool/should of both
// (knn boost 0.7 + multi_match boost 0.3), mirroring Agno Python's
// agno.vectordb.opensearch.
//
// Authentication: HTTP basic auth (Username/Password). For AWS OpenSearch
// Service with SigV4, supply an HTTPClient whose Transport signs requests.
package opensearch

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
	"github.com/devalexandre/agno-golang/agno/vectordb/elasticsearch"
)

// Engine is the k-NN engine backing the knn_vector field.
type Engine string

const (
	EngineLucene Engine = "lucene"
	EngineFaiss  Engine = "faiss"
	EngineNmslib Engine = "nmslib"
)

const (
	// DefaultURL is used when Config.URL is empty.
	DefaultURL = "http://localhost:9200"
	// DefaultIndexName is used when Config.IndexName is empty.
	DefaultIndexName = "agno_documents"

	vectorBoost  = 0.7
	keywordBoost = 0.3
)

// Config configures an OpenSearch vector DB.
type Config struct {
	// URL of the cluster (e.g. https://localhost:9200).
	URL       string
	IndexName string
	Username  string
	Password  string

	Embedder   embedder.Embedder
	Dimensions int // defaults to the embedder's dimensions
	// Distance maps to the knn space_type: cosine -> cosinesimil (default),
	// l2 -> l2, max_inner_product -> innerproduct.
	Distance   vectordb.Distance
	Engine     Engine // default lucene
	SearchType vectordb.SearchType
	// Parameters are merged over the engine defaults {"m":16,"ef_construction":512}.
	// "ef_search" (default 512) is applied as the index setting
	// knn.algo_param.ef_search instead of a method parameter.
	Parameters map[string]interface{}

	HTTPClient *http.Client
	Timeout    time.Duration // default 30s for the built-in client
}

// OpenSearch implements vectordb.VectorDB over the OpenSearch REST API.
type OpenSearch struct {
	*vectordb.BaseVectorDB
	cfg    Config
	client *http.Client
	params map[string]interface{}
}

var _ vectordb.VectorDB = (*OpenSearch)(nil)

// New creates an OpenSearch vector DB. It does not contact the cluster.
func New(cfg Config) (*OpenSearch, error) {
	if cfg.URL == "" {
		cfg.URL = DefaultURL
	}
	cfg.URL = strings.TrimRight(cfg.URL, "/")
	if cfg.IndexName == "" {
		cfg.IndexName = DefaultIndexName
	}
	if cfg.Engine == "" {
		cfg.Engine = EngineLucene
	}
	switch cfg.Engine {
	case EngineLucene, EngineFaiss, EngineNmslib:
	default:
		return nil, fmt.Errorf("opensearch: unsupported engine %q", cfg.Engine)
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
	params := map[string]interface{}{"m": 16, "ef_construction": 512}
	for k, v := range cfg.Parameters {
		params[k] = v
	}
	client := cfg.HTTPClient
	if client == nil {
		t := cfg.Timeout
		if t <= 0 {
			t = 30 * time.Second
		}
		client = &http.Client{Timeout: t}
	}
	return &OpenSearch{BaseVectorDB: base, cfg: cfg, client: client, params: params}, nil
}

// SpaceType returns the knn space_type for the configured distance.
func (o *OpenSearch) SpaceType() string {
	switch o.cfg.Distance {
	case vectordb.DistanceL2, vectordb.DistanceEuclidean:
		return "l2"
	case vectordb.DistanceMaxInnerProduct, vectordb.DistanceDot, vectordb.DistanceIP:
		return "innerproduct"
	default:
		return "cosinesimil"
	}
}

// SupportsKNNFilter reports whether the engine accepts a filter inside the knn
// clause (every engine but nmslib).
func (o *OpenSearch) SupportsKNNFilter() bool { return o.cfg.Engine != EngineNmslib }

// ---------------------------------------------------------------- transport

type apiError struct {
	Status int
	Body   string
}

func (e *apiError) Error() string { return fmt.Sprintf("opensearch: HTTP %d: %s", e.Status, e.Body) }

func (o *OpenSearch) do(ctx context.Context, method, path string, body interface{}, contentType string) (int, []byte, error) {
	var rdr io.Reader
	switch b := body.(type) {
	case nil:
	case []byte:
		rdr = bytes.NewReader(b)
	default:
		payload, err := json.Marshal(b)
		if err != nil {
			return 0, nil, err
		}
		rdr = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, o.cfg.URL+path, rdr)
	if err != nil {
		return 0, nil, err
	}
	if rdr != nil {
		if contentType == "" {
			contentType = "application/json"
		}
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")
	if o.cfg.Username != "" {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(o.cfg.Username+":"+o.cfg.Password)))
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("opensearch: request failed: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return resp.StatusCode, data, err
}

func (o *OpenSearch) doJSON(ctx context.Context, method, path string, body, out interface{}) error {
	status, data, err := o.do(ctx, method, path, body, "")
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

func (o *OpenSearch) indexPath() string { return "/" + url.PathEscape(o.cfg.IndexName) }

// ---------------------------------------------------------------- index lifecycle

// Mapping returns the full index body (settings + mappings) used by Create.
func (o *OpenSearch) Mapping() map[string]interface{} {
	keywordSub := map[string]interface{}{"keyword": map[string]interface{}{"type": "keyword", "ignore_above": 256}}
	efSearch := interface{}(512)
	methodParams := map[string]interface{}{}
	for k, v := range o.params {
		if k == "ef_search" {
			efSearch = v
			continue
		}
		methodParams[k] = v
	}
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"index": map[string]interface{}{
				"knn":                      true,
				"knn.algo_param.ef_search": efSearch,
			},
		},
		"mappings": map[string]interface{}{
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
					"type":      "knn_vector",
					"dimension": o.Dimensions,
					"method": map[string]interface{}{
						"name":       "hnsw",
						"space_type": o.SpaceType(),
						"engine":     string(o.cfg.Engine),
						"parameters": methodParams,
					},
				},
				"content":      map[string]interface{}{"type": "text", "analyzer": "standard"},
				"name":         map[string]interface{}{"type": "text", "fields": keywordSub},
				"meta_data":    map[string]interface{}{"type": "object", "dynamic": true},
				"source":       map[string]interface{}{"type": "keyword"},
				"content_id":   map[string]interface{}{"type": "text", "fields": keywordSub},
				"content_hash": map[string]interface{}{"type": "keyword"},
			},
		},
	}
}

// Create creates the k-NN index if it does not exist.
func (o *OpenSearch) Create(ctx context.Context) error {
	exists, err := o.Exists(ctx)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if o.Dimensions <= 0 {
		return errors.New("opensearch: vector dimensions unknown; set Config.Dimensions or an embedder")
	}
	err = o.doJSON(ctx, http.MethodPut, o.indexPath(), o.Mapping(), nil)
	var ae *apiError
	if errors.As(err, &ae) && strings.Contains(ae.Body, "resource_already_exists_exception") {
		return nil
	}
	return err
}

// Exists reports whether the index exists (HEAD /<index>).
func (o *OpenSearch) Exists(ctx context.Context) (bool, error) {
	status, data, err := o.do(ctx, http.MethodHead, o.indexPath(), nil, "")
	if err != nil {
		return false, err
	}
	switch status {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	}
	return false, &apiError{Status: status, Body: string(data)}
}

// Drop deletes the index (a missing index is not an error).
func (o *OpenSearch) Drop(ctx context.Context) error {
	status, data, err := o.do(ctx, http.MethodDelete, o.indexPath(), nil, "")
	if err != nil {
		return err
	}
	if status == http.StatusNotFound || (status >= 200 && status < 300) {
		return nil
	}
	return &apiError{Status: status, Body: string(data)}
}

// Optimize force-merges the index into a single segment.
func (o *OpenSearch) Optimize(ctx context.Context) error {
	return o.doJSON(ctx, http.MethodPost, o.indexPath()+"/_forcemerge?max_num_segments=1", nil, nil)
}

// GetCount returns the number of documents (0 if the index is missing).
func (o *OpenSearch) GetCount(ctx context.Context) (int64, error) {
	var out struct {
		Count int64 `json:"count"`
	}
	err := o.doJSON(ctx, http.MethodGet, o.indexPath()+"/_count", nil, &out)
	var ae *apiError
	if errors.As(err, &ae) && ae.Status == http.StatusNotFound {
		return 0, nil
	}
	return out.Count, err
}

// ---------------------------------------------------------------- writes

func (o *OpenSearch) buildIndexDoc(doc *document.Document, filters map[string]interface{}) map[string]interface{} {
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
	if v, ok := meta["content_id"]; ok && v != nil {
		out["content_id"] = v
	}
	if h, ok := meta["content_hash"]; ok && h != nil {
		out["content_hash"] = h
	}
	return out
}

func (o *OpenSearch) bulk(ctx context.Context, docs []*document.Document, filters map[string]interface{}, upsert bool) error {
	if len(docs) == 0 {
		return nil
	}
	if err := o.EmbedDocuments(docs); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		if len(doc.Embeddings) == 0 {
			return fmt.Errorf("opensearch: document %q has no embedding (configure an embedder)", doc.ID)
		}
		if o.Dimensions > 0 && len(doc.Embeddings) != o.Dimensions {
			return fmt.Errorf("opensearch: document %q embedding has %d dims, index expects %d", doc.ID, len(doc.Embeddings), o.Dimensions)
		}
		id := elasticsearch.DocumentID(doc)
		if doc.ID == "" {
			doc.ID = id
		}
		src := o.buildIndexDoc(doc, filters)
		action, payload := "index", interface{}(src)
		if upsert {
			action = "update"
			payload = map[string]interface{}{"doc": src, "doc_as_upsert": true}
		}
		if err := enc.Encode(map[string]interface{}{action: map[string]interface{}{"_index": o.cfg.IndexName, "_id": id}}); err != nil {
			return err
		}
		if err := enc.Encode(payload); err != nil {
			return err
		}
	}
	status, data, err := o.do(ctx, http.MethodPost, "/_bulk?refresh=true", buf.Bytes(), "application/x-ndjson")
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
		return fmt.Errorf("opensearch: decode bulk response: %w", err)
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
	return fmt.Errorf("opensearch: bulk write failed for %d/%d documents: %s", failed, len(resp.Items), strings.Join(samples, "; "))
}

// Insert indexes documents (bulk "index" action). Filters are merged into meta_data.
func (o *OpenSearch) Insert(ctx context.Context, docs []*document.Document, filters map[string]interface{}) error {
	return o.bulk(ctx, docs, filters, false)
}

// Upsert writes documents with bulk "update" + doc_as_upsert.
func (o *OpenSearch) Upsert(ctx context.Context, docs []*document.Document, filters map[string]interface{}) error {
	return o.bulk(ctx, docs, filters, true)
}

// IDExists reports whether a document with this _id exists.
func (o *OpenSearch) IDExists(ctx context.Context, id string) (bool, error) {
	if id == "" {
		return false, nil
	}
	status, data, err := o.do(ctx, http.MethodHead, o.indexPath()+"/_doc/"+url.PathEscape(id), nil, "")
	if err != nil {
		return false, err
	}
	switch status {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	}
	return false, &apiError{Status: status, Body: string(data)}
}

// DocExists checks existence by the document's _id (its ID or content MD5).
func (o *OpenSearch) DocExists(ctx context.Context, doc *document.Document) (bool, error) {
	if doc == nil {
		return false, nil
	}
	return o.IDExists(ctx, elasticsearch.DocumentID(doc))
}

// NameExists reports whether any document has this exact name.
func (o *OpenSearch) NameExists(ctx context.Context, name string) (bool, error) {
	return o.fieldExists(ctx, "name.keyword", name)
}

// ContentHashExists reports whether any document carries this content hash.
func (o *OpenSearch) ContentHashExists(ctx context.Context, hash string) (bool, error) {
	return o.fieldExists(ctx, "content_hash", hash)
}

func (o *OpenSearch) fieldExists(ctx context.Context, field, value string) (bool, error) {
	var out searchResponse
	body := map[string]interface{}{"size": 1, "query": map[string]interface{}{"term": map[string]interface{}{field: value}}}
	err := o.doJSON(ctx, http.MethodPost, o.indexPath()+"/_search", body, &out)
	var ae *apiError
	if errors.As(err, &ae) && ae.Status == http.StatusNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return out.Hits.Total.Value > 0 || len(out.Hits.Hits) > 0, nil
}

// DeleteByName removes every document with this exact name.
func (o *OpenSearch) DeleteByName(ctx context.Context, name string) error {
	body := map[string]interface{}{"query": map[string]interface{}{"term": map[string]interface{}{"name.keyword": name}}}
	return o.doJSON(ctx, http.MethodPost, o.indexPath()+"/_delete_by_query?refresh=true&conflicts=proceed", body, nil)
}

// ---------------------------------------------------------------- search

// Search dispatches on the configured search type.
func (o *OpenSearch) Search(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	switch o.SearchType {
	case vectordb.SearchTypeKeyword:
		return o.KeywordSearch(ctx, query, limit, filters)
	case vectordb.SearchTypeHybrid:
		return o.HybridSearch(ctx, query, limit, filters)
	default:
		return o.VectorSearch(ctx, query, limit, filters)
	}
}

func (o *OpenSearch) queryVector(query string) ([]float64, error) {
	if o.Embedder == nil {
		return nil, errors.New("opensearch: an embedder is required for vector search")
	}
	vec, err := o.EmbedQuery(query)
	if err != nil {
		return nil, err
	}
	if len(vec) == 0 {
		return nil, errors.New("opensearch: empty query embedding")
	}
	return vec, nil
}

func normLimit(limit int) int {
	if limit <= 0 {
		return 5
	}
	return limit
}

// VectorSearch runs a knn query. Filters are pushed inside the knn clause
// (efficient filtering) when the engine supports it, otherwise applied as a
// bool post-filter.
func (o *OpenSearch) VectorSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	limit = normLimit(limit)
	vec, err := o.queryVector(query)
	if err != nil {
		return nil, err
	}
	knn := map[string]interface{}{"vector": vec, "k": limit}
	conds := elasticsearch.BuildFilterConditions(filters)
	body := map[string]interface{}{"size": limit}
	switch {
	case len(conds) == 0:
		body["query"] = map[string]interface{}{"knn": map[string]interface{}{"embedding": knn}}
	case o.SupportsKNNFilter():
		knn["filter"] = map[string]interface{}{"bool": map[string]interface{}{"filter": conds}}
		body["query"] = map[string]interface{}{"knn": map[string]interface{}{"embedding": knn}}
	default:
		body["query"] = map[string]interface{}{"bool": map[string]interface{}{
			"must":   []interface{}{map[string]interface{}{"knn": map[string]interface{}{"embedding": knn}}},
			"filter": conds,
		}}
	}
	return o.runSearch(ctx, body)
}

func multiMatch(query string, boost float64) map[string]interface{} {
	mm := map[string]interface{}{"query": query, "fields": []string{"content", "name"}, "type": "best_fields"}
	if boost > 0 {
		mm["boost"] = boost
	}
	return map[string]interface{}{"multi_match": mm}
}

// KeywordSearch runs a multi_match over content and name.
func (o *OpenSearch) KeywordSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	limit = normLimit(limit)
	q := multiMatch(query, 0)
	body := map[string]interface{}{"size": limit, "query": q}
	if conds := elasticsearch.BuildFilterConditions(filters); len(conds) > 0 {
		body["query"] = map[string]interface{}{"bool": map[string]interface{}{"must": []interface{}{q}, "filter": conds}}
	}
	return o.runSearch(ctx, body)
}

// HybridSearch combines knn (boost 0.7) and multi_match (boost 0.3) in a bool
// should; filters scope both halves.
func (o *OpenSearch) HybridSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	limit = normLimit(limit)
	vec, err := o.queryVector(query)
	if err != nil {
		return nil, err
	}
	knn := map[string]interface{}{"vector": vec, "k": limit, "boost": vectorBoost}
	conds := elasticsearch.BuildFilterConditions(filters)
	if len(conds) > 0 && o.SupportsKNNFilter() {
		knn["filter"] = map[string]interface{}{"bool": map[string]interface{}{"filter": conds}}
	}
	boolQ := map[string]interface{}{
		"should": []interface{}{
			map[string]interface{}{"knn": map[string]interface{}{"embedding": knn}},
			multiMatch(query, keywordBoost),
		},
	}
	if len(conds) > 0 {
		boolQ["filter"] = conds
	}
	return o.runSearch(ctx, map[string]interface{}{"size": limit, "query": map[string]interface{}{"bool": boolQ}})
}

type searchResponse struct {
	Hits struct {
		Total struct {
			Value int64 `json:"value"`
		} `json:"total"`
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

func (o *OpenSearch) runSearch(ctx context.Context, body map[string]interface{}) ([]*vectordb.SearchResult, error) {
	var out searchResponse
	err := o.doJSON(ctx, http.MethodPost, o.indexPath()+"/_search", body, &out)
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
