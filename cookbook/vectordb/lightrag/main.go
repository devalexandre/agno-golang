// Example: a LightRAG server as an agno vector DB. LightRAG chunks, embeds
// and builds a knowledge graph server-side; Search returns its answer.
//
//	docker run -d -p 9621:9621 --env-file .env ghcr.io/hkuds/lightrag:latest
//	# (.env configures LightRAG's LLM/embedding bindings, see the LightRAG docs)
//	go run ./cookbook/vectordb/lightrag
//
// Env: LIGHTRAG_SERVER_URL (default http://localhost:9621), LIGHTRAG_API_KEY.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/vectordb/lightrag"
)

func main() {
	ctx := context.Background()
	db := lightrag.New(lightrag.Config{Mode: lightrag.ModeHybrid})

	docs := []*document.Document{
		{Name: "recipes.txt", Content: "Tom Yum is a hot and sour Thai soup with lemongrass, galangal and shrimp. Carbonara is a Roman pasta with eggs and guanciale."},
	}
	if err := db.Insert(ctx, docs, nil); err != nil {
		log.Fatal(err)
	}
	fmt.Println("queued, track id:", docs[0].Metadata["lightrag_track_id"])

	// Indexing is asynchronous on the server; re-run the query once it finishes.
	results, err := db.Search(ctx, "Which dish is a sour soup?", 1, nil)
	if err != nil {
		log.Fatal(err)
	}
	for _, r := range results {
		fmt.Println(r.Document.Content)
	}
}
