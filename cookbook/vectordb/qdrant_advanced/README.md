# Qdrant Advanced Features Demo

This example demonstrates advanced features of Qdrant vector database integration in Agno-Golang.

## Features Demonstrated

### 1. **Batch Upsert**
- Efficiently insert multiple documents in batches
- Automatic embedding generation
- Configurable batch size for optimal performance

### 2. **Advanced Filtering**
- Complex filter conditions (Must, Should, MustNot)
- Multiple filter operators (Equal, Range, In, etc.)
- Metadata-based filtering

### 3. **Reranking**
- Improve search results with reranking
- Configurable score boosting
- Content similarity-based reranking

### 4. **Batch Search**
- Execute multiple search queries efficiently
- Parallel query processing
- Consistent result format

### 5. **Payload Updates**
- Update document metadata without re-embedding
- Filter-based bulk updates
- Preserve vector embeddings

### 6. **Collection Information**
- Get collection statistics
- Monitor vector dimensions
- Track document count

### 7. **Hybrid Search**
- Combine vector and keyword search
- Weighted result merging
- Best of both search methods

### 8. **Delete by Filter**
- Remove documents based on metadata
- Bulk deletion operations
- Filter-based cleanup

## Prerequisites

1. **Docker** - Required for testcontainers
   ```bash
   # Verify Docker is running
   docker ps
   ```

2. **Ollama Local Server**
   ```bash
   # Install Ollama: https://ollama.ai
   ollama pull gemma:2b
   ollama serve
   ```

3. **Go 1.21+**
   ```bash
   go version
   ```

## Running the Example

```bash
# Make sure Docker and Ollama are running
docker ps
ollama serve  # In another terminal

# Run the example (testcontainers will automatically start Qdrant)
cd cookbook/vectordb/qdrant_advanced
go run main.go
```

## How Testcontainers Works

This example uses testcontainers to automatically manage Qdrant:

```go
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
    // ... returns container, host, and port
}
```

**Benefits:**
- ✅ No manual Qdrant setup required
- ✅ Automatic container startup and cleanup
- ✅ Isolated environment for each run
- ✅ Works in CI/CD pipelines
- ✅ Reproducible results

## Expected Output

```
🚀 Qdrant Advanced Features Demo
==================================================

📦 Creating collection...

1️⃣ Demo: Batch Upsert
--------------------------------------------------
✅ Inserted 5 documents in batches

2️⃣ Demo: Advanced Filtering
--------------------------------------------------
Found 2 results for 'programming languages' with category='programming':
  1. Go Programming (score: 0.8542)
  2. Python Programming (score: 0.8123)

3️⃣ Demo: Search with Reranking
--------------------------------------------------
Top 3 reranked results for 'artificial intelligence and learning':
  1. Machine Learning (score: 0.9234)
  2. Deep Learning (score: 0.9012)
  3. Go Programming (score: 0.6543)

... (more output)
```

## Advanced Filter Examples

### Must Condition (AND logic)
```go
advFilter := &qdrant.AdvancedFilter{
    Must: []qdrant.FilterCondition{
        {
            Field:    "category",
            Operator: qdrant.FilterOpEqual,
            Value:    "programming",
        },
        {
            Field:    "year",
            Operator: qdrant.FilterOpRange,
            Value: map[string]interface{}{
                "gte": 2000.0,
                "lte": 2020.0,
            },
        },
    },
}
```

### Should Condition (OR logic)
```go
advFilter := &qdrant.AdvancedFilter{
    Should: []qdrant.FilterCondition{
        {
            Field:    "category",
            Operator: qdrant.FilterOpEqual,
            Value:    "ai",
        },
        {
            Field:    "category",
            Operator: qdrant.FilterOpEqual,
            Value:    "database",
        },
    },
}
```

### MustNot Condition (NOT logic)
```go
advFilter := &qdrant.AdvancedFilter{
    MustNot: []qdrant.FilterCondition{
        {
            Field:    "category",
            Operator: qdrant.FilterOpEqual,
            Value:    "deprecated",
        },
    },
}
```

### In Operator
```go
advFilter := &qdrant.AdvancedFilter{
    Must: []qdrant.FilterCondition{
        {
            Field:    "language",
            Operator: qdrant.FilterOpIn,
            Value:    []interface{}{"go", "python", "rust"},
        },
    },
}
```

## Reranking Configuration

```go
rerankConfig := &qdrant.RerankingConfig{
    Enabled:    true,
    TopK:       20,        // Fetch top 20 for reranking
    ScoreBoost: 1.5,       // Boost reranked scores by 1.5x
}

results, err := qdrantDB.SearchWithReranking(
    ctx,
    "your query",
    5,  // Return top 5 after reranking
    nil,
    rerankConfig,
)
```

## Batch Operations

### Batch Upsert
```go
// Insert 1000 documents in batches of 100
err := qdrantDB.BatchUpsert(ctx, documents, 100, nil)
```

### Batch Search
```go
queries := []string{
    "query 1",
    "query 2",
    "query 3",
}

results, err := qdrantDB.BatchSearch(ctx, queries, 10, nil)
// results[0] = results for query 1
// results[1] = results for query 2
// results[2] = results for query 3
```

## Payload Operations

### Update Payload
```go
filters := map[string]interface{}{
    "category": "programming",
}

payload := map[string]interface{}{
    "reviewed":   true,
    "updated_at": time.Now().Format(time.RFC3339),
}

err := qdrantDB.UpdatePayload(ctx, filters, payload)
```

### Delete by Filter
```go
filters := map[string]interface{}{
    "status": "archived",
}

err := qdrantDB.DeleteByFilter(ctx, filters)
```

## Hybrid Search

Combines vector similarity search with keyword matching:

```go
results, err := qdrantDB.HybridSearch(ctx, "machine learning", 10, nil)
// Returns results ranked by both semantic similarity and keyword relevance
```

## Performance Tips

1. **Batch Size**: Use appropriate batch sizes (50-200) for bulk operations
2. **Reranking**: Only rerank when needed, as it adds computational overhead
3. **Filters**: Use indexed fields for better filter performance
4. **Hybrid Search**: Best for queries that benefit from both semantic and keyword matching

## Related Documentation

- [Qdrant Documentation](https://qdrant.tech/documentation/)
- [Vector Database Guide](../../../docs/vectordb/README.md)
- [Embedder Documentation](../../../docs/embedder/README.md)

## Implementation Details

### Files
- `agno/vectordb/qdrant/qdrant.go` - Basic Qdrant implementation
- `agno/vectordb/qdrant/advanced.go` - Advanced features (NEW)
- `cookbook/vectordb/qdrant_advanced/main.go` - This example

### New Methods
- `SearchWithAdvancedFilters()` - Complex filtering
- `SearchWithReranking()` - Result reranking
- `BatchSearch()` - Multiple queries
- `BatchUpsert()` - Bulk insertion
- `UpdatePayload()` - Metadata updates
- `DeleteByFilter()` - Bulk deletion
- `GetCollectionInfo()` - Collection stats
- `HybridSearchWithSparse()` - Sparse vector support
- `CalculateRelevanceScore()` - Relevance scoring

## Troubleshooting

### Connection Issues
```bash
# Check if Qdrant is running
curl http://localhost:6333/collections
```

### Ollama Issues
```bash
# Verify Ollama is running
curl http://localhost:11434/api/tags

# Check if model is available
ollama list
```

### Docker Issues
```bash
# Check if Docker is running
docker ps

# Check container logs
docker logs <container_id>
```

### Performance Issues
- Reduce batch size if memory constrained
- Use filters to narrow search space
- Consider collection optimization settings

## Next Steps

- Explore [PgVector integration](../pgvector/)
- Learn about [Knowledge Base](../../agents/knowledge_pdf/)
- Try [RAG implementation](../../agents/knowledge_filters_example/)
