// Package reranker is the legacy location of the reranker interface.
//
// Deprecated: use github.com/devalexandre/agno-golang/agno/knowledge/reranker,
// the canonical package (Rerank with a limit, candidate widening via
// SearchLimit, Cohere/Infinity/Lexical/MMR/Recency implementations). This
// package only remains as a thin adapter so existing code keeps compiling.
package reranker

import (
	"context"

	"github.com/devalexandre/agno-golang/agno/knowledge"
	kreranker "github.com/devalexandre/agno-golang/agno/knowledge/reranker"
)

// Reranker is the interface for document reranking models.
//
// Deprecated: use knowledge/reranker.Reranker. This is an alias of the legacy
// knowledge.Reranker interface.
type Reranker = knowledge.Reranker

// ToCanonical wraps a legacy Reranker as a canonical knowledge/reranker.Reranker.
//
// Deprecated: implement knowledge/reranker.Reranker directly.
func ToCanonical(r Reranker) kreranker.Reranker {
	return knowledge.AdaptLegacyReranker(r)
}

// FromCanonical exposes a canonical reranker through the legacy interface
// (no limit: all reranked results are returned).
//
// Deprecated: use the canonical reranker directly.
func FromCanonical(r kreranker.Reranker) Reranker {
	if r == nil {
		return nil
	}
	return canonicalAdapter{r}
}

type canonicalAdapter struct{ inner kreranker.Reranker }

func (c canonicalAdapter) Rerank(ctx context.Context, query string, results []*knowledge.SearchResult) ([]*knowledge.SearchResult, error) {
	return c.inner.Rerank(ctx, query, results, 0)
}

func (c canonicalAdapter) GetID() string { return c.inner.GetID() }

// Canonical returns the wrapped canonical reranker, letting
// knowledge.AdaptLegacyReranker unwrap it instead of double-wrapping.
func (c canonicalAdapter) Canonical() kreranker.Reranker { return c.inner }
