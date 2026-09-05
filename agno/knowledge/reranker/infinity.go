package reranker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/devalexandre/agno-golang/agno/vectordb"
)

// InfinityReranker reorders results using a self-hosted Infinity reranking
// server (https://github.com/michaelfeil/infinity), which exposes an
// OpenAI-style /rerank endpoint.
type InfinityReranker struct {
	BaseReranker
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewInfinityReranker creates an Infinity reranker. baseURL defaults to
// INFINITY_BASE_URL then http://localhost:7997.
func NewInfinityReranker(baseURL, model string, topN int) *InfinityReranker {
	if baseURL == "" {
		baseURL = os.Getenv("INFINITY_BASE_URL")
	}
	if baseURL == "" {
		baseURL = "http://localhost:7997"
	}
	if model == "" {
		model = "mixedbread-ai/mxbai-rerank-large-v1"
	}
	return &InfinityReranker{
		BaseReranker: BaseReranker{ID: model, Model: model, TopN: topN},
		BaseURL:      baseURL,
		APIKey:       os.Getenv("INFINITY_API_KEY"),
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

type infinityRerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n,omitempty"`
}

type infinityRerankResponse struct {
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
	} `json:"results"`
}

// Rerank implements Reranker.
func (r *InfinityReranker) Rerank(ctx context.Context, query string, results []*vectordb.SearchResult, topN int) ([]*vectordb.SearchResult, error) {
	if len(results) == 0 {
		return results, nil
	}
	if topN <= 0 {
		topN = r.TopN
	}

	docs := make([]string, len(results))
	for i, res := range results {
		if res.Document != nil {
			docs[i] = res.Document.Content
		}
	}

	reqBody := infinityRerankRequest{Model: r.Model, Query: query, Documents: docs}
	if topN > 0 {
		reqBody.TopN = topN
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", r.BaseURL+"/rerank", bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if r.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+r.APIKey)
	}

	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("infinity reranker: status %d: %s", resp.StatusCode, string(body))
	}

	var out infinityRerankResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}

	reranked := make([]*vectordb.SearchResult, 0, len(out.Results))
	for _, item := range out.Results {
		if item.Index < 0 || item.Index >= len(results) {
			continue
		}
		res := results[item.Index]
		res.Score = item.RelevanceScore
		reranked = append(reranked, res)
	}
	sort.SliceStable(reranked, func(i, j int) bool { return reranked[i].Score > reranked[j].Score })
	return reranked, nil
}

var _ Reranker = (*InfinityReranker)(nil)
