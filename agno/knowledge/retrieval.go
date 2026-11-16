package knowledge

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/knowledge/querytransformer"
	"github.com/devalexandre/agno-golang/agno/knowledge/reranker"
)

// searchFunc fetches limit candidates for query from the vector db.
type searchFunc func(ctx context.Context, query string, limit int) ([]*SearchResult, error)

// retrieve is the knowledge-level retrieval pipeline, mirroring Python
// Knowledge.search:
//
//  1. optional QueryTransformer rewrites the query (a failure keeps the original);
//  2. the vector db returns reranker.SearchLimit(maxResults) candidates
//     (exactly maxResults without a reranker: the old behaviour);
//  3. the reranker reorders them against the ORIGINAL query and the result is
//     trimmed to maxResults. A reranker error wrapping reranker.ErrInvalidInput
//     (misconfiguration) is returned; any other reranker error falls back to
//     the vector db order.
func retrieve(ctx context.Context, search searchFunc, query string, maxResults int,
	rr reranker.Reranker, qt querytransformer.QueryTransformer, emb embedder.Embedder) ([]*SearchResult, error) {

	searchQuery := query
	if qt != nil {
		transformed, err := qt.Transform(ctx, query)
		if err != nil {
			log.Printf("[KNOWLEDGE] Error transforming query, using it as asked: %v", err)
		} else if transformed != "" {
			searchQuery = transformed
		}
	}

	if rr == nil {
		return search(ctx, searchQuery, maxResults)
	}

	results, err := search(ctx, searchQuery, reranker.SearchLimit(rr, maxResults))
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return results, nil
	}

	reranked, err := rr.Rerank(reranker.ContextWithEmbedder(ctx, emb), query, results, maxResults)
	if err != nil {
		if errors.Is(err, reranker.ErrInvalidInput) {
			// A misconfigured reranker would otherwise look like it ran and changed nothing.
			return nil, fmt.Errorf("failed to rerank documents: %w", err)
		}
		// A reranker failure degrades ordering, not availability.
		log.Printf("[KNOWLEDGE] Error reranking documents, keeping vector db order: %v", err)
		return trimResults(results, maxResults), nil
	}
	return trimResults(reranked, maxResults), nil
}

func trimResults(results []*SearchResult, maxResults int) []*SearchResult {
	if maxResults > 0 && len(results) > maxResults {
		return results[:maxResults]
	}
	return results
}

// AdaptLegacyReranker wraps a legacy knowledge.Reranker (Rerank without a
// limit) as a canonical reranker.Reranker. It uses the default candidate
// widening (x3, capped at 100).
func AdaptLegacyReranker(r Reranker) reranker.Reranker {
	if r == nil {
		return nil
	}
	if u, ok := r.(interface{ Canonical() reranker.Reranker }); ok {
		return u.Canonical()
	}
	return &legacyReranker{inner: r}
}

type legacyReranker struct {
	reranker.BaseReranker
	inner Reranker
}

func (l *legacyReranker) Rerank(ctx context.Context, query string, results []*SearchResult, limit int) ([]*SearchResult, error) {
	out, err := l.inner.Rerank(ctx, query, results)
	if err != nil {
		return nil, err
	}
	return trimResults(out, limit), nil
}

func (l *legacyReranker) GetID() string {
	if id, ok := l.inner.(interface{ GetID() string }); ok {
		return id.GetID()
	}
	return "legacy"
}

// retrieve runs the retrieval pipeline over k.VectorDB with the given filters.
func (k *BaseKnowledge) retrieve(ctx context.Context, query string, maxResults int, filters map[string]interface{}) ([]*SearchResult, error) {
	search := func(ctx context.Context, q string, limit int) ([]*SearchResult, error) {
		return k.VectorDB.Search(ctx, q, limit, filters)
	}
	var emb embedder.Embedder
	if k.Reranker != nil && k.VectorDB != nil {
		// Prefer the vector db's embedder: it is the model the documents were indexed with.
		emb = k.VectorDB.GetEmbedder()
	}
	if emb == nil && k.Reranker != nil {
		emb = k.Embedder
	}
	return retrieve(ctx, search, query, maxResults, k.Reranker, k.QueryTransformer, emb)
}
