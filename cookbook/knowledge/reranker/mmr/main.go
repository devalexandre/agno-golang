// MMR (Maximal Marginal Relevance) reranker applied directly to search results.
//
// MMR picks results that are relevant to the query but unlike each other, so
// near-duplicates do not crowd out the rest. Lambda weights relevance (1.0)
// against diversity (0.0). It needs an embedding on every result
// (Document.Embeddings) and an embedder for the query: MMRReranker.Embedder,
// or, when used through Knowledge, the vector db's embedder.
//
// Runs offline: a tiny in-file embedder maps texts to hand-made 3-d vectors.
// With a real store set kb.Reranker = reranker.NewMMRReranker(nil, 0.5, 0)
// (see cookbook/knowledge/rerank_hyde).
//
//	go run ./cookbook/knowledge/reranker/mmr
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/knowledge/reranker"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

// Axes: [goroutines, channels, history].
var vectors = map[string][]float64{
	"How does Go handle concurrency?":                                 {1, 0.6, 0},
	"Goroutines are lightweight threads managed by the Go runtime.":   {1, 0.1, 0},
	"Goroutines are cheap: thousands run on a handful of OS threads.": {0.98, 0.12, 0},
	"A goroutine starts with a small stack that grows on demand.":     {0.95, 0.1, 0.05},
	"Channels let goroutines communicate by sending typed values.":    {0.4, 1, 0},
	"Go was designed at Google in 2007 and released in 2009.":         {0.05, 0, 1},
}

func main() {
	ctx := context.Background()
	query := "How does Go handle concurrency?"
	emb := &fakeEmbedder{}

	// Candidates as a vector db would return them (by relevance, with embeddings).
	var results []*vectordb.SearchResult
	for _, text := range []string{
		"Goroutines are lightweight threads managed by the Go runtime.",
		"Goroutines are cheap: thousands run on a handful of OS threads.",
		"A goroutine starts with a small stack that grows on demand.",
		"Channels let goroutines communicate by sending typed values.",
		"Go was designed at Google in 2007 and released in 2009.",
	} {
		results = append(results, &vectordb.SearchResult{
			Document: &document.Document{Content: text, Embeddings: vectors[text]},
		})
	}

	for _, lambda := range []float64{1.0, 0.5} {
		mmr := reranker.NewMMRReranker(emb, lambda, 0)
		top, err := mmr.Rerank(ctx, query, results, 3)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("lambda=%.1f (top 3):\n", lambda)
		for i, r := range top {
			fmt.Printf("  %d. (%.3f) %s\n", i+1, r.Score, r.Document.Content)
		}
	}
	// With lambda=1 the three goroutine near-duplicates win; with 0.5 the
	// channels and history documents replace two of them. MMR scores are not
	// monotonic: the returned order is what counts.

	// The candidate pool MMR asks the store for when used through Knowledge.
	fmt.Println("search limit for 3 results:", reranker.NewMMRReranker(nil, 0.5, 0).SearchLimit(3))
}

// fakeEmbedder returns the hand-made vector of a known text.
type fakeEmbedder struct{}

func (fakeEmbedder) GetEmbedding(text string) ([]float64, error) {
	if v, ok := vectors[text]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("no vector for %q", text)
}
func (e fakeEmbedder) GetEmbeddingAndUsage(text string) ([]float64, map[string]interface{}, error) {
	v, err := e.GetEmbedding(text)
	return v, nil, err
}
func (fakeEmbedder) GetDimensions() int { return 3 }
func (fakeEmbedder) GetID() string      { return "fake" }
