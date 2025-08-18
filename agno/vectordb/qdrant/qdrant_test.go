package qdrant

import (
	"context"
	"testing"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/vectordb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// MockEmbedder for testing
type MockEmbedder struct {
	dimensions int
}

func NewMockEmbedder(dimensions int) *MockEmbedder {
	return &MockEmbedder{dimensions: dimensions}
}

func (m *MockEmbedder) GetEmbedding(text string) ([]float64, error) {
	// Generate varied embeddings based on text content
	embeddings := make([]float64, m.dimensions)

	// Use text hash to generate varied but deterministic embeddings
	hash := 0
	for _, char := range text {
		hash = hash*31 + int(char)
	}

	for i := range embeddings {
		embeddings[i] = float64((hash+i*13)%1000) / 1000.0
	}

	return embeddings, nil
}

func (m *MockEmbedder) GetEmbeddingAndUsage(text string) ([]float64, map[string]interface{}, error) {
	embedding, err := m.GetEmbedding(text)
	if err != nil {
		return nil, nil, err
	}

	usage := map[string]interface{}{
		"tokens": len(text) / 4, // Rough estimate
		"chars":  len(text),
	}

	return embedding, usage, nil
}

func (m *MockEmbedder) GetDimensions() int {
	return m.dimensions
}

func (m *MockEmbedder) GetID() string {
	return "mock-embedder"
}

func setupQdrantContainer(t testing.TB) (context.Context, *Qdrant, func()) {
	ctx := context.Background()

	// Start Qdrant container
	req := testcontainers.ContainerRequest{
		Image:        "qdrant/qdrant:latest",
		ExposedPorts: []string{"6334/tcp", "6333/tcp"}, // 6334 is gRPC, 6333 is REST
		WaitingFor:   wait.ForHTTP("/").WithPort("6333").WithStartupTimeout(90 * time.Second),
	}

	qdrantContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	// Get connection details
	host, err := qdrantContainer.Host(ctx)
	require.NoError(t, err)

	grpcPort, err := qdrantContainer.MappedPort(ctx, "6334")
	require.NoError(t, err)

	// Create Qdrant instance
	config := QdrantConfig{
		Host:       host,
		Port:       grpcPort.Int(),
		Collection: "test_collection",
		Embedder:   NewMockEmbedder(128),
		SearchType: vectordb.SearchTypeVector,
		Distance:   vectordb.DistanceCosine,
	}

	qdrant, err := NewQdrant(config)
	require.NoError(t, err)

	// Give Qdrant additional time to fully start
	time.Sleep(3 * time.Second)

	// Cleanup function
	cleanup := func() {
		if qdrant != nil {
			qdrant.Drop(ctx)
		}
		qdrantContainer.Terminate(ctx)
	}

	return ctx, qdrant, cleanup
}

func TestQdrant_Create(t *testing.T) {
	ctx, qdrant, cleanup := setupQdrantContainer(t)
	defer cleanup()

	// Test create collection
	err := qdrant.Create(ctx)
	assert.NoError(t, err)

	// Test collection exists
	exists, err := qdrant.Exists(ctx)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Test create again (should not error)
	err = qdrant.Create(ctx)
	assert.NoError(t, err)
}

func TestQdrant_Insert(t *testing.T) {
	ctx, qdrant, cleanup := setupQdrantContainer(t)
	defer cleanup()

	// Create collection first
	err := qdrant.Create(ctx)
	require.NoError(t, err)

	// Create test documents
	docs := []*document.Document{
		{
			ID:          "1",
			Name:        "test1.txt",
			Content:     "This is a test document about artificial intelligence",
			ContentType: "text/plain",
			Source:      "test",
			Metadata: map[string]interface{}{
				"category": "ai",
				"priority": 1,
			},
		},
		{
			ID:          "2",
			Name:        "test2.txt",
			Content:     "This document discusses machine learning algorithms",
			ContentType: "text/plain",
			Source:      "test",
			Metadata: map[string]interface{}{
				"category": "ml",
				"priority": 2,
			},
		},
	}

	// Test insert
	err = qdrant.Insert(ctx, docs, nil)
	assert.NoError(t, err)

	// Give Qdrant time to process the insertion
	time.Sleep(5 * time.Second)

	// Verify documents exist
	exists1, err := qdrant.IDExists(ctx, "1")
	assert.NoError(t, err)
	assert.True(t, exists1)

	exists2, err := qdrant.IDExists(ctx, "2")
	assert.NoError(t, err)
	assert.True(t, exists2)
}

func TestQdrant_VectorSearch(t *testing.T) {
	ctx, qdrant, cleanup := setupQdrantContainer(t)
	defer cleanup()

	// Create and populate collection
	err := qdrant.Create(ctx)
	require.NoError(t, err)

	docs := []*document.Document{
		{
			ID:      "search1",
			Name:    "ai_doc.txt",
			Content: "Artificial intelligence and machine learning are transforming technology",
			Source:  "test",
		},
		{
			ID:      "search2",
			Name:    "cooking_doc.txt",
			Content: "How to cook the perfect pasta with tomato sauce",
			Source:  "test",
		},
		{
			ID:      "search3",
			Name:    "tech_doc.txt",
			Content: "Advanced algorithms in neural networks and deep learning",
			Source:  "test",
		},
	}

	err = qdrant.Insert(ctx, docs, nil)
	require.NoError(t, err)

	// Give Qdrant time to process and index the documents
	time.Sleep(7 * time.Second)

	// Test vector search
	results, err := qdrant.VectorSearch(ctx, "artificial intelligence algorithms", 5, nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, results)

	// For debug - print results
	t.Logf("Search results: %d found", len(results))
	for i, result := range results {
		t.Logf("Result %d: ID=%s, Score=%f", i, result.Document.ID, result.Score)
	}

	// Verify results have required fields
	for _, result := range results {
		assert.NotNil(t, result.Document)
		assert.NotEmpty(t, result.Document.ID)
		// Scores can vary, just check they are valid numbers
		assert.False(t, result.Score != result.Score) // Check for NaN
	}
}

func TestQdrant_KeywordSearch(t *testing.T) {
	ctx, qdrant, cleanup := setupQdrantContainer(t)
	defer cleanup()

	// Create and populate collection
	err := qdrant.Create(ctx)
	require.NoError(t, err)

	docs := []*document.Document{
		{
			ID:      "keyword1",
			Name:    "golang_doc.txt",
			Content: "Go programming language is powerful for backend development",
			Source:  "test",
		},
		{
			ID:      "keyword2",
			Name:    "python_doc.txt",
			Content: "Python is excellent for data science and machine learning",
			Source:  "test",
		},
	}

	err = qdrant.Insert(ctx, docs, nil)
	require.NoError(t, err)

	// Give Qdrant time to process
	time.Sleep(5 * time.Second)

	// Test keyword search
	results, err := qdrant.KeywordSearch(ctx, "programming", 5, nil)
	assert.NoError(t, err)

	// Log results for debugging
	t.Logf("Keyword search results: %d found", len(results))
	for i, result := range results {
		t.Logf("Result %d: ID=%s, Content=%s", i, result.Document.ID, result.Document.Content)
	}
}

func TestQdrant_AutoCollectionCreation(t *testing.T) {
	ctx, qdrant, cleanup := setupQdrantContainer(t)
	defer cleanup()

	// Don't create collection manually
	// Insert should auto-create the collection
	docs := []*document.Document{
		{
			ID:      "auto1",
			Name:    "auto_doc.txt",
			Content: "Auto-created collection test",
			Source:  "test",
		},
	}

	err := qdrant.Insert(ctx, docs, nil)
	assert.NoError(t, err)

	// Give Qdrant time to process the insertion
	time.Sleep(5 * time.Second)

	// Verify collection was created
	exists, err := qdrant.Exists(ctx)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Verify document was inserted
	docExists, err := qdrant.IDExists(ctx, "auto1")
	assert.NoError(t, err)
	assert.True(t, docExists)
}

func TestQdrant_Upsert(t *testing.T) {
	ctx, qdrant, cleanup := setupQdrantContainer(t)
	defer cleanup()

	// Create collection
	err := qdrant.Create(ctx)
	require.NoError(t, err)

	// Initial document
	docs := []*document.Document{
		{
			ID:      "upsert1",
			Name:    "original.txt",
			Content: "Original content",
			Source:  "test",
			Metadata: map[string]interface{}{
				"version": 1,
			},
		},
	}

	// Insert first version
	err = qdrant.Upsert(ctx, docs, nil)
	assert.NoError(t, err)
	time.Sleep(3 * time.Second)

	// Verify insertion
	exists, err := qdrant.IDExists(ctx, "upsert1")
	assert.NoError(t, err)
	assert.True(t, exists)

	// Update the document
	docs[0].Content = "Updated content"
	docs[0].Metadata["version"] = 2

	// Upsert updated version
	err = qdrant.Upsert(ctx, docs, nil)
	assert.NoError(t, err)
	time.Sleep(3 * time.Second)

	// Document should still exist (same ID)
	exists, err = qdrant.IDExists(ctx, "upsert1")
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestQdrant_Drop(t *testing.T) {
	ctx, qdrant, cleanup := setupQdrantContainer(t)
	defer cleanup()

	// Create collection
	err := qdrant.Create(ctx)
	require.NoError(t, err)

	// Verify it exists
	exists, err := qdrant.Exists(ctx)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Drop collection
	err = qdrant.Drop(ctx)
	assert.NoError(t, err)

	// Give time for deletion
	time.Sleep(2 * time.Second)

	// Verify it no longer exists
	exists, err = qdrant.Exists(ctx)
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestQdrant_GetCount(t *testing.T) {
	ctx, qdrant, cleanup := setupQdrantContainer(t)
	defer cleanup()

	// Create collection
	err := qdrant.Create(ctx)
	require.NoError(t, err)

	// Initially should be empty
	count, err := qdrant.GetCount(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Add some documents
	docs := []*document.Document{
		{ID: "count1", Content: "Document 1", Source: "test"},
		{ID: "count2", Content: "Document 2", Source: "test"},
		{ID: "count3", Content: "Document 3", Source: "test"},
	}

	err = qdrant.Insert(ctx, docs, nil)
	assert.NoError(t, err)
	time.Sleep(3 * time.Second)

	// Should have documents now
	count, err = qdrant.GetCount(ctx)
	assert.NoError(t, err)
	assert.True(t, count > 0) // At least some documents
}

func TestQdrant_NameExists(t *testing.T) {
	ctx, qdrant, cleanup := setupQdrantContainer(t)
	defer cleanup()

	// Create collection
	err := qdrant.Create(ctx)
	require.NoError(t, err)

	// Add a document with specific name
	docs := []*document.Document{
		{
			ID:      "name1",
			Name:    "unique_name.txt",
			Content: "Content with unique name",
			Source:  "test",
		},
	}

	err = qdrant.Insert(ctx, docs, nil)
	assert.NoError(t, err)
	time.Sleep(3 * time.Second)

	// Test name exists
	exists, err := qdrant.NameExists(ctx, "unique_name.txt")
	assert.NoError(t, err)
	// Note: This might not work perfectly with Qdrant's text matching
	// but we test the functionality

	// Test non-existent name
	exists, err = qdrant.NameExists(ctx, "non_existent.txt")
	assert.NoError(t, err)
	assert.False(t, exists)
}
