// Package reranker define a interface canônica para rerankers de resultados
// de busca. Espelha o contrato de agno-python/knowledge/reranker/base.py.
//
// Um Reranker recebe os resultados de uma busca no VectorDB e os reordena
// por relevância (ex.: Cohere Rerank, cross-encoder/Infinity, MMR, recência),
// melhorando a precisão do top-N do RAG.
//
// Rerankers devem ser configurados no nível do Knowledge (BaseKnowledge.Reranker
// ou RAGPipeline.RetrievalReranker): o Knowledge amplia a busca para
// SearchLimit(maxResults) candidatos, reordena com o reranker e corta para
// maxResults.
package reranker

import (
	"context"
	"errors"

	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

const (
	// DefaultCandidateMultiplier is the number of candidates fetched per
	// requested result when a reranker does not choose its own.
	DefaultCandidateMultiplier = 3
	// DefaultMaxCandidates caps the widened fetch so a large request cannot
	// turn one search into an unbounded scan.
	DefaultMaxCandidates = 100
)

// ErrInvalidInput marks a reranker failure caused by misconfiguration or by
// search results the reranker cannot work with (the Python ValueError case).
// The knowledge retrieval pipeline propagates these errors instead of silently
// falling back to the vector db order; any other reranker error degrades to the
// unreranked results.
var ErrInvalidInput = errors.New("reranker: invalid input")

// Reranker é o contrato canônico para reordenação de resultados de busca.
type Reranker interface {
	// Rerank reordena os resultados pela relevância em relação à query.
	// limit is the number of results the caller keeps: rerankers that select
	// a subset may stop early, scoring rerankers may ignore it. limit <= 0
	// means "no caller limit" (the reranker's own TopN, if any, still applies).
	Rerank(ctx context.Context, query string, results []*vectordb.SearchResult, limit int) ([]*vectordb.SearchResult, error)

	// GetID retorna o identificador do reranker (ex.: "cohere-rerank-v3").
	GetID() string
}

// SearchLimiter is implemented by rerankers that decide how many candidates
// the vector db should return for them. BaseReranker implements it, so every
// reranker embedding BaseReranker does too.
type SearchLimiter interface {
	SearchLimit(maxResults int) int
}

// SearchLimit returns how many candidates to fetch for r when the caller wants
// maxResults. Rerankers that do not implement SearchLimiter get the defaults
// (x3, capped at 100). A nil reranker returns maxResults unchanged.
func SearchLimit(r Reranker, maxResults int) int {
	if r == nil {
		return maxResults
	}
	if sl, ok := r.(SearchLimiter); ok {
		return sl.SearchLimit(maxResults)
	}
	return searchLimit(maxResults, DefaultCandidateMultiplier, DefaultMaxCandidates)
}

func searchLimit(maxResults, multiplier, maxCandidates int) int {
	if maxResults <= 0 {
		return maxResults
	}
	if multiplier < 1 {
		multiplier = DefaultCandidateMultiplier
	}
	if maxCandidates < 1 {
		maxCandidates = DefaultMaxCandidates
	}
	widened := maxResults * multiplier
	if widened > maxCandidates {
		widened = maxCandidates
	}
	// The ceiling caps the widening, never the caller's own request.
	if widened < maxResults {
		widened = maxResults
	}
	return widened
}

// BaseReranker fornece implementação default reutilizável.
type BaseReranker struct {
	ID    string
	TopN  int
	Model string
	// CandidateMultiplier is the number of candidates fetched per requested
	// result. Zero means DefaultCandidateMultiplier (3); set it to 1 for a
	// reranker that only needs to reorder.
	CandidateMultiplier int
	// MaxCandidates is the ceiling on the widened fetch. Zero means
	// DefaultMaxCandidates (100).
	MaxCandidates int
}

// GetID implementa Reranker.
func (b *BaseReranker) GetID() string {
	if b.ID != "" {
		return b.ID
	}
	return b.Model
}

// SearchLimit implementa SearchLimiter:
// max(min(maxResults*CandidateMultiplier, MaxCandidates), maxResults).
func (b *BaseReranker) SearchLimit(maxResults int) int {
	return searchLimit(maxResults, b.CandidateMultiplier, b.MaxCandidates)
}

// effectiveLimit combines the caller limit with the reranker TopN: the
// smallest positive value wins, 0 means unlimited.
func effectiveLimit(limit, topN int) int {
	switch {
	case limit > 0 && topN > 0:
		if topN < limit {
			return topN
		}
		return limit
	case limit > 0:
		return limit
	default:
		return topN
	}
}

type embedderCtxKey struct{}

// ContextWithEmbedder attaches the embedder the documents were indexed with to
// ctx. The knowledge pipeline does this before reranking so rerankers that
// embed the query (MMRReranker) use the same model as the vector db, the way
// Python attaches the embedder to its search results.
func ContextWithEmbedder(ctx context.Context, emb embedder.Embedder) context.Context {
	if emb == nil {
		return ctx
	}
	return context.WithValue(ctx, embedderCtxKey{}, emb)
}

// EmbedderFromContext returns the embedder attached by ContextWithEmbedder.
func EmbedderFromContext(ctx context.Context) embedder.Embedder {
	if ctx == nil {
		return nil
	}
	emb, _ := ctx.Value(embedderCtxKey{}).(embedder.Embedder)
	return emb
}

// copyResult returns a shallow copy of res so a reranker can set Score without
// mutating the caller's result. The Document pointer stays shared.
func copyResult(res *vectordb.SearchResult) *vectordb.SearchResult {
	if res == nil {
		return nil
	}
	c := *res
	return &c
}
