package reranker

import (
	"context"
	"sort"
	"strings"

	"github.com/devalexandre/agno-golang/agno/vectordb"
)

// LexicalReranker is a dependency-free reranker that scores results by term
// overlap (Jaccard-like) between the query and each document. It requires no
// API and is useful as a fallback or for offline/testing scenarios.
type LexicalReranker struct {
	BaseReranker
}

// NewLexicalReranker creates a lexical reranker returning up to topN results.
func NewLexicalReranker(topN int) *LexicalReranker {
	return &LexicalReranker{BaseReranker{ID: "lexical", Model: "lexical", TopN: topN}}
}

// Rerank implements Reranker.
func (r *LexicalReranker) Rerank(_ context.Context, query string, results []*vectordb.SearchResult, topN int) ([]*vectordb.SearchResult, error) {
	if len(results) == 0 {
		return results, nil
	}
	if topN <= 0 {
		topN = r.TopN
	}

	qTerms := termSet(query)
	scored := make([]*vectordb.SearchResult, len(results))
	copy(scored, results)

	for _, res := range scored {
		content := ""
		if res.Document != nil {
			content = res.Document.Content
		}
		res.Score = overlapScore(qTerms, termSet(content))
	}

	sort.SliceStable(scored, func(i, j int) bool { return scored[i].Score > scored[j].Score })

	if topN > 0 && topN < len(scored) {
		scored = scored[:topN]
	}
	return scored, nil
}

func termSet(s string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, w := range strings.Fields(strings.ToLower(s)) {
		w = strings.Trim(w, ".,!?;:()[]{}\"'")
		if w != "" {
			set[w] = struct{}{}
		}
	}
	return set
}

// overlapScore returns the fraction of query terms present in the document.
func overlapScore(q, d map[string]struct{}) float64 {
	if len(q) == 0 {
		return 0
	}
	matches := 0
	for term := range q {
		if _, ok := d[term]; ok {
			matches++
		}
	}
	return float64(matches) / float64(len(q))
}

var _ Reranker = (*LexicalReranker)(nil)
