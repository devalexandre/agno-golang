package pgvector

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/vectordb"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// MockEmbedder for testing
type MockEmbedder struct {
	dimensions int
}

func (m *MockEmbedder) GetEmbedding(text string) ([]float64, error) {
	// Generate more varied embeddings based on text content and length
	embedding := make([]float64, m.dimensions)

	// Use text length as base
	baseValue := float64(len(text)) / 100.0

	// Add variation based on text content
	for i := range embedding {
		// Create variation based on character at position and index
		charValue := 0.5
		if i < len(text) {
			charValue = float64(text[i]) / 255.0
		}

		// Combine base value with character-based variation and position
		positionFactor := float64(i) / float64(m.dimensions)
		embedding[i] = (baseValue + charValue + positionFactor) / 3.0

		// Normalize to reasonable range
		if embedding[i] > 1.0 {
			embedding[i] = 1.0
		}
	}
	return embedding, nil
}

func (m *MockEmbedder) GetEmbeddingAndUsage(text string) ([]float64, map[string]interface{}, error) {
	embedding, err := m.GetEmbedding(text)
	usage := map[string]interface{}{
		"prompt_tokens": len(text),
		"total_tokens":  len(text),
	}
	return embedding, usage, err
}

func (m *MockEmbedder) GetDimensions() int {
	return m.dimensions
}

func (m *MockEmbedder) GetID() string {
	return "mock-embedder"
}

func setupPgVectorContainer(tb testing.TB) (*postgres.PostgresContainer, *PgVector, func()) {
	ctx := context.Background()

	// Start PostgreSQL container with pgvector extension
	pgContainer, err := postgres.Run(ctx,
		"pgvector/pgvector:pg16",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		tb.Fatalf("Failed to start PostgreSQL container: %v", err)
	}

	// Get connection string
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		tb.Fatalf("Failed to get connection string: %v", err)
	}

	// Create PgVector instance
	mockEmbedder := &MockEmbedder{dimensions: 128}
	pgVector, err := NewPgVector(PgVectorConfig{
		ConnectionString: connStr,
		TableName:        "test_documents",
		Schema:           "public",
		Embedder:         mockEmbedder,
		SearchType:       vectordb.SearchTypeVector,
		Distance:         vectordb.DistanceCosine,
	})
	if err != nil {
		tb.Fatalf("Failed to create PgVector instance: %v", err)
	}

	// Cleanup function
	cleanup := func() {
		if pgVector != nil {
			pgVector.Close()
		}
		if err := pgContainer.Terminate(ctx); err != nil {
			tb.Logf("Failed to terminate container: %v", err)
		}
	}

	return pgContainer, pgVector, cleanup
}

func TestPgVectorWithTestcontainers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	_, pgVector, cleanup := setupPgVectorContainer(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Create Table", func(t *testing.T) {
		err := pgVector.Create(ctx)
		if err != nil {
			t.Fatalf("Failed to create table: %v", err)
		}

		exists, err := pgVector.Exists(ctx)
		if err != nil {
			t.Fatalf("Failed to check table existence: %v", err)
		}

		if !exists {
			t.Fatal("Table should exist after creation")
		}
	})

	t.Run("Insert and Search Documents", func(t *testing.T) {
		// Create test documents
		docs := []*document.Document{
			{
				ID:      "doc1",
				Name:    "Test Document 1",
				Content: "This is a test document about artificial intelligence and machine learning",
				Metadata: map[string]interface{}{
					"category": "AI",
					"priority": "high",
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			{
				ID:      "doc2",
				Name:    "Test Document 2",
				Content: "This document discusses deep learning algorithms and neural networks",
				Metadata: map[string]interface{}{
					"category": "ML",
					"priority": "medium",
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			{
				ID:      "doc3",
				Name:    "Test Document 3",
				Content: "Natural language processing and computer vision are important AI fields",
				Metadata: map[string]interface{}{
					"category": "NLP",
					"priority": "low",
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		}

		// Insert documents
		err := pgVector.Insert(ctx, docs, nil)
		if err != nil {
			t.Fatalf("Failed to insert documents: %v", err)
		}

		// Test vector search
		results, err := pgVector.VectorSearch(ctx, "artificial intelligence", 5, nil)
		if err != nil {
			t.Fatalf("Failed to search documents: %v", err)
		}

		if len(results) == 0 {
			t.Fatal("Expected search results, got none")
		}

		t.Logf("Found %d results", len(results))
		for i, result := range results {
			t.Logf("Result %d: ID=%s, Score=%.4f, Content=%s",
				i+1, result.Document.ID, result.Score, result.Document.Content[:50]+"...")
		}

		// Verify results are sorted by relevance
		for i := 1; i < len(results); i++ {
			if results[i].Score > results[i-1].Score {
				t.Errorf("Results not sorted by score: %f > %f", results[i].Score, results[i-1].Score)
			}
		}
	})

	t.Run("Search with Filters", func(t *testing.T) {
		// Search with category filter
		filters := map[string]interface{}{
			"category": "AI",
		}

		results, err := pgVector.VectorSearch(ctx, "intelligence", 5, filters)
		if err != nil {
			t.Fatalf("Failed to search with filters: %v", err)
		}

		if len(results) == 0 {
			t.Fatal("Expected filtered search results, got none")
		}

		// Verify all results match the filter
		for _, result := range results {
			if category, ok := result.Document.Metadata["category"]; !ok || category != "AI" {
				t.Errorf("Result doesn't match filter: got category=%v, want=AI", category)
			}
		}
	})

	t.Run("Keyword Search", func(t *testing.T) {
		results, err := pgVector.KeywordSearch(ctx, "neural networks", 5, nil)
		if err != nil {
			t.Fatalf("Failed to perform keyword search: %v", err)
		}

		if len(results) == 0 {
			t.Fatal("Expected keyword search results, got none")
		}

		t.Logf("Keyword search found %d results", len(results))
		for i, result := range results {
			t.Logf("Keyword Result %d: ID=%s, Score=%.4f",
				i+1, result.Document.ID, result.Score)
		}
	})

	t.Run("Hybrid Search", func(t *testing.T) {
		results, err := pgVector.HybridSearch(ctx, "machine learning", 3, nil)
		if err != nil {
			t.Fatalf("Failed to perform hybrid search: %v", err)
		}

		if len(results) == 0 {
			t.Fatal("Expected hybrid search results, got none")
		}

		t.Logf("Hybrid search found %d results", len(results))
		for i, result := range results {
			t.Logf("Hybrid Result %d: ID=%s, Score=%.4f",
				i+1, result.Document.ID, result.Score)
		}
	})

	t.Run("Upsert Documents", func(t *testing.T) {
		// Create a document that should replace an existing one
		docs := []*document.Document{
			{
				ID:      "doc1", // Same ID as existing document
				Name:    "Updated Test Document 1",
				Content: "This is an updated test document about advanced artificial intelligence",
				Metadata: map[string]interface{}{
					"category": "AI",
					"priority": "critical",
					"updated":  true,
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		}

		// Upsert document
		err := pgVector.Upsert(ctx, docs, nil)
		if err != nil {
			t.Fatalf("Failed to upsert document: %v", err)
		}

		// Verify the document was updated
		results, err := pgVector.VectorSearch(ctx, "advanced artificial intelligence", 1, nil)
		if err != nil {
			t.Fatalf("Failed to search for updated document: %v", err)
		}

		if len(results) == 0 {
			t.Fatal("Expected to find updated document")
		}

		doc := results[0].Document
		if doc.Name != "Updated Test Document 1" {
			t.Errorf("Document not updated: got name=%s, want=Updated Test Document 1", doc.Name)
		}

		if priority, ok := doc.Metadata["priority"]; !ok || priority != "critical" {
			t.Errorf("Document metadata not updated: got priority=%v, want=critical", priority)
		}
	})

	t.Run("Get Count", func(t *testing.T) {
		count, err := pgVector.GetCount(ctx)
		if err != nil {
			t.Fatalf("Failed to get count: %v", err)
		}

		if count != 3 {
			t.Errorf("Expected count=3, got=%d", count)
		}
	})

	t.Run("Document Existence Checks", func(t *testing.T) {
		// Test ID exists
		exists, err := pgVector.IDExists(ctx, "doc1")
		if err != nil {
			t.Fatalf("Failed to check ID existence: %v", err)
		}
		if !exists {
			t.Error("Document doc1 should exist")
		}

		// Test ID doesn't exist
		exists, err = pgVector.IDExists(ctx, "nonexistent")
		if err != nil {
			t.Fatalf("Failed to check ID existence: %v", err)
		}
		if exists {
			t.Error("Document nonexistent should not exist")
		}

		// Test document exists
		doc := &document.Document{ID: "doc2"}
		exists, err = pgVector.DocExists(ctx, doc)
		if err != nil {
			t.Fatalf("Failed to check document existence: %v", err)
		}
		if !exists {
			t.Error("Document doc2 should exist")
		}

		// Test name exists
		exists, err = pgVector.NameExists(ctx, "Test Document 2")
		if err != nil {
			t.Fatalf("Failed to check name existence: %v", err)
		}
		if !exists {
			t.Error("Document with name 'Test Document 2' should exist")
		}
	})

	t.Run("Optimize", func(t *testing.T) {
		err := pgVector.Optimize(ctx)
		if err != nil {
			t.Fatalf("Failed to optimize: %v", err)
		}
	})

	t.Run("Drop Table", func(t *testing.T) {
		err := pgVector.Drop(ctx)
		if err != nil {
			t.Fatalf("Failed to drop table: %v", err)
		}

		exists, err := pgVector.Exists(ctx)
		if err != nil {
			t.Fatalf("Failed to check table existence after drop: %v", err)
		}

		if exists {
			t.Error("Table should not exist after drop")
		}
	})
}

func TestPgVectorDifferentDistanceMetrics(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	distances := []vectordb.Distance{
		vectordb.DistanceCosine,
		vectordb.DistanceL2,
		vectordb.DistanceMaxInnerProduct,
	}

	for _, distance := range distances {
		t.Run(fmt.Sprintf("Distance_%s", distance), func(t *testing.T) {
			_, pgVector, cleanup := setupPgVectorContainer(t)
			defer cleanup()

			// Update distance metric
			pgVector.Distance = distance

			// Create table
			err := pgVector.Create(ctx)
			if err != nil {
				t.Fatalf("Failed to create table with %s distance: %v", distance, err)
			}

			// Insert test documents
			docs := []*document.Document{
				{
					ID:        "test1",
					Content:   "Test document for distance metric",
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
			}

			err = pgVector.Insert(ctx, docs, nil)
			if err != nil {
				t.Fatalf("Failed to insert documents with %s distance: %v", distance, err)
			}

			// Test search
			results, err := pgVector.VectorSearch(ctx, "test document", 1, nil)
			if err != nil {
				t.Fatalf("Failed to search with %s distance: %v", distance, err)
			}

			if len(results) == 0 {
				t.Fatalf("Expected search results with %s distance", distance)
			}

			t.Logf("Distance %s: Score=%.4f, Distance=%.4f",
				distance, results[0].Score, results[0].Distance)
		})
	}
}

func BenchmarkPgVectorOperations(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	_, pgVector, cleanup := setupPgVectorContainer(b)
	defer cleanup()

	ctx := context.Background()

	// Setup
	err := pgVector.Create(ctx)
	if err != nil {
		b.Fatalf("Failed to create table: %v", err)
	}

	// Insert some test data
	docs := make([]*document.Document, 100)
	for i := 0; i < 100; i++ {
		docs[i] = &document.Document{
			ID:      fmt.Sprintf("doc%d", i),
			Name:    fmt.Sprintf("Document %d", i),
			Content: fmt.Sprintf("This is test document number %d with some content for benchmarking performance", i),
			Metadata: map[string]interface{}{
				"index": i,
				"type":  "benchmark",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	}

	err = pgVector.Insert(ctx, docs, nil)
	if err != nil {
		b.Fatalf("Failed to insert benchmark data: %v", err)
	}

	b.ResetTimer()

	b.Run("VectorSearch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := pgVector.VectorSearch(ctx, "test document", 10, nil)
			if err != nil {
				b.Fatalf("Search failed: %v", err)
			}
		}
	})

	b.Run("KeywordSearch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := pgVector.KeywordSearch(ctx, "document content", 10, nil)
			if err != nil {
				b.Fatalf("Keyword search failed: %v", err)
			}
		}
	})

	b.Run("HybridSearch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := pgVector.HybridSearch(ctx, "test document", 10, nil)
			if err != nil {
				b.Fatalf("Hybrid search failed: %v", err)
			}
		}
	})

	b.Run("Insert", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			doc := &document.Document{
				ID:      fmt.Sprintf("bench_doc_%d", i),
				Content: fmt.Sprintf("Benchmark document %d", i),
			}
			err := pgVector.Insert(ctx, []*document.Document{doc}, nil)
			if err != nil {
				b.Fatalf("Insert failed: %v", err)
			}
		}
	})

	b.Run("Upsert", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			doc := &document.Document{
				ID:      fmt.Sprintf("upsert_doc_%d", i),
				Content: fmt.Sprintf("Upsert document %d", i),
			}
			err := pgVector.Upsert(ctx, []*document.Document{doc}, nil)
			if err != nil {
				b.Fatalf("Upsert failed: %v", err)
			}
		}
	})
}
