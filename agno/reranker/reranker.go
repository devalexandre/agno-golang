package reranker

import (
	"context"

	"github.com/devalexandre/agno-golang/agno/knowledge"
)

// Reranker is the interface for document reranking models.
type Reranker interface {
	// Rerank reranks a list of search results based on a query.
	Rerank(ctx context.Context, query string, results []*knowledge.SearchResult) ([]*knowledge.SearchResult, error)
}
