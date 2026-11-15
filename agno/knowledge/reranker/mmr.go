package reranker

import (
	"context"
	"fmt"
	"math"

	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

// DefaultMMRCandidateMultiplier is MMR's default pool widening: selection
// compares candidates against each other, so it needs a wider pool.
const DefaultMMRCandidateMultiplier = 5

// MMRReranker selects results that are relevant to the query but unlike each
// other (Maximal Marginal Relevance). Port of agno-python
// knowledge/reranker/mmr.py.
//
// It requires an embedding on every candidate (Document.Embeddings) and an
// embedder for the query. The query embedder is Embedder when set, otherwise
// the embedder the knowledge pipeline attached to ctx (ContextWithEmbedder),
// i.e. the vector db's own embedder, so the query uses the indexing model.
//
// Returned results are shallow copies whose Score is the MMR score at the
// moment each was picked. Those scores are NOT monotonic: the returned order is
// authoritative, so do not re-sort by Score.
type MMRReranker struct {
	BaseReranker
	// Embedder embeds the query. Optional when used through Knowledge.
	Embedder embedder.Embedder
	// Lambda weights relevance against diversity: 1.0 ranks by relevance
	// alone, 0.0 by difference alone. Must be within [0, 1].
	Lambda float64
}

// NewMMRReranker creates an MMR reranker. emb may be nil when the reranker is
// used through Knowledge (the vector db embedder is used). lambda must be in
// [0,1] (Python default 0.5). topN <= 0 means "no cap" (recommended on
// Knowledge, which trims to max results anyway).
func NewMMRReranker(emb embedder.Embedder, lambda float64, topN int) *MMRReranker {
	return &MMRReranker{
		BaseReranker: BaseReranker{
			ID:                  "mmr",
			Model:               "mmr",
			TopN:                topN,
			CandidateMultiplier: DefaultMMRCandidateMultiplier,
		},
		Embedder: emb,
		Lambda:   lambda,
	}
}

// SearchLimit implements SearchLimiter with MMR's wider default pool (x5).
func (r *MMRReranker) SearchLimit(maxResults int) int {
	mult := r.CandidateMultiplier
	if mult < 1 {
		mult = DefaultMMRCandidateMultiplier
	}
	return searchLimit(maxResults, mult, r.MaxCandidates)
}

// Rerank implements Reranker.
func (r *MMRReranker) Rerank(ctx context.Context, query string, results []*vectordb.SearchResult, limit int) ([]*vectordb.SearchResult, error) {
	if len(results) == 0 {
		return results, nil
	}
	if r.Lambda < 0 || r.Lambda > 1 || math.IsNaN(r.Lambda) {
		return nil, fmt.Errorf("%w: MMRReranker lambda must be between 0.0 and 1.0, got %v", ErrInvalidInput, r.Lambda)
	}

	// A zero vector is indistinguishable from a broken ingest and would be
	// selected as maximally different from everything.
	missing := 0
	for _, res := range results {
		if res == nil || res.Document == nil || isZero(res.Document.Embeddings) {
			missing++
		}
	}
	if missing > 0 {
		return nil, fmt.Errorf("%w: MMRReranker requires embeddings on search results, but the vector db returned %d result(s) without one", ErrInvalidInput, missing)
	}

	n := effectiveLimit(limit, r.TopN)
	if n <= 0 || n > len(results) {
		n = len(results)
	}

	emb := r.Embedder
	if emb == nil {
		emb = EmbedderFromContext(ctx)
	}
	if emb == nil {
		return nil, fmt.Errorf("%w: MMRReranker needs an embedder to embed the query; set MMRReranker.Embedder or use it through a Knowledge whose vector db exposes one", ErrInvalidInput)
	}
	queryEmbedding, err := emb.GetEmbedding(query)
	if err != nil {
		return nil, fmt.Errorf("mmr reranker: embed query: %w", err)
	}
	return r.selectMMR(queryEmbedding, results, n)
}

func (r *MMRReranker) selectMMR(queryEmbedding []float64, results []*vectordb.SearchResult, limit int) ([]*vectordb.SearchResult, error) {
	if len(queryEmbedding) == 0 {
		return nil, fmt.Errorf("%w: MMRReranker could not embed the query: the embedder returned no vector", ErrInvalidInput)
	}
	for _, res := range results {
		if len(res.Document.Embeddings) != len(queryEmbedding) {
			return nil, fmt.Errorf("%w: MMRReranker requires embeddings of one dimension, but got %d (query) and %d (document); the query embedder and the indexed documents likely use different models",
				ErrInvalidInput, len(queryEmbedding), len(res.Document.Embeddings))
		}
	}

	// Normalise once: every similarity below is then a dot product.
	embeddings := make([][]float64, len(results))
	relevance := make([]float64, len(results))
	unitQuery := unit(queryEmbedding)
	for i, res := range results {
		embeddings[i] = unit(res.Document.Embeddings)
		relevance[i] = dot(unitQuery, embeddings[i])
	}

	remaining := make([]int, len(results))
	for i := range remaining {
		remaining[i] = i
	}
	type pick struct {
		index int
		score float64
	}
	selected := make([]pick, 0, limit)

	// Seed with the closest match to the query; scoring the first pick with the
	// MMR formula would tie every candidate at lambda=0.
	firstPos := 0
	for pos, c := range remaining {
		if relevance[c] > relevance[remaining[firstPos]] {
			firstPos = pos
		}
	}
	first := remaining[firstPos]
	selected = append(selected, pick{first, relevance[first]})
	remaining = append(remaining[:firstPos], remaining[firstPos+1:]...)

	// Each candidate's similarity to its nearest selected document.
	bestRedundancy := make([]float64, len(results))
	for _, c := range remaining {
		bestRedundancy[c] = dot(embeddings[c], embeddings[first])
	}

	for len(remaining) > 0 && len(selected) < limit {
		bestPos := 0
		bestScore := math.Inf(-1)
		for pos, c := range remaining {
			score := r.Lambda*relevance[c] - (1.0-r.Lambda)*bestRedundancy[c]
			if score > bestScore {
				bestScore = score
				bestPos = pos
			}
		}
		best := remaining[bestPos]
		selected = append(selected, pick{best, bestScore})
		remaining = append(remaining[:bestPos], remaining[bestPos+1:]...)
		for _, c := range remaining {
			if sim := dot(embeddings[c], embeddings[best]); sim > bestRedundancy[c] {
				bestRedundancy[c] = sim
			}
		}
	}

	out := make([]*vectordb.SearchResult, 0, len(selected))
	for _, p := range selected {
		res := copyResult(results[p.index])
		res.Score = p.score
		out = append(out, res)
	}
	return out, nil
}

func isZero(v []float64) bool {
	for _, x := range v {
		if x != 0 {
			return false
		}
	}
	return true
}

func dot(a, b []float64) float64 {
	var s float64
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}

func unit(v []float64) []float64 {
	norm := math.Sqrt(dot(v, v))
	out := make([]float64, len(v))
	if norm == 0 {
		return out
	}
	for i, x := range v {
		out[i] = x / norm
	}
	return out
}

var (
	_ Reranker      = (*MMRReranker)(nil)
	_ SearchLimiter = (*MMRReranker)(nil)
)
