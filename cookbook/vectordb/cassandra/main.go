// Example: Apache Cassandra 5 native vector search (SAI) as an agno vector DB.
//
//	docker run -d --name cassandra -p 9042:9042 cassandra:5.0
//	docker exec -it cassandra cqlsh -e \
//	  "CREATE KEYSPACE agno WITH replication = {'class':'SimpleStrategy','replication_factor':1};"
//	ollama pull nomic-embed-text
//	go run ./cookbook/vectordb/cassandra
//
// Env: CASSANDRA_HOST (default 127.0.0.1).
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb/cassandra"
)

func main() {
	ctx := context.Background()
	emb := embedder.NewOllamaEmbedder(embedder.WithOllamaModel("nomic-embed-text", 768))

	host := os.Getenv("CASSANDRA_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	db, closeFn, err := cassandra.NewWithGocql(
		cassandra.GocqlConfig{Hosts: []string{host}},
		cassandra.Config{Keyspace: "agno", TableName: "recipes", Embedder: emb},
	)
	if err != nil {
		log.Fatal(err)
	}
	defer closeFn()
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

	results, err := db.Search(ctx, "spicy soup", 2, map[string]interface{}{"cuisine": "thai"})
	if err != nil {
		log.Fatal(err)
	}
	for _, r := range results {
		fmt.Printf("%.3f  %s\n", r.Score, r.Document.Name)
	}
}
