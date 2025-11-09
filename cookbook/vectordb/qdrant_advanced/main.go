package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
	"github.com/devalexandre/agno-golang/agno/vectordb/qdrant"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func main() {
	ctx := context.Background()

	fmt.Println("🚀 Qdrant Advanced Features Demo")
	fmt.Println("=" + string(make([]byte, 50)))

	// Start Qdrant container using testcontainers
	fmt.Println("\n🐳 Starting Qdrant container...")
	container, host, port, err := setupQdrantContainer(ctx)
	if err != nil {
		log.Fatalf("Failed to start Qdrant container: %v", err)
	}
	defer func() {
		fmt.Println("\n🧹 Stopping Qdrant container...")
		if err := container.Terminate(ctx); err != nil {
			log.Printf("Failed to terminate container: %v", err)
		}
	}()

	fmt.Printf("✅ Qdrant running at %s:%d\n", host, port)

	// Create Ollama embedder (local)
	ollamaEmbedder := embedder.NewOllamaEmbedder(
		embedder.WithOllamaModel("gemma:2b", 2048),
		embedder.WithOllamaHost("http://localhost:11434"),
	)

	// Create Qdrant instance
	qdrantDB, err := qdrant.NewQdrant(qdrant.QdrantConfig{
		Host:       host,
		Port:       port,
		Collection: "advanced_demo",
		Embedder:   ollamaEmbedder,
		SearchType: vectordb.SearchTypeVector,
		Distance:   vectordb.DistanceCosine,
	})
	if err != nil {
		log.Fatalf("Failed to create Qdrant: %v", err)
	}
	defer qdrantDB.Close()

	// Create collection
	fmt.Println("\n📦 Creating collection...")
	if err := qdrantDB.Create(ctx); err != nil {
		log.Fatalf("Failed to create collection: %v", err)
	}

	// Demo 1: Batch Upsert
	fmt.Println("\n1️⃣ Demo: Batch Upsert")
	fmt.Println("-" + string(make([]byte, 50)))

	documents := []*document.Document{
		{
			ID:      "1",
			Name:    "Go Programming",
			Content: "Go is a statically typed, compiled programming language designed at Google. It is syntactically similar to C, but with memory safety, garbage collection, structural typing, and CSP-style concurrency.",
			Metadata: map[string]interface{}{
				"category": "programming",
				"language": "go",
				"year":     2009,
			},
		},
		{
			ID:      "2",
			Name:    "Python Programming",
			Content: "Python is a high-level, interpreted programming language with dynamic semantics. Its high-level built-in data structures, combined with dynamic typing and dynamic binding, make it very attractive for Rapid Application Development.",
			Metadata: map[string]interface{}{
				"category": "programming",
				"language": "python",
				"year":     1991,
			},
		},
		{
			ID:      "3",
			Name:    "Machine Learning",
			Content: "Machine learning is a method of data analysis that automates analytical model building. It is a branch of artificial intelligence based on the idea that systems can learn from data, identify patterns and make decisions with minimal human intervention.",
			Metadata: map[string]interface{}{
				"category": "ai",
				"field":    "machine-learning",
				"year":     2010,
			},
		},
		{
			ID:      "4",
			Name:    "Deep Learning",
			Content: "Deep learning is part of a broader family of machine learning methods based on artificial neural networks with representation learning. Learning can be supervised, semi-supervised or unsupervised.",
			Metadata: map[string]interface{}{
				"category": "ai",
				"field":    "deep-learning",
				"year":     2012,
			},
		},
		{
			ID:      "5",
			Name:    "Vector Databases",
			Content: "Vector databases are specialized databases designed to store and query high-dimensional vectors efficiently. They are essential for similarity search, recommendation systems, and AI applications.",
			Metadata: map[string]interface{}{
				"category": "database",
				"type":     "vector",
				"year":     2020,
			},
		},
	}

	if err := qdrantDB.BatchUpsert(ctx, documents, 2, nil); err != nil {
		log.Fatalf("Failed to batch upsert: %v", err)
	}
	fmt.Println("✅ Inserted 5 documents in batches")

	// Demo 2: Advanced Filtering
	fmt.Println("\n2️⃣ Demo: Advanced Filtering")
	fmt.Println("-" + string(make([]byte, 50)))

	advFilter := &qdrant.AdvancedFilter{
		Must: []qdrant.FilterCondition{
			{
				Field:    "category",
				Operator: qdrant.FilterOpEqual,
				Value:    "programming",
			},
		},
	}

	results, err := qdrantDB.SearchWithAdvancedFilters(ctx, "programming languages", 5, advFilter)
	if err != nil {
		log.Fatalf("Failed to search with advanced filters: %v", err)
	}

	fmt.Printf("Found %d results for 'programming languages' with category='programming':\n", len(results))
	for i, result := range results {
		fmt.Printf("  %d. %s (score: %.4f)\n", i+1, result.Document.Name, result.Score)
	}

	// Demo 3: Reranking
	fmt.Println("\n3️⃣ Demo: Search with Reranking")
	fmt.Println("-" + string(make([]byte, 50)))

	rerankConfig := &qdrant.RerankingConfig{
		Enabled:    true,
		TopK:       10,
		ScoreBoost: 1.5,
	}

	rerankedResults, err := qdrantDB.SearchWithReranking(ctx, "artificial intelligence and learning", 3, nil, rerankConfig)
	if err != nil {
		log.Fatalf("Failed to search with reranking: %v", err)
	}

	fmt.Printf("Top 3 reranked results for 'artificial intelligence and learning':\n")
	for i, result := range rerankedResults {
		fmt.Printf("  %d. %s (score: %.4f)\n", i+1, result.Document.Name, result.Score)
	}

	// Demo 4: Batch Search
	fmt.Println("\n4️⃣ Demo: Batch Search")
	fmt.Println("-" + string(make([]byte, 50)))

	queries := []string{
		"programming languages",
		"artificial intelligence",
		"databases",
	}

	batchResults, err := qdrantDB.BatchSearch(ctx, queries, 2, nil)
	if err != nil {
		log.Fatalf("Failed to batch search: %v", err)
	}

	for i, query := range queries {
		fmt.Printf("\nQuery: '%s'\n", query)
		for j, result := range batchResults[i] {
			fmt.Printf("  %d. %s (score: %.4f)\n", j+1, result.Document.Name, result.Score)
		}
	}

	// Demo 5: Update Payload
	fmt.Println("\n5️⃣ Demo: Update Payload")
	fmt.Println("-" + string(make([]byte, 50)))

	updateFilters := map[string]interface{}{
		"category": "programming",
	}

	updatePayload := map[string]interface{}{
		"updated":    true,
		"updated_at": "2025-01-09",
	}

	if err := qdrantDB.UpdatePayload(ctx, updateFilters, updatePayload); err != nil {
		log.Fatalf("Failed to update payload: %v", err)
	}
	fmt.Println("✅ Updated payload for all programming documents")

	// Verify update
	results, err = qdrantDB.Search(ctx, "programming", 2, map[string]interface{}{"category": "programming"})
	if err != nil {
		log.Fatalf("Failed to verify update: %v", err)
	}

	fmt.Println("\nVerifying updates:")
	for i, result := range results {
		updated := result.Document.Metadata["updated"]
		updatedAt := result.Document.Metadata["updated_at"]
		fmt.Printf("  %d. %s - Updated: %v, Updated At: %v\n", i+1, result.Document.Name, updated, updatedAt)
	}

	// Demo 6: Collection Info
	fmt.Println("\n6️⃣ Demo: Collection Information")
	fmt.Println("-" + string(make([]byte, 50)))

	info, err := qdrantDB.GetCollectionInfo(ctx)
	if err != nil {
		log.Fatalf("Failed to get collection info: %v", err)
	}

	fmt.Printf("Collection: %s\n", info.Name)
	fmt.Printf("Vector Size: %d\n", info.VectorSize)
	fmt.Printf("Points Count: %d\n", info.PointsCount)
	fmt.Printf("Status: %s\n", info.Status)

	// Demo 7: Hybrid Search
	fmt.Println("\n7️⃣ Demo: Hybrid Search")
	fmt.Println("-" + string(make([]byte, 50)))

	hybridResults, err := qdrantDB.HybridSearch(ctx, "learning systems", 3, nil)
	if err != nil {
		log.Fatalf("Failed to perform hybrid search: %v", err)
	}

	fmt.Printf("Top 3 hybrid search results for 'learning systems':\n")
	for i, result := range hybridResults {
		fmt.Printf("  %d. %s (score: %.4f)\n", i+1, result.Document.Name, result.Score)
	}

	// Demo 8: Delete by Filter
	fmt.Println("\n8️⃣ Demo: Delete by Filter")
	fmt.Println("-" + string(make([]byte, 50)))

	deleteFilters := map[string]interface{}{
		"category": "database",
	}

	if err := qdrantDB.DeleteByFilter(ctx, deleteFilters); err != nil {
		log.Fatalf("Failed to delete by filter: %v", err)
	}
	fmt.Println("✅ Deleted all database documents")

	// Verify deletion
	count, err := qdrantDB.GetCount(ctx)
	if err != nil {
		log.Fatalf("Failed to get count: %v", err)
	}
	fmt.Printf("Remaining documents: %d\n", count)

	// Cleanup
	fmt.Println("\n🧹 Cleaning up...")
	if err := qdrantDB.Drop(ctx); err != nil {
		log.Fatalf("Failed to drop collection: %v", err)
	}
	fmt.Println("✅ Collection dropped")

	fmt.Println("\n✨ Demo completed successfully!")
}

// setupQdrantContainer starts a Qdrant container for the demo
func setupQdrantContainer(ctx context.Context) (testcontainers.Container, string, int, error) {
	req := testcontainers.ContainerRequest{
		Image:        "qdrant/qdrant:latest",
		ExposedPorts: []string{"6333/tcp"},
		WaitingFor:   wait.ForHTTP("/").WithPort("6333/tcp").WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to start container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to get container host: %w", err)
	}

	mappedPort, err := container.MappedPort(ctx, "6333")
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to get mapped port: %w", err)
	}

	return container, host, mappedPort.Int(), nil
}
