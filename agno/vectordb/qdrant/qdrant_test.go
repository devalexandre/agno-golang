package qdrant

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupQdrantContainer starts a Qdrant container for testing
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

func TestQdrantBasicOperations(t *testing.T) {
	ctx := context.Background()

	// Start Qdrant container
	container, host, port, err := setupQdrantContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup Qdrant container: %v", err)
	}
	defer container.Terminate(ctx)

	// Create mock embedder
	mockEmbedder := embedder.NewMockEmbedder(768)

	// Create Qdrant instance
	qdrantDB, err := NewQdrant(QdrantConfig{
		Host:       host,
		Port:       port,
		Collection: "test_collection",
		Embedder:   mockEmbedder,
		SearchType: vectordb.SearchTypeVector,
		Distance:   vectordb.DistanceCosine,
	})
	if err != nil {
		t.Fatalf("Failed to create Qdrant: %v", err)
	}
	defer qdrantDB.Close()

	// Test: Create collection
	t.Run("CreateCollection", func(t *testing.T) {
		err := qdrantDB.Create(ctx)
		if err != nil {
			t.Fatalf("Failed to create collection: %v", err)
		}

		exists, err := qdrantDB.Exists(ctx)
		if err != nil {
			t.Fatalf("Failed to check collection existence: %v", err)
		}
		if !exists {
			t.Error("Collection should exist after creation")
		}
	})

	// Test: Insert documents
	t.Run("InsertDocuments", func(t *testing.T) {
		docs := []*document.Document{
			{
				ID:      "1",
				Name:    "Test Doc 1",
				Content: "This is a test document about Go programming",
				Metadata: map[string]interface{}{
					"category": "programming",
					"language": "go",
				},
			},
			{
				ID:      "2",
				Name:    "Test Doc 2",
				Content: "This is a test document about Python programming",
				Metadata: map[string]interface{}{
					"category": "programming",
					"language": "python",
				},
			},
		}

		err := qdrantDB.Insert(ctx, docs, nil)
		if err != nil {
			t.Fatalf("Failed to insert documents: %v", err)
		}

		// Verify count
		count, err := qdrantDB.GetCount(ctx)
		if err != nil {
			t.Fatalf("Failed to get count: %v", err)
		}
		if count != 2 {
			t.Errorf("Expected 2 documents, got %d", count)
		}
	})

	// Test: Search
	t.Run("Search", func(t *testing.T) {
		results, err := qdrantDB.Search(ctx, "programming", 10, nil)
		if err != nil {
			t.Fatalf("Failed to search: %v", err)
		}

		if len(results) == 0 {
			t.Error("Expected search results, got none")
		}
	})

	// Test: Search with filters
	t.Run("SearchWithFilters", func(t *testing.T) {
		filters := map[string]interface{}{
			"language": "go",
		}

		results, err := qdrantDB.Search(ctx, "programming", 10, filters)
		if err != nil {
			t.Fatalf("Failed to search with filters: %v", err)
		}

		if len(results) == 0 {
			t.Error("Expected filtered results, got none")
		}

		// Verify all results match filter
		for _, result := range results {
			if lang, ok := result.Document.Metadata["language"].(string); ok {
				if lang != "go" {
					t.Errorf("Expected language 'go', got '%s'", lang)
				}
			}
		}
	})

	// Test: Drop collection
	t.Run("DropCollection", func(t *testing.T) {
		err := qdrantDB.Drop(ctx)
		if err != nil {
			t.Fatalf("Failed to drop collection: %v", err)
		}

		exists, err := qdrantDB.Exists(ctx)
		if err != nil {
			t.Fatalf("Failed to check collection existence: %v", err)
		}
		if exists {
			t.Error("Collection should not exist after drop")
		}
	})
}

func TestQdrantAdvancedFiltering(t *testing.T) {
	ctx := context.Background()

	// Start Qdrant container
	container, host, port, err := setupQdrantContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup Qdrant container: %v", err)
	}
	defer container.Terminate(ctx)

	// Create mock embedder
	mockEmbedder := embedder.NewMockEmbedder(768)

	// Create Qdrant instance
	qdrantDB, err := NewQdrant(QdrantConfig{
		Host:       host,
		Port:       port,
		Collection: "test_advanced",
		Embedder:   mockEmbedder,
		SearchType: vectordb.SearchTypeVector,
		Distance:   vectordb.DistanceCosine,
	})
	if err != nil {
		t.Fatalf("Failed to create Qdrant: %v", err)
	}
	defer qdrantDB.Close()

	// Create collection
	if err := qdrantDB.Create(ctx); err != nil {
		t.Fatalf("Failed to create collection: %v", err)
	}
	defer qdrantDB.Drop(ctx)

	// Insert test documents
	docs := []*document.Document{
		{
			ID:      "1",
			Name:    "Go Programming",
			Content: "Go is a programming language",
			Metadata: map[string]interface{}{
				"category": "programming",
				"year":     2009,
			},
		},
		{
			ID:      "2",
			Name:    "Python Programming",
			Content: "Python is a programming language",
			Metadata: map[string]interface{}{
				"category": "programming",
				"year":     1991,
			},
		},
		{
			ID:      "3",
			Name:    "Machine Learning",
			Content: "ML is a field of AI",
			Metadata: map[string]interface{}{
				"category": "ai",
				"year":     2010,
			},
		},
	}

	if err := qdrantDB.Insert(ctx, docs, nil); err != nil {
		t.Fatalf("Failed to insert documents: %v", err)
	}

	// Test: Advanced filtering with Must condition
	t.Run("AdvancedFilterMust", func(t *testing.T) {
		advFilter := &AdvancedFilter{
			Must: []FilterCondition{
				{
					Field:    "category",
					Operator: FilterOpEqual,
					Value:    "programming",
				},
			},
		}

		results, err := qdrantDB.SearchWithAdvancedFilters(ctx, "programming", 10, advFilter)
		if err != nil {
			t.Fatalf("Failed to search with advanced filters: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(results))
		}
	})

	// Test: Batch operations
	t.Run("BatchUpsert", func(t *testing.T) {
		newDocs := []*document.Document{
			{
				ID:      "4",
				Name:    "Rust Programming",
				Content: "Rust is a systems programming language",
				Metadata: map[string]interface{}{
					"category": "programming",
					"year":     2010,
				},
			},
			{
				ID:      "5",
				Name:    "Deep Learning",
				Content: "Deep learning is a subset of ML",
				Metadata: map[string]interface{}{
					"category": "ai",
					"year":     2012,
				},
			},
		}

		err := qdrantDB.BatchUpsert(ctx, newDocs, 2, nil)
		if err != nil {
			t.Fatalf("Failed to batch upsert: %v", err)
		}

		count, err := qdrantDB.GetCount(ctx)
		if err != nil {
			t.Fatalf("Failed to get count: %v", err)
		}
		if count != 5 {
			t.Errorf("Expected 5 documents after batch upsert, got %d", count)
		}
	})

	// Test: Update payload
	t.Run("UpdatePayload", func(t *testing.T) {
		filters := map[string]interface{}{
			"category": "programming",
		}

		payload := map[string]interface{}{
			"updated": true,
		}

		err := qdrantDB.UpdatePayload(ctx, filters, payload)
		if err != nil {
			t.Fatalf("Failed to update payload: %v", err)
		}

		// Verify update
		results, err := qdrantDB.Search(ctx, "programming", 10, filters)
		if err != nil {
			t.Fatalf("Failed to search after update: %v", err)
		}

		for _, result := range results {
			if updated, ok := result.Document.Metadata["updated"].(bool); !ok || !updated {
				t.Error("Expected updated=true in metadata")
			}
		}
	})

	// Test: Delete by filter
	t.Run("DeleteByFilter", func(t *testing.T) {
		filters := map[string]interface{}{
			"category": "ai",
		}

		err := qdrantDB.DeleteByFilter(ctx, filters)
		if err != nil {
			t.Fatalf("Failed to delete by filter: %v", err)
		}

		// Verify deletion
		count, err := qdrantDB.GetCount(ctx)
		if err != nil {
			t.Fatalf("Failed to get count: %v", err)
		}

		// Should have only programming documents left
		if count > 3 {
			t.Errorf("Expected <= 3 documents after deletion, got %d", count)
		}
	})
}

func TestQdrantBatchSearch(t *testing.T) {
	ctx := context.Background()

	// Start Qdrant container
	container, host, port, err := setupQdrantContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup Qdrant container: %v", err)
	}
	defer container.Terminate(ctx)

	// Create mock embedder
	mockEmbedder := embedder.NewMockEmbedder(768)

	// Create Qdrant instance
	qdrantDB, err := NewQdrant(QdrantConfig{
		Host:       host,
		Port:       port,
		Collection: "test_batch",
		Embedder:   mockEmbedder,
		SearchType: vectordb.SearchTypeVector,
		Distance:   vectordb.DistanceCosine,
	})
	if err != nil {
		t.Fatalf("Failed to create Qdrant: %v", err)
	}
	defer qdrantDB.Close()

	// Create collection and insert documents
	if err := qdrantDB.Create(ctx); err != nil {
		t.Fatalf("Failed to create collection: %v", err)
	}
	defer qdrantDB.Drop(ctx)

	docs := []*document.Document{
		{
			ID:      "1",
			Name:    "Programming",
			Content: "Programming languages",
		},
		{
			ID:      "2",
			Name:    "AI",
			Content: "Artificial intelligence",
		},
		{
			ID:      "3",
			Name:    "Database",
			Content: "Database systems",
		},
	}

	if err := qdrantDB.Insert(ctx, docs, nil); err != nil {
		t.Fatalf("Failed to insert documents: %v", err)
	}

	// Test batch search
	t.Run("BatchSearch", func(t *testing.T) {
		queries := []string{
			"programming",
			"artificial intelligence",
			"database",
		}

		results, err := qdrantDB.BatchSearch(ctx, queries, 2, nil)
		if err != nil {
			t.Fatalf("Failed to batch search: %v", err)
		}

		if len(results) != len(queries) {
			t.Errorf("Expected %d result sets, got %d", len(queries), len(results))
		}

		for i, resultSet := range results {
			if len(resultSet) == 0 {
				t.Errorf("Query %d returned no results", i)
			}
		}
	})
}
