package storage

import "sort"

// SortPaginateKnowledge sorts knowledge rows (by "created_at" — default —,
// "updated_at" or "name"; "desc" by default) and returns the requested
// 1-based page. Nil limit returns every row.
func SortPaginateKnowledge(rows []*KnowledgeRow, limit, page *int, sortBy, sortOrder *string) []*KnowledgeRow {
	by, order := "created_at", "desc"
	if sortBy != nil && *sortBy != "" {
		by = *sortBy
	}
	if sortOrder != nil && *sortOrder != "" {
		order = *sortOrder
	}
	i64 := func(p *int64) int64 {
		if p == nil {
			return 0
		}
		return *p
	}
	less := func(a, b *KnowledgeRow) bool {
		switch by {
		case "name":
			if a.Name != b.Name {
				return a.Name < b.Name
			}
		case "updated_at":
			if i64(a.UpdatedAt) != i64(b.UpdatedAt) {
				return i64(a.UpdatedAt) < i64(b.UpdatedAt)
			}
		default:
			if i64(a.CreatedAt) != i64(b.CreatedAt) {
				return i64(a.CreatedAt) < i64(b.CreatedAt)
			}
		}
		return a.ID < b.ID
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if order == "asc" {
			return less(rows[i], rows[j])
		}
		return less(rows[j], rows[i])
	})
	if limit == nil || *limit <= 0 {
		return rows
	}
	p := 1
	if page != nil && *page > 0 {
		p = *page
	}
	start := (p - 1) * *limit
	if start >= len(rows) {
		return []*KnowledgeRow{}
	}
	end := start + *limit
	if end > len(rows) {
		end = len(rows)
	}
	return rows[start:end]
}
