// Recency reranker applied directly to search results.
//
// RecencyReranker blends the search score with an exponential decay on a
// document timestamp:
//
//	score = (1-Weight)*relevance + Weight*0.5^(ageDays/HalfLifeDays)
//
// The timestamp comes from Document.Metadata["updated_at"] (TimestampKey),
// falling back to "_agno_updated_at"; RFC 3339 strings, time.Time and epoch
// seconds/milliseconds are accepted. Undated documents rank on relevance alone.
// Defaults: 30-day half-life, weight 0.3.
//
// Runs offline. Through Knowledge: kb.Reranker = reranker.NewRecencyReranker()
// (see cookbook/knowledge/rerank_hyde).
//
//	go run ./cookbook/knowledge/reranker/recency
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/knowledge/reranker"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

func main() {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	results := []*vectordb.SearchResult{
		result("Go 1.18 added generics.", 0.92, "2022-03-15T00:00:00Z"),
		result("Go 1.22 changed loop variable scoping.", 0.88, "2024-02-06T00:00:00Z"),
		result("Go 1.26 release notes.", 0.80, now.AddDate(0, 0, -10)), // time.Time
		result("Undated blog post about Go releases.", 0.85, nil),
		result("Go 1.25 release notes.", 0.78, now.AddDate(0, -3, 0).Unix()), // epoch seconds
	}

	show("search order", results)

	for _, weight := range []float64{0.3, 0.7} {
		rec := reranker.NewRecencyReranker()
		rec.Weight = weight
		rec.HalfLifeDays = 90
		rec.Now = func() time.Time { return now } // fixed clock for a reproducible demo
		out, err := rec.Rerank(context.Background(), "latest Go release", results, 3)
		if err != nil {
			log.Fatal(err)
		}
		show(fmt.Sprintf("recency weight=%.1f half-life=90d (top 3)", weight), out)
	}
}

func result(content string, score float64, updatedAt interface{}) *vectordb.SearchResult {
	meta := map[string]interface{}{}
	if updatedAt != nil {
		meta[reranker.RecencyMetadataKey] = updatedAt
	}
	return &vectordb.SearchResult{Document: &document.Document{Content: content, Metadata: meta}, Score: score}
}

func show(title string, rs []*vectordb.SearchResult) {
	fmt.Println(title + ":")
	for i, r := range rs {
		fmt.Printf("  %d. (%.3f) %s\n", i+1, r.Score, r.Document.Content)
	}
}
