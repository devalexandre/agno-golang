// Package querytransformer rewrites a search query before it reaches the
// vector db. Port of agno-python knowledge/query_transformer.
//
// A reranker reorders results after the search; a query transformer changes
// what is searched for. Set one on BaseKnowledge.QueryTransformer or
// RAGPipeline.QueryTransformer. Reranking still scores against the question
// the caller asked, never the transformed query.
package querytransformer

import "context"

// QueryTransformer returns the query to search with. Returning query unchanged
// is valid. The knowledge pipeline treats an error as "search with the
// original query", so a failing transform degrades search instead of breaking
// it.
type QueryTransformer interface {
	Transform(ctx context.Context, query string) (string, error)
}

// Func adapts a plain function to QueryTransformer.
type Func func(ctx context.Context, query string) (string, error)

// Transform implements QueryTransformer.
func (f Func) Transform(ctx context.Context, query string) (string, error) {
	return f(ctx, query)
}
