// Example: Valkey with the valkey-search module as an agno vector DB
// (vector, keyword and hybrid search with TAG filters).
//
//	docker run -d -p 6379:6379 valkey/valkey-bundle:latest
//	ollama pull nomic-embed-text
//	go run ./cookbook/vectordb/valkey
//
// Env: VALKEY_ADDR (default localhost:6379). Filterable TAG fields: id, name,
// source, category, status, tag, mode, content_id, content_hash, linked_to.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
	"github.com/devalexandre/agno-golang/agno/vectordb/valkey"
)

func main() {
	ctx := context.Background()
	emb := embedder.NewOllamaEmbedder(embedder.WithOllamaModel("nomic-embed-text", 768))

	addr := os.Getenv("VALKEY_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	db, err := valkey.New(valkey.Config{
		IndexName:  "agno_recipes",
		Addr:       addr,
		Embedder:   emb,
		SearchType: vectordb.SearchTypeHybrid,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Create(ctx); err != nil {
		log.Fatal(err)
	}

	docs := []*document.Document{
		{ID: "pad-thai", Name: "Pad Thai", Content: "Stir-fried rice noodles with tamarind, fish sauce, peanuts and lime.", Metadata: map[string]interface{}{"category": "thai"}},
		{ID: "tom-yum", Name: "Tom Yum", Content: "Hot and sour Thai soup with lemongrass, galangal and shrimp.", Metadata: map[string]interface{}{"category": "thai"}},
		{ID: "carbonara", Name: "Carbonara", Content: "Roman pasta with eggs, pecorino, guanciale and black pepper.", Metadata: map[string]interface{}{"category": "italian"}},
	}
	if err := db.Upsert(ctx, docs, nil); err != nil {
		log.Fatal(err)
	}

	results, err := db.Search(ctx, "spicy soup", 2, map[string]interface{}{"category": "thai"})
	if err != nil {
		log.Fatal(err)
	}
	for _, r := range results {
		fmt.Printf("%.4f  %s\n", r.Score, r.Document.Name)
	}
}
