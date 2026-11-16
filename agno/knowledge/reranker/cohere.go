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

// CohereReranker reorders search results using the Cohere Rerank API.
type CohereReranker struct {
	BaseReranker
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

// NewCohereReranker creates a Cohere reranker. apiKey falls back to COHERE_API_KEY.
// Default model is rerank-english-v3.0.
func NewCohereReranker(apiKey, model string, topN int) *CohereReranker {
	if apiKey == "" {
		apiKey = os.Getenv("COHERE_API_KEY")
	}
	if model == "" {
		model = "rerank-english-v3.0"
	}
	return &CohereReranker{
		BaseReranker: BaseReranker{ID: model, Model: model, TopN: topN},
		APIKey:       apiKey,
		BaseURL:      "https://api.cohere.ai/v1",
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

type cohereRerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n,omitempty"`
}

type cohereRerankResponse struct {
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
	} `json:"results"`
}

// Rerank implements Reranker.
func (r *CohereReranker) Rerank(ctx context.Context, query string, results []*vectordb.SearchResult, limit int) ([]*vectordb.SearchResult, error) {
	if len(results) == 0 {
		return results, nil
	}
	if r.APIKey == "" {
		return nil, fmt.Errorf("cohere reranker: API key is required")
	}
	topN := effectiveLimit(limit, r.TopN)

	docs := make([]string, len(results))
	for i, res := range results {
		if res.Document != nil {
			docs[i] = res.Document.Content
		}
	}

	reqBody := cohereRerankRequest{Model: r.Model, Query: query, Documents: docs}
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
	req.Header.Set("Authorization", "Bearer "+r.APIKey)

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
		return nil, fmt.Errorf("cohere reranker: status %d: %s", resp.StatusCode, string(body))
	}

	var out cohereRerankResponse
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
	// Cohere returns sorted by relevance, but ensure ordering defensively.
	sort.SliceStable(reranked, func(i, j int) bool { return reranked[i].Score > reranked[j].Score })
	return reranked, nil
}

var _ Reranker = (*CohereReranker)(nil)
