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
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/vectordb"
)

// SentenceTransformerReranker is the Go counterpart of Agno Python's
// SentenceTransformerReranker (a sentence-transformers CrossEncoder). Go cannot
// run the model in-process, so it calls a Hugging Face Text Embeddings
// Inference (TEI) server hosting the same cross-encoder (POST {BaseURL}/rerank):
//
//	docker run -p 8081:80 ghcr.io/huggingface/text-embeddings-inference:cpu-1.5 \
//	    --model-id BAAI/bge-reranker-v2-m3
type SentenceTransformerReranker struct {
	BaseReranker
	// BaseURL defaults to SENTENCE_TRANSFORMERS_RERANK_URL, then
	// TEI_RERANK_URL, then http://localhost:8081.
	BaseURL string
	// APIKey is sent as a Bearer token when set (defaults to TEI_API_KEY).
	APIKey string
	// RawScores returns un-normalised logits instead of sigmoid scores.
	RawScores  bool
	Truncate   bool
	HTTPClient *http.Client
}

// NewSentenceTransformerReranker creates the reranker. model defaults to
// BAAI/bge-reranker-v2-m3 and only labels the reranker (TEI serves one model).
func NewSentenceTransformerReranker(baseURL, model string, topN int) *SentenceTransformerReranker {
	for _, v := range []string{baseURL, os.Getenv("SENTENCE_TRANSFORMERS_RERANK_URL"), os.Getenv("TEI_RERANK_URL"), "http://localhost:8081"} {
		if v != "" {
			baseURL = v
			break
		}
	}
	if model == "" {
		model = "BAAI/bge-reranker-v2-m3"
	}
	return &SentenceTransformerReranker{
		BaseReranker: BaseReranker{ID: model, Model: model, TopN: topN},
		BaseURL:      strings.TrimRight(baseURL, "/"),
		APIKey:       os.Getenv("TEI_API_KEY"),
		Truncate:     true,
		HTTPClient:   &http.Client{Timeout: 60 * time.Second},
	}
}

type teiRerankRequest struct {
	Query      string   `json:"query"`
	Texts      []string `json:"texts"`
	RawScores  bool     `json:"raw_scores"`
	ReturnText bool     `json:"return_text"`
	Truncate   bool     `json:"truncate"`
}

type teiRerankItem struct {
	Index int     `json:"index"`
	Score float64 `json:"score"`
}

// Rerank implements Reranker: every result is scored by the cross-encoder,
// sorted by score and cut to the effective limit (min of limit and TopN).
func (r *SentenceTransformerReranker) Rerank(ctx context.Context, query string, results []*vectordb.SearchResult, limit int) ([]*vectordb.SearchResult, error) {
	if len(results) == 0 {
		return results, nil
	}
	texts := make([]string, len(results))
	for i, res := range results {
		if res != nil && res.Document != nil {
			texts[i] = res.Document.Content
		}
	}
	payload, err := json.Marshal(teiRerankRequest{Query: query, Texts: texts, RawScores: r.RawScores, Truncate: r.Truncate})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.BaseURL+"/rerank", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if r.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+r.APIKey)
	}
	client := r.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sentence-transformer reranker: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var items []teiRerankItem
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("sentence-transformer reranker: decode response: %w", err)
	}
	reranked := make([]*vectordb.SearchResult, 0, len(items))
	for _, it := range items {
		if it.Index < 0 || it.Index >= len(results) || results[it.Index] == nil {
			continue
		}
		c := copyResult(results[it.Index])
		c.Score = it.Score
		reranked = append(reranked, c)
	}
	sort.SliceStable(reranked, func(i, j int) bool { return reranked[i].Score > reranked[j].Score })
	if n := effectiveLimit(limit, r.TopN); n > 0 && len(reranked) > n {
		reranked = reranked[:n]
	}
	return reranked, nil
}

var _ Reranker = (*SentenceTransformerReranker)(nil)
