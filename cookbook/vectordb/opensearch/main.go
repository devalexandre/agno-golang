// Example: OpenSearch k-NN as an agno vector DB (hybrid vector + keyword search).
//
//	docker run -p 9200:9200 -e discovery.type=single-node \
//	  -e DISABLE_SECURITY_PLUGIN=true opensearchproject/opensearch:2.17.0
//	ollama pull nomic-embed-text
//	go run ./cookbook/vectordb/opensearch
//
// Env: OPENSEARCH_URL (default http://localhost:9200).
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
	"github.com/devalexandre/agno-golang/agno/vectordb/opensearch"
)

func main() {
	ctx := context.Background()
	emb := embedder.NewOllamaEmbedder(embedder.WithOllamaModel("nomic-embed-text", 768))

	url := os.Getenv("OPENSEARCH_URL")
	if url == "" {
		url = "http://localhost:9200"
	}
	db, err := opensearch.New(opensearch.Config{
		URL:        url,
		IndexName:  "agno_recipes",
		Embedder:   emb,
		SearchType: vectordb.SearchTypeHybrid,
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Create(ctx); err != nil {
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
		fmt.Println("documents:", n)
	}

	results, err := db.Search(ctx, "spicy soup", 2, map[string]interface{}{"cuisine": "thai"})
	if err != nil {
		log.Fatal(err)
	}
	for _, r := range results {
		fmt.Printf("%.3f  %s\n", r.Score, r.Document.Name)
	}
}
