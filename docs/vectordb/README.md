# 🔍 Vector Database Module

The Vector Database module provides a unified interface for vector storage and similarity search operations in Agno-Golang, with native support for multiple vector database backends.

## 🚀 Features

### ✅ Core Capabilities
- **Unified Interface**: Single `VectorDB` interface for all vector database backends
- **Native Compatibility**: Direct integration without adapters
- **Multiple Backends**: Qdrant, Chroma, and extensible architecture
- **Advanced Search**: Vector similarity, hybrid search, and filtering
- **Collection Management**: Create, drop, and manage collections
- **Metadata Support**: Rich metadata storage and filtering
- **Performance Optimized**: Efficient batch operations and connection pooling

## 🔧 Architecture

```
┌─────────────────────┐    ┌──────────────────┐    ┌─────────────────────┐
│   Knowledge Base    │───▶│   VectorDB       │───▶│   Backend           │
│                     │    │   Interface      │    │   (Qdrant/Chroma)   │
└─────────────────────┘    └──────────────────┘    └─────────────────────┘
                                    │
                           ┌──────────────────┐
                           │   Documents      │
                           │   + Embeddings   │
                           └──────────────────┘
```

## 📖 Interface Definition

### Core VectorDB Interface
```go
type VectorDB interface {
    // Collection management
    Create(ctx context.Context) error
    Drop(ctx context.Context) error
    Exists(ctx context.Context) (bool, error)
    
    // Document operations
    Add(ctx context.Context, documents []document.Document) error
    Delete(ctx context.Context, documentIDs []string) error
    Update(ctx context.Context, documents []document.Document) error
    
    // Search operations
    Search(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*SearchResult, error)
    SearchByVector(ctx context.Context, vector []float64, limit int, filters map[string]interface{}) ([]*SearchResult, error)
    
    // Utility methods
    GetCount(ctx context.Context) (int64, error)
    GetInfo() VectorDBInfo
}
```

### Search Result Structure
```go
type SearchResult struct {
    ID       string             `json:"id"`
    Score    float64            `json:"score"`
    Document *document.Document `json:"document"`
    Metadata map[string]interface{} `json:"metadata"`
}
```

## 🗄️ Qdrant Implementation

### Configuration
```go
type QdrantConfig struct {
    Host       string
    Port       int
    Collection string
    Embedder   embedder.Embedder
    SearchType SearchType
    Distance   DistanceMetric
    ApiKey     string     // Optional for Qdrant Cloud
    Secure     bool       // Enable HTTPS
    Timeout    time.Duration
}
```

### Usage Example
```go
package main

import (
    "context"
    "github.com/devalexandre/agno-golang/agno/vectordb/qdrant"
    "github.com/devalexandre/agno-golang/agno/embedder"
)

func main() {
    ctx := context.Background()
    
    // Setup embedder
    embedder := embedder.NewOpenAIEmbedder()
    
    // Configure Qdrant
    config := qdrant.QdrantConfig{
        Host:       "localhost",
        Port:       6334, // gRPC port
        Collection: "my-collection",
        Embedder:   embedder,
        SearchType: vectordb.SearchTypeVector,
        Distance:   vectordb.DistanceCosine,
        Timeout:    30 * time.Second,
    }
    
    // Create vector database
    vectorDB, err := qdrant.NewQdrant(config)
    if err != nil {
        panic(err)
    }
    
    // Create collection
    err = vectorDB.Create(ctx)
    if err != nil {
        panic(err)
    }
    
    // Add documents
    documents := []document.Document{
        {
            ID:      "doc1",
            Content: "This is a sample document about AI",
            Metadata: map[string]interface{}{
                "category": "AI",
                "author":   "John Doe",
            },
        },
    }
    
    err = vectorDB.Add(ctx, documents)
    if err != nil {
        panic(err)
    }
    
    // Search
    results, err := vectorDB.Search(ctx, "artificial intelligence", 5, nil)
    if err != nil {
        panic(err)
    }
    
    for _, result := range results {
        fmt.Printf("Score: %.2f - %s\n", result.Score, result.Document.Content)
    }
}
```

## 🔍 Search Types

### Vector Search
```go
// Semantic similarity search
results, err := vectorDB.Search(ctx, "machine learning concepts", 10, nil)
```

### Filtered Search
```go
// Search with metadata filters
filters := map[string]interface{}{
    "category": "AI",
    "year": map[string]interface{}{
        "gte": 2020,
    },
}

results, err := vectorDB.Search(ctx, "neural networks", 5, filters)
```

### Vector-Based Search
```go
// Search using pre-computed embeddings
queryVector := []float64{0.1, 0.2, 0.3, ...} // 1536 dimensions for OpenAI
results, err := vectorDB.SearchByVector(ctx, queryVector, 10, nil)
```

## 📊 Distance Metrics

### Available Metrics
```go
const (
    DistanceCosine    DistanceMetric = "cosine"     // Recommended for text
    DistanceEuclidean DistanceMetric = "euclidean"  // L2 distance
    DistanceDotProduct DistanceMetric = "dot"       // Dot product similarity
    DistanceManhattan DistanceMetric = "manhattan"  // L1 distance
)
```

### Usage Recommendations
- **Cosine**: Best for text embeddings (OpenAI, Sentence Transformers)
- **Euclidean**: Good for normalized vectors
- **Dot Product**: Fast for normalized vectors
- **Manhattan**: Robust to outliers

## 🔧 Collection Management

### Create Collection
```go
// Create with default settings
err := vectorDB.Create(ctx)

// Check if collection exists first
exists, err := vectorDB.Exists(ctx)
if !exists {
    err = vectorDB.Create(ctx)
}
```

### Drop Collection
```go
// Remove entire collection and all data
err := vectorDB.Drop(ctx)
```

### Collection Information
```go
info := vectorDB.GetInfo()
fmt.Printf("Collection: %s\n", info.Collection)
fmt.Printf("Backend: %s\n", info.Backend)
fmt.Printf("Vector Size: %d\n", info.VectorSize)
```

## 📈 Batch Operations

### Batch Add
```go
// Add multiple documents efficiently
documents := make([]document.Document, 1000)
for i := 0; i < 1000; i++ {
    documents[i] = document.Document{
        ID:      fmt.Sprintf("doc-%d", i),
        Content: fmt.Sprintf("Document content %d", i),
    }
}

err := vectorDB.Add(ctx, documents)
```

### Batch Update
```go
// Update existing documents
updatedDocs := []document.Document{
    {
        ID:      "doc1",
        Content: "Updated content for document 1",
        Metadata: map[string]interface{}{
            "updated_at": time.Now(),
        },
    },
}

err := vectorDB.Update(ctx, updatedDocs)
```

### Batch Delete
```go
// Delete multiple documents by ID
documentIDs := []string{"doc1", "doc2", "doc3"}
err := vectorDB.Delete(ctx, documentIDs)
```

## 🚀 Performance Optimization

### Connection Pooling
```go
config := qdrant.QdrantConfig{
    Host:       "localhost",
    Port:       6334,
    Collection: "my-collection",
    Embedder:   embedder,
    Timeout:    30 * time.Second,
    // Connection pool settings
    MaxConnections: 10,
    IdleTimeout:    5 * time.Minute,
}
```

### Batch Size Optimization
```go
// Optimal batch sizes for different operations
const (
    OptimalBatchSize = 100    // For most operations
    LargeBatchSize   = 500    // For bulk insertions
    SmallBatchSize   = 50     // For high-frequency updates
)
```

### Async Operations
```go
// Process documents in background
go func() {
    err := vectorDB.Add(ctx, documents)
    if err != nil {
        log.Printf("Error adding documents: %v", err)
    }
}()
```

## 🔧 Configuration Examples

### Local Development
```go
config := qdrant.QdrantConfig{
    Host:       "localhost",
    Port:       6334,
    Collection: "dev-collection",
    Embedder:   embedder.NewOpenAIEmbedder(),
    SearchType: vectordb.SearchTypeVector,
    Distance:   vectordb.DistanceCosine,
}
```

### Production with Authentication
```go
config := qdrant.QdrantConfig{
    Host:       "qdrant.example.com",
    Port:       443,
    Collection: "prod-collection",
    Embedder:   embedder.NewOpenAIEmbedder(),
    SearchType: vectordb.SearchTypeVector,
    Distance:   vectordb.DistanceCosine,
    ApiKey:     os.Getenv("QDRANT_API_KEY"),
    Secure:     true,
    Timeout:    60 * time.Second,
}
```

### High-Performance Setup
```go
config := qdrant.QdrantConfig{
    Host:       "localhost",
    Port:       6334,
    Collection: "high-perf",
    Embedder:   embedder.NewOpenAIEmbedder(),
    SearchType: vectordb.SearchTypeVector,
    Distance:   vectordb.DistanceCosine,
    Timeout:    10 * time.Second,
    MaxConnections: 20,
    BatchSize:     500,
}
```

## 🧪 Testing

### Unit Tests
```go
func TestVectorDB(t *testing.T) {
    ctx := context.Background()
    
    // Setup test vector database
    vectorDB := setupTestVectorDB(t)
    
    // Test document addition
    docs := []document.Document{
        {ID: "test1", Content: "Test document"},
    }
    
    err := vectorDB.Add(ctx, docs)
    assert.NoError(t, err)
    
    // Test search
    results, err := vectorDB.Search(ctx, "test", 1, nil)
    assert.NoError(t, err)
    assert.Len(t, results, 1)
    assert.Equal(t, "test1", results[0].ID)
}
```

### Integration Tests
```bash
# Run with Qdrant container
docker run -d -p 6333:6333 -p 6334:6334 --name test-qdrant qdrant/qdrant

# Run tests
go test ./agno/vectordb/...

# Cleanup
docker stop test-qdrant && docker rm test-qdrant
```

## 🔧 Troubleshooting

### Common Issues

#### 1. Connection Failures
```go
// Add retry logic
func connectWithRetry(config QdrantConfig) (*QdrantClient, error) {
    for attempts := 0; attempts < 3; attempts++ {
        client, err := qdrant.NewQdrant(config)
        if err == nil {
            return client, nil
        }
        
        time.Sleep(time.Duration(attempts+1) * time.Second)
    }
    return nil, fmt.Errorf("failed to connect after retries")
}
```

#### 2. Performance Issues
```go
// Monitor performance
start := time.Now()
err := vectorDB.Add(ctx, documents)
duration := time.Since(start)

if duration > 10*time.Second {
    log.Printf("Slow operation detected: %v", duration)
}
```

#### 3. Memory Issues
```go
// Process in smaller batches
const batchSize = 100
for i := 0; i < len(documents); i += batchSize {
    end := i + batchSize
    if end > len(documents) {
        end = len(documents)
    }
    
    batch := documents[i:end]
    err := vectorDB.Add(ctx, batch)
    if err != nil {
        return err
    }
}
```

## 🚀 Extending VectorDB

### Custom Backend Implementation
```go
type CustomVectorDB struct {
    config CustomConfig
    client *CustomClient
}

func (c *CustomVectorDB) Create(ctx context.Context) error {
    // Implement collection creation
    return nil
}

func (c *CustomVectorDB) Search(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*SearchResult, error) {
    // Implement search logic
    return nil, nil
}

// Implement other VectorDB interface methods...
```

### Registration
```go
func init() {
    vectordb.RegisterBackend("custom", NewCustomVectorDB)
}
```

---

The Vector Database module provides a robust, performant, and extensible foundation for vector operations in Agno-Golang, supporting multiple backends while maintaining a consistent interface.
