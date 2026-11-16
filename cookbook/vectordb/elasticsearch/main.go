// Example: Elasticsearch as an agno vector DB (vector, keyword and hybrid search).
//
// Start a local single-node cluster (security disabled for the demo):
//
//	docker run -p 9200:9200 -e discovery.type=single-node \
//	  -e xpack.security.enabled=false docker.elastic.co/elasticsearch/elasticsearch:8.15.0
//	go run ./cookbook/vectordb/elasticsearch
//
// For Elastic Cloud set Addresses to the https endpoint and APIKey (or
// Username/Password).
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
	"github.com/devalexandre/agno-golang/agno/vectordb/elasticsearch"
)

func main() {
	ctx := context.Background()

	emb := embedder.NewOllamaEmbedder(embedder.WithOllamaModel("nomic-embed-text", 768))
	db, err := elasticsearch.New(elasticsearch.Config{
		Addresses:  []string{"http://localhost:9200"},
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

	count, _ := db.GetCount(ctx)
	fmt.Printf("indexed %d documents\n", count)

	results, err := db.Search(ctx, "spicy sour soup", 2, map[string]interface{}{"cuisine": "thai"})
	if err != nil {
		log.Fatal(err)
	}
	for _, r := range results {
		fmt.Printf("%.3f  %s: %s\n", r.Score, r.Document.Name, r.Document.Content)
	}
}
