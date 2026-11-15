// Example: Upstash Vector (serverless, REST) as an agno vector DB.
//
// Create an index at https://console.upstash.com (dimension 768, cosine), then:
//
//	export UPSTASH_VECTOR_REST_URL=https://<your-index>.upstash.io
//	export UPSTASH_VECTOR_REST_TOKEN=<token>
//	ollama pull nomic-embed-text
//	go run ./cookbook/vectordb/upstash
//
// On an index created with a built-in embedding model, leave Embedder nil and
// set UseUpstashEmbeddings so Upstash embeds the text server-side.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb/upstash"
)

func main() {
	ctx := context.Background()
	emb := embedder.NewOllamaEmbedder(embedder.WithOllamaModel("nomic-embed-text", 768))

	// URL and Token come from UPSTASH_VECTOR_REST_URL / UPSTASH_VECTOR_REST_TOKEN.
	db, err := upstash.New(upstash.Config{Namespace: "recipes", Embedder: emb})
	if err != nil {
		log.Fatal(err)
	}

	docs := []*document.Document{
		{ID: "pad-thai", Name: "Pad Thai", Content: "Stir-fried rice noodles with tamarind, fish sauce, peanuts and lime.", Metadata: map[string]interface{}{"cuisine": "thai"}},
		{ID: "tom-yum", Name: "Tom Yum", Content: "Hot and sour Thai soup with lemongrass, galangal and shrimp.", Metadata: map[string]interface{}{"cuisine": "thai"}},
		{ID: "carbonara", Name: "Carbonara", Content: "Roman pasta with eggs, pecorino, guanciale and black pepper.", Metadata: map[string]interface{}{"cuisine": "italian"}},
	}
	if err := db.Upsert(ctx, docs, nil); err != nil {
		log.Fatal(err)
	}
	if n, err := db.GetCount(ctx); err == nil {
		fmt.Println("documents in namespace:", n)
	}

	results, err := db.Search(ctx, "spicy soup", 2, map[string]interface{}{"cuisine": "thai"})
	if err != nil {
		log.Fatal(err)
	}
	for _, r := range results {
		fmt.Printf("%.3f  %s\n", r.Score, r.Document.Name)
	}
}
