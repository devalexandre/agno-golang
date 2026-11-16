// Example: Couchbase (Query + Search services over REST) as an agno vector DB.
//
//	docker run -d -p 8091-8096:8091-8096 -p 11210:11210 couchbase:enterprise-7.6.3
//	# open http://localhost:8091, initialise the cluster with the Data, Query,
//	# Index and Search services and create the bucket "recipes"
//	export COUCHBASE_USER=Administrator COUCHBASE_PASSWORD=password
//	ollama pull nomic-embed-text
//	go run ./cookbook/vectordb/couchbase
//
// Env: COUCHBASE_QUERY_URL (default http://localhost:8093),
// COUCHBASE_SEARCH_URL (default http://localhost:8094).
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb/couchbase"
)

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	ctx := context.Background()
	emb := embedder.NewOllamaEmbedder(embedder.WithOllamaModel("nomic-embed-text", 768))

	db, err := couchbase.New(couchbase.Config{
		QueryURL:    envOr("COUCHBASE_QUERY_URL", "http://localhost:8093"),
		SearchURL:   envOr("COUCHBASE_SEARCH_URL", "http://localhost:8094"),
		Username:    envOr("COUCHBASE_USER", "Administrator"),
		Password:    envOr("COUCHBASE_PASSWORD", "password"),
		Bucket:      "recipes",
		Scope:       "agno",
		Collection:  "docs",
		SearchIndex: "agno_recipes_idx",
		Embedder:    emb,
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Create(ctx); err != nil {
		log.Fatal(err)
	}

	docs := []*document.Document{
		{ID: "pad-thai", Name: "Pad Thai", Content: "Stir-fried rice noodles with tamarind, fish sauce, peanuts and lime."},
		{ID: "tom-yum", Name: "Tom Yum", Content: "Hot and sour Thai soup with lemongrass, galangal and shrimp."},
		{ID: "carbonara", Name: "Carbonara", Content: "Roman pasta with eggs, pecorino, guanciale and black pepper."},
	}
	if err := db.Upsert(ctx, docs, nil); err != nil {
		log.Fatal(err)
	}

	// The search index is built asynchronously; results may lag on first run.
	results, err := db.Search(ctx, "spicy soup", 2, nil)
	if err != nil {
		log.Fatal(err)
	}
	for _, r := range results {
		fmt.Printf("%.3f  %s\n", r.Score, r.Document.Name)
	}
}
