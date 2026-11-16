// Knowledge filter expressions (agno/filters).
//
// Builds composable filters (EQ, NEQ, GT, GTE, LT, LTE, IN, CONTAINS,
// STARTSWITH, AND, OR, NOT), serializes them to the same JSON shape Python
// uses (so they can travel through APIs), evaluates them against document
// metadata, and applies one to knowledge search results with
// knowledge.FilterResults (BaseKnowledge.SearchWithFilterExpr does the same
// against a live vector DB).
//
// Runs fully offline. Requires no env vars or services.
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/filters"
	"github.com/devalexandre/agno-golang/agno/knowledge"
)

func main() {
	f := filters.OR(
		filters.AND(filters.EQ("type", "article"), filters.GT("word_count", 500)),
		filters.AND(filters.EQ("type", "tutorial"), filters.NOT(filters.EQ("difficulty", "beginner"))),
	).And(filters.IN("lang", "en", "pt"))

	data, _ := json.MarshalIndent(f, "", "  ")
	fmt.Printf("filter: %s\nJSON:\n%s\n\n", f, data)

	// Round trip, e.g. a filter received from an HTTP request body.
	var received filters.Expr
	if err := json.Unmarshal(data, &received); err != nil {
		log.Fatal(err)
	}

	docs := []*knowledge.SearchResult{
		{Document: document.NewDocumentWithMetadata("Long article", map[string]interface{}{"type": "article", "word_count": 1200, "lang": "en"}), Score: 0.91},
		{Document: document.NewDocumentWithMetadata("Short article", map[string]interface{}{"type": "article", "word_count": 200, "lang": "en"}), Score: 0.88},
		{Document: document.NewDocumentWithMetadata("Advanced tutorial", map[string]interface{}{"type": "tutorial", "difficulty": "advanced", "lang": "pt"}), Score: 0.80},
		{Document: document.NewDocumentWithMetadata("Beginner tutorial", map[string]interface{}{"type": "tutorial", "difficulty": "beginner", "lang": "en"}), Score: 0.75},
		{Document: document.NewDocumentWithMetadata("German article", map[string]interface{}{"type": "article", "word_count": 900, "lang": "de"}), Score: 0.70},
	}

	for _, r := range knowledge.FilterResults(docs, &received) {
		fmt.Printf("match (%.2f): %s %v\n", r.Score, r.Document.Content, r.Document.Metadata)
	}

	// Legacy equality maps convert to expressions too.
	legacy := filters.FromMap(map[string]interface{}{"lang": "en"})
	fmt.Printf("\nlegacy map filter %s keeps %d docs\n", legacy, len(knowledge.FilterResults(docs, legacy)))
}
