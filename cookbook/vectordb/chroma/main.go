package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb/chroma"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func main() {
	ctx := context.Background()

	// 1. Start ChromaDB using Testcontainers
	fmt.Println("🚀 Starting ChromaDB container...")
	req := testcontainers.ContainerRequest{
		Image:        "chromadb/chroma:latest",
		ExposedPorts: []string{"8000/tcp"},
		WaitingFor:   wait.ForHTTP("/api/v1/heartbeat").WithPort("8000/tcp"),
	}

	chromaContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		log.Fatalf("Failed to start container: %v", err)
	}
	defer func() {
		if err := chromaContainer.Terminate(ctx); err != nil {
			log.Printf("Failed to terminate container: %v", err)
		}
	}()

	// Get the host and port
	host, err := chromaContainer.Host(ctx)
	if err != nil {
		log.Fatalf("Failed to get container host: %v", err)
	}
	port, err := chromaContainer.MappedPort(ctx, "8000")
	if err != nil {
		log.Fatalf("Failed to get container port: %v", err)
	}

	fmt.Printf("✅ ChromaDB started at %s:%d\n", host, port.Int())

	// 2. Initialize Embedder (Ollama)
	emb := embedder.NewOllamaEmbedder(embedder.WithOllamaModel("nomic-embed-text", 768))

	// 3. Initialize ChromaDB Client
	db := chroma.NewChromaDB(chroma.ChromaOptions{
		Host:       host,
		Port:       port.Int(),
		Collection: "test_collection",
		Embedder:   emb,
	})

	// 4. Create Collection
	fmt.Println("📦 Creating collection...")
	if err := db.Create(ctx); err != nil {
		log.Fatalf("Failed to create collection: %v", err)
	}

	// 5. Insert Documents
	fmt.Println("📝 Inserting documents...")
	docs := []*document.Document{
		{
			ID:      "1",
			Content: "Agno is a powerful framework for building AI agents.",
			Metadata: map[string]interface{}{
				"category": "framework",
			},
		},
		{
			ID:      "2",
			Content: "ChromaDB is an open-source vector database.",
			Metadata: map[string]interface{}{
				"category": "database",
			},
		},
		{
			ID:      "3",
			Content: "Go is a statically typed, compiled programming language.",
			Metadata: map[string]interface{}{
				"category": "language",
			},
		},
	}

	if err := db.Insert(ctx, docs, nil); err != nil {
		log.Fatalf("Failed to insert documents: %v", err)
	}

	// Wait a bit for indexing (Chroma is usually fast but good practice)
	time.Sleep(1 * time.Second)

	// 6. Perform Search
	query := "What is Agno?"
	fmt.Printf("\n🔎 Searching for: '%s'\n", query)

	results, err := db.Search(ctx, query, 2, nil)
	if err != nil {
		log.Fatalf("Failed to search: %v", err)
	}

	fmt.Printf("Found %d results:\n", len(results))
	for i, res := range results {
		fmt.Printf("%d. Score: %.4f | Content: %s\n", i+1, res.Score, res.Document.Content)
	}

	// 7. Clean up
	fmt.Println("\n🧹 Cleaning up...")
	if err := db.Drop(ctx); err != nil {
		log.Printf("Failed to drop collection: %v", err)
	}
}
