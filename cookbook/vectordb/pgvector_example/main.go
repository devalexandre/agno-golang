package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
	"github.com/devalexandre/agno-golang/agno/vectordb/pgvector"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func main() {
	ctx := context.Background()

	fmt.Println("🚀 PgVector Demo with Testcontainers")
	fmt.Println("=" + string(make([]byte, 50)))

	// Start PostgreSQL container with pgvector extension
	fmt.Println("\n🐳 Starting PostgreSQL container with pgvector...")
	container, connStr, err := setupPgVectorContainer(ctx)
	if err != nil {
		log.Fatalf("Failed to start PostgreSQL container: %v", err)
	}
	defer func() {
		fmt.Println("\n🧹 Stopping PostgreSQL container...")
		if err := container.Terminate(ctx); err != nil {
			log.Printf("Failed to terminate container: %v", err)
		}
	}()

	fmt.Printf("✅ PostgreSQL running with connection: %s\n", connStr)

	// Create Ollama embedder (local)
	ollamaEmbedder := embedder.NewOllamaEmbedder(
		embedder.WithOllamaModel("gemma:2b", 2048),
		embedder.WithOllamaHost("http://localhost:11434"),
	)

	// Create PgVector instance
	pgDB, err := pgvector.NewPgVector(pgvector.PgVectorConfig{
		ConnectionString: connStr,
		TableName:        "documents",
		Embedder:         ollamaEmbedder,
		SearchType:       vectordb.SearchTypeVector,
		Distance:         vectordb.DistanceCosine,
	})
	if err != nil {
		log.Fatalf("Failed to create PgVector: %v", err)
	}
	defer pgDB.Close()

	// Create table
	fmt.Println("\n📦 Creating table...")
	if err := pgDB.Create(ctx); err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	// Demo 1: Insert Documents
	fmt.Println("\n1️⃣ Demo: Insert Documents")
	fmt.Println("-" + string(make([]byte, 50)))

	documents := []*document.Document{
		{
			ID:      "1",
			Name:    "Go Programming",
			Content: "Go is a statically typed, compiled programming language designed at Google.",
			Metadata: map[string]interface{}{
				"category": "programming",
				"language": "go",
			},
		},
		{
			ID:      "2",
			Name:    "Python Programming",
			Content: "Python is a high-level, interpreted programming language with dynamic semantics.",
			Metadata: map[string]interface{}{
				"category": "programming",
				"language": "python",
			},
		},
		{
			ID:      "3",
			Name:    "Machine Learning",
			Content: "Machine learning is a method of data analysis that automates analytical model building.",
			Metadata: map[string]interface{}{
				"category": "ai",
				"field":    "machine-learning",
			},
		},
		{
			ID:      "4",
			Name:    "PostgreSQL Database",
			Content: "PostgreSQL is a powerful, open source object-relational database system.",
			Metadata: map[string]interface{}{
				"category": "database",
				"type":     "relational",
			},
		},
	}

	if err := pgDB.Insert(ctx, documents, nil); err != nil {
		log.Fatalf("Failed to insert documents: %v", err)
	}
	fmt.Println("✅ Inserted 4 documents")

	// Demo 2: Vector Search
	fmt.Println("\n2️⃣ Demo: Vector Search")
	fmt.Println("-" + string(make([]byte, 50)))

	results, err := pgDB.Search(ctx, "programming languages", 3, nil)
	if err != nil {
		log.Fatalf("Failed to search: %v", err)
	}

	fmt.Printf("Top 3 results for 'programming languages':\n")
	for i, result := range results {
		fmt.Printf("  %d. %s (score: %.4f)\n", i+1, result.Document.Name, result.Score)
	}

	// Demo 3: Search with Filters
	fmt.Println("\n3️⃣ Demo: Search with Metadata Filters")
	fmt.Println("-" + string(make([]byte, 50)))

	filters := map[string]interface{}{
		"category": "programming",
	}

	filteredResults, err := pgDB.Search(ctx, "coding", 5, filters)
	if err != nil {
		log.Fatalf("Failed to search with filters: %v", err)
	}

	fmt.Printf("Results for 'coding' with category='programming':\n")
	for i, result := range filteredResults {
		fmt.Printf("  %d. %s (score: %.4f)\n", i+1, result.Document.Name, result.Score)
	}

	// Demo 4: Get Document Count
	fmt.Println("\n4️⃣ Demo: Get Document Count")
	fmt.Println("-" + string(make([]byte, 50)))

	count, err := pgDB.GetCount(ctx)
	if err != nil {
		log.Fatalf("Failed to get count: %v", err)
	}
	fmt.Printf("Total documents: %d\n", count)

	// Demo 5: Check Document Existence
	fmt.Println("\n5️⃣ Demo: Check Document Existence")
	fmt.Println("-" + string(make([]byte, 50)))

	exists, err := pgDB.IDExists(ctx, "1")
	if err != nil {
		log.Fatalf("Failed to check existence: %v", err)
	}
	fmt.Printf("Document with ID '1' exists: %v\n", exists)

	// Demo 6: Hybrid Search
	fmt.Println("\n6️⃣ Demo: Hybrid Search")
	fmt.Println("-" + string(make([]byte, 50)))

	hybridResults, err := pgDB.HybridSearch(ctx, "database systems", 3, nil)
	if err != nil {
		log.Fatalf("Failed to perform hybrid search: %v", err)
	}

	fmt.Printf("Top 3 hybrid search results for 'database systems':\n")
	for i, result := range hybridResults {
		fmt.Printf("  %d. %s (score: %.4f)\n", i+1, result.Document.Name, result.Score)
	}

	// Cleanup
	fmt.Println("\n🧹 Cleaning up...")
	if err := pgDB.Drop(ctx); err != nil {
		log.Fatalf("Failed to drop table: %v", err)
	}
	fmt.Println("✅ Table dropped")

	fmt.Println("\n✨ Demo completed successfully!")
}

// setupPgVectorContainer starts a PostgreSQL container with pgvector extension
func setupPgVectorContainer(ctx context.Context) (testcontainers.Container, string, error) {
	req := testcontainers.ContainerRequest{
		Image:        "pgvector/pgvector:pg16",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "postgres",
			"POSTGRES_DB":       "vectordb",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to start container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get container host: %w", err)
	}

	mappedPort, err := container.MappedPort(ctx, "5432")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get mapped port: %w", err)
	}

	connStr := fmt.Sprintf("host=%s port=%d user=postgres password=postgres dbname=vectordb sslmode=disable",
		host, mappedPort.Int())

	return container, connStr, nil
}
