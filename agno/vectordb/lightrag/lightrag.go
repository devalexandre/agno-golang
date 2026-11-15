// Package lightrag adapts a LightRAG server (https://github.com/HKUDS/LightRAG)
// to the vectordb.VectorDB interface, mirroring Agno Python's
// agno.vectordb.lightrag.LightRag.
//
// LightRAG owns chunking, embedding and its knowledge graph, so this adapter
// is a thin HTTP client: Insert/Upsert send document text to /documents/text,
// Search posts to /query (mode "hybrid" by default) and returns the generated
// answer as a single result, Drop clears every document and the LLM cache.
// Per-document existence checks and counting are not exposed by the server and
// report false / -1 like the Python adapter.
package lightrag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

// DefaultServerURL is the LightRAG server default address.
const DefaultServerURL = "http://localhost:9621"

// Query modes supported by LightRAG's /query endpoint.
const (
	ModeLocal  = "local"
	ModeGlobal = "global"
	ModeHybrid = "hybrid"
	ModeNaive  = "naive"
	ModeMix    = "mix"
)

// Config configures a LightRAG client.
type Config struct {
	// ServerURL defaults to LIGHTRAG_SERVER_URL, then http://localhost:9621.
	ServerURL string
	// APIKey defaults to LIGHTRAG_API_KEY.
	APIKey string
	// AuthHeaderName (default "X-API-KEY") and AuthHeaderFormat (default
	// "{api_key}") control how APIKey is sent, e.g. "Authorization" /
	// "Bearer {api_key}".
	AuthHeaderName   string
	AuthHeaderFormat string
	// Mode is the /query mode for Search (default hybrid).
	Mode       string
	HTTPClient *http.Client
}

// LightRAG implements vectordb.VectorDB over the LightRAG server API.
type LightRAG struct {
	cfg    Config
	client *http.Client
}

var _ vectordb.VectorDB = (*LightRAG)(nil)

// New creates a LightRAG client.
func New(cfg Config) *LightRAG {
	if cfg.ServerURL == "" {
		cfg.ServerURL = os.Getenv("LIGHTRAG_SERVER_URL")
	}
	if cfg.ServerURL == "" {
		cfg.ServerURL = DefaultServerURL
	}
	cfg.ServerURL = strings.TrimRight(cfg.ServerURL, "/")
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("LIGHTRAG_API_KEY")
	}
	if cfg.AuthHeaderName == "" {
		cfg.AuthHeaderName = "X-API-KEY"
	}
	if cfg.AuthHeaderFormat == "" {
		cfg.AuthHeaderFormat = "{api_key}"
	}
	if cfg.Mode == "" {
		cfg.Mode = ModeHybrid
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	return &LightRAG{cfg: cfg, client: client}
}

func (l *LightRAG) do(ctx context.Context, method, path string, body interface{}, out interface{}) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, l.cfg.ServerURL+path, rdr)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if l.cfg.APIKey != "" {
		req.Header.Set(l.cfg.AuthHeaderName, strings.ReplaceAll(l.cfg.AuthHeaderFormat, "{api_key}", l.cfg.APIKey))
	}
	resp, err := l.client.Do(req)
	if err != nil {
		return fmt.Errorf("lightrag: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("lightrag: %s %s: status %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

// Create is a no-op: the LightRAG server manages its own storage.
func (l *LightRAG) Create(context.Context) error { return nil }

// Exists reports false, like the Python adapter (the server has no notion of a collection).
func (l *LightRAG) Exists(context.Context) (bool, error) { return false, nil }

// Drop deletes every document and clears the query cache.
func (l *LightRAG) Drop(ctx context.Context) error {
	if err := l.do(ctx, http.MethodDelete, "/documents", nil, nil); err != nil {
		return err
	}
	return l.do(ctx, http.MethodPost, "/documents/clear_cache", map[string]interface{}{"modes": []string{"default", "naive"}}, nil)
}

// Optimize is a no-op.
func (l *LightRAG) Optimize(context.Context) error { return nil }

type insertResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	TrackID string `json:"track_id"`
}

// InsertText sends one text to /documents/text and returns the server track_id.
func (l *LightRAG) InsertText(ctx context.Context, fileSource, text string) (string, error) {
	body := map[string]interface{}{"text": text}
	if fileSource != "" {
		body["file_source"] = fileSource
	}
	var out insertResponse
	if err := l.do(ctx, http.MethodPost, "/documents/text", body, &out); err != nil {
		return "", err
	}
	return out.TrackID, nil
}

// DocumentIDForTrack resolves the server document ID for a track_id
// (GET /documents/track_status/{id}); "" if the server has not created it yet.
func (l *LightRAG) DocumentIDForTrack(ctx context.Context, trackID string) (string, error) {
	var out struct {
		Documents []struct {
			ID string `json:"id"`
		} `json:"documents"`
	}
	if err := l.do(ctx, http.MethodGet, "/documents/track_status/"+trackID, nil, &out); err != nil {
		return "", err
	}
	if len(out.Documents) == 0 {
		return "", nil
	}
	return out.Documents[0].ID, nil
}

// DeleteByExternalID deletes a server document by its LightRAG document ID.
func (l *LightRAG) DeleteByExternalID(ctx context.Context, id string) error {
	return l.do(ctx, http.MethodDelete, "/documents/delete_document",
		map[string]interface{}{"doc_ids": []string{id}, "delete_file": false}, nil)
}

// Insert sends each document's content to the server. The document name (or
// source) is used as file_source so LightRAG can cite it.
func (l *LightRAG) Insert(ctx context.Context, docs []*document.Document, _ map[string]interface{}) error {
	for _, d := range docs {
		if d == nil || strings.TrimSpace(d.Content) == "" {
			continue
		}
		src := d.Name
		if src == "" {
			src = d.Source
		}
		trackID, err := l.InsertText(ctx, src, d.Content)
		if err != nil {
			return err
		}
		if trackID != "" {
			if d.Metadata == nil {
				d.Metadata = map[string]interface{}{}
			}
			d.Metadata["lightrag_track_id"] = trackID
		}
	}
	return nil
}

// Upsert is Insert: LightRAG deduplicates content server-side.
func (l *LightRAG) Upsert(ctx context.Context, docs []*document.Document, filters map[string]interface{}) error {
	return l.Insert(ctx, docs, filters)
}

// Query posts to /query with the given mode and converts the answer to results.
func (l *LightRAG) Query(ctx context.Context, query, mode string) ([]*vectordb.SearchResult, error) {
	if mode == "" {
		mode = l.cfg.Mode
	}
	var raw json.RawMessage
	if err := l.do(ctx, http.MethodPost, "/query",
		map[string]interface{}{"query": query, "mode": mode, "include_references": true}, &raw); err != nil {
		return nil, err
	}
	return formatResponse(raw, query, mode), nil
}

func formatResponse(raw json.RawMessage, query, mode string) []*vectordb.SearchResult {
	baseMeta := func() map[string]interface{} {
		return map[string]interface{}{"source": "lightrag", "query": query, "mode": mode}
	}
	wrap := func(content string, meta map[string]interface{}) *vectordb.SearchResult {
		return &vectordb.SearchResult{Document: &document.Document{Content: content, Source: "lightrag", Metadata: meta}, Score: 1}
	}
	var obj map[string]interface{}
	if json.Unmarshal(raw, &obj) == nil {
		if resp, ok := obj["response"]; ok {
			meta := baseMeta()
			if refs, ok := obj["references"]; ok {
				meta["references"] = refs
			}
			return []*vectordb.SearchResult{wrap(fmt.Sprint(resp), meta)}
		}
	}
	var list []interface{}
	if json.Unmarshal(raw, &list) == nil {
		out := make([]*vectordb.SearchResult, 0, len(list))
		for _, item := range list {
			if m, ok := item.(map[string]interface{}); ok {
				if c, ok := m["content"]; ok {
					meta, ok := m["metadata"].(map[string]interface{})
					if !ok {
						meta = baseMeta()
					}
					out = append(out, wrap(fmt.Sprint(c), meta))
					continue
				}
			}
			out = append(out, wrap(fmt.Sprint(item), baseMeta()))
		}
		return out
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		s = string(raw)
	}
	return []*vectordb.SearchResult{wrap(s, baseMeta())}
}

// Search queries the server with the configured mode. Filters are not
// supported by LightRAG and are ignored; limit is applied to list responses.
func (l *LightRAG) Search(ctx context.Context, query string, limit int, _ map[string]interface{}) ([]*vectordb.SearchResult, error) {
	res, err := l.Query(ctx, query, "")
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(res) > limit {
		res = res[:limit]
	}
	return res, nil
}

// VectorSearch uses the "naive" (pure vector retrieval) mode.
func (l *LightRAG) VectorSearch(ctx context.Context, query string, limit int, _ map[string]interface{}) ([]*vectordb.SearchResult, error) {
	res, err := l.Query(ctx, query, ModeNaive)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(res) > limit {
		res = res[:limit]
	}
	return res, nil
}

// KeywordSearch uses the "local" (entity/keyword driven) mode.
func (l *LightRAG) KeywordSearch(ctx context.Context, query string, limit int, _ map[string]interface{}) ([]*vectordb.SearchResult, error) {
	res, err := l.Query(ctx, query, ModeLocal)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(res) > limit {
		res = res[:limit]
	}
	return res, nil
}

// HybridSearch uses the "hybrid" mode.
func (l *LightRAG) HybridSearch(ctx context.Context, query string, limit int, _ map[string]interface{}) ([]*vectordb.SearchResult, error) {
	res, err := l.Query(ctx, query, ModeHybrid)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(res) > limit {
		res = res[:limit]
	}
	return res, nil
}

// GetCount is not exposed by LightRAG; it returns -1.
func (l *LightRAG) GetCount(context.Context) (int64, error) { return -1, nil }

// GetEmbedder returns nil: embeddings are computed by the server.
func (l *LightRAG) GetEmbedder() embedder.Embedder { return nil }

// DocExists always reports false (not exposed by the server).
func (l *LightRAG) DocExists(context.Context, *document.Document) (bool, error) { return false, nil }

// NameExists always reports false (not exposed by the server).
func (l *LightRAG) NameExists(context.Context, string) (bool, error) { return false, nil }

// IDExists always reports false (not exposed by the server).
func (l *LightRAG) IDExists(context.Context, string) (bool, error) { return false, nil }
