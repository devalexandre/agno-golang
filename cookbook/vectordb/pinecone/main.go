package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb/pinecone"
)

func main() {
	ctx := context.Background()

	// 1. Configuration
	apiKey := os.Getenv("PINECONE_API_KEY")
	indexURL := os.Getenv("PINECONE_INDEX_URL")

	if apiKey == "" || indexURL == "" {
		fmt.Println("⚠️  PINECONE_API_KEY and PINECONE_INDEX_URL environment variables are required.")
		fmt.Println("Skipping example execution.")
		return
	}

	// 2. Initialize Embedder
	emb := embedder.NewOllamaEmbedder(embedder.WithOllamaModel("nomic-embed-text", 768))

	// 3. Initialize Pinecone Client
	db := pinecone.NewPineconeDB(pinecone.PineconeOptions{
		APIKey:    apiKey,
		IndexURL:  indexURL,
		Namespace: "agno-test",
		Embedder:  emb,
	})

	// 4. Check Connection
	exists, err := db.Exists(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to Pinecone: %v", err)
	}
	if !exists {
		log.Fatal("Index does not exist or is not accessible")
	}
	fmt.Println("✅ Connected to Pinecone index")

	// 5. Insert Documents
	fmt.Println("📝 Inserting documents...")
	docs := []*document.Document{
		{
			ID:      "p1",
			Content: "Pinecone is a managed vector database service.",
			Metadata: map[string]interface{}{
				"type": "saas",
			},
		},
		{
			ID:      "p2",
			Content: "Agno supports multiple vector providers.",
			Metadata: map[string]interface{}{
				"type": "framework",
			},
		},
	}

	if err := db.Insert(ctx, docs, nil); err != nil {
		log.Fatalf("Failed to insert documents: %v", err)
	}

	// Wait for consistency
	time.Sleep(2 * time.Second)

	// 6. Perform Search
	query := "What is Pinecone?"
	fmt.Printf("\n🔎 Searching for: '%s'\n", query)

	results, err := db.Search(ctx, query, 2, nil)
	if err != nil {
		log.Fatalf("Failed to search: %v", err)
	}

	fmt.Printf("Found %d results:\n", len(results))
	for i, res := range results {
		fmt.Printf("%d. Score: %.4f | Content: %s\n", i+1, res.Score, res.Document.Content)
	}
}
