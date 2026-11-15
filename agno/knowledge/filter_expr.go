package knowledge

import (
	"context"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/filters"
)

// FilterResults keeps the results whose document metadata matches expr.
// A nil expr returns results unchanged.
func FilterResults(results []*SearchResult, expr *filters.Expr) []*SearchResult {
	if expr == nil {
		return results
	}
	out := make([]*SearchResult, 0, len(results))
	for _, r := range results {
		if r == nil || r.Document == nil {
			continue
		}
		meta := r.Document.Metadata
		if meta == nil {
			meta = map[string]interface{}{}
		}
		if expr.Match(meta) {
			out = append(out, r)
		}
	}
	return out
}

// SearchWithFilterExpr searches with a filter expression (AND/OR/NOT/EQ/IN/GT...).
//
// Vector databases in this module take equality maps, so the expression is
// applied after the search: the backend is asked for oversample x
// numDocuments candidates (constrained by the knowledge base's static
// Include filters) and the expression narrows them to at most numDocuments.
func (k *BaseKnowledge) SearchWithFilterExpr(ctx context.Context, query string, numDocuments int, expr *filters.Expr) ([]*SearchResult, error) {
	if numDocuments <= 0 {
		numDocuments = k.NumDocuments
	}
	if numDocuments <= 0 {
		numDocuments = 5
	}
	if k.VectorDB == nil {
		return nil, fmt.Errorf("vector database not configured")
	}
	var base map[string]interface{}
	if k.Filters != nil && len(k.Filters.Include) > 0 {
		base = k.Filters.Include
	}
	limit := numDocuments
	if expr != nil {
		limit = numDocuments * 4
	}
	results, err := k.VectorDB.Search(ctx, query, limit, base)
	if err != nil {
		return nil, err
	}
	results = FilterResults(results, expr)
	if len(results) > numDocuments {
		results = results[:numDocuments]
	}
	return results, nil
}
