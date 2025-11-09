# PgVector Demo with Testcontainers

This example demonstrates PgVector (PostgreSQL with vector extension) integration using testcontainers for automatic container management.

## Features Demonstrated

### 1. **Automatic Container Management**
- Testcontainers automatically starts PostgreSQL with pgvector
- No manual setup required
- Automatic cleanup after execution

### 2. **Vector Search**
- Semantic similarity search using embeddings
- Cosine distance for similarity calculation
- Efficient vector operations with pgvector extension

### 3. **Metadata Filtering**
- Filter search results by metadata fields
- Combine vector search with traditional filters
- JSON-based metadata storage

### 4. **Hybrid Search**
- Combines vector similarity with keyword matching
- Best of both semantic and lexical search
- Weighted result merging

### 5. **Document Management**
- Insert, update, and delete documents
- Check document existence
- Get document counts

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

# Run the example
cd cookbook/vectordb/pgvector_example
go run main.go
```

## Expected Output

```
🚀 PgVector Demo with Testcontainers
==================================================

🐳 Starting PostgreSQL container with pgvector...
✅ PostgreSQL running with connection: host=localhost port=xxxxx...

📦 Creating table...

1️⃣ Demo: Insert Documents
--------------------------------------------------
✅ Inserted 4 documents

2️⃣ Demo: Vector Search
--------------------------------------------------
Top 3 results for 'programming languages':
  1. Go Programming (score: 0.8542)
  2. Python Programming (score: 0.8123)
  3. Machine Learning (score: 0.6234)

3️⃣ Demo: Search with Metadata Filters
--------------------------------------------------
Results for 'coding' with category='programming':
  1. Go Programming (score: 0.7891)
  2. Python Programming (score: 0.7654)

... (more output)

🧹 Stopping PostgreSQL container...
✨ Demo completed successfully!
```

## How It Works

### 1. Container Setup
```go
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
    // ... container creation
}
```

### 2. Ollama Embedder
```go
ollamaEmbedder := embedder.NewOllamaEmbedder(
    embedder.WithOllamaModel("gemma:2b", 2048),
    embedder.WithOllamaHost("http://localhost:11434"),
)
```

### 3. PgVector Configuration
```go
pgDB, err := pgvector.NewPgVector(pgvector.PgVectorConfig{
    ConnectionString: connStr,
    TableName:        "documents",
    Embedder:         ollamaEmbedder,
    SearchType:       vectordb.SearchTypeVector,
    Distance:         vectordb.DistanceCosine,
})
```

## Key Features

### Vector Search
```go
results, err := pgDB.Search(ctx, "programming languages", 3, nil)
```

### Filtered Search
```go
filters := map[string]interface{}{
    "category": "programming",
}
results, err := pgDB.Search(ctx, "coding", 5, filters)
```

### Hybrid Search
```go
results, err := pgDB.HybridSearch(ctx, "database systems", 3, nil)
```

### Document Operations
```go
// Insert
err := pgDB.Insert(ctx, documents, nil)

// Check existence
exists, err := pgDB.IDExists(ctx, "1")

// Get count
count, err := pgDB.GetCount(ctx)

// Drop table
err := pgDB.Drop(ctx)
```

## Advantages of Testcontainers

1. **No Manual Setup**: Containers start automatically
2. **Isolation**: Each run uses a fresh database
3. **Reproducibility**: Same environment every time
4. **Cleanup**: Automatic container termination
5. **CI/CD Ready**: Works in automated pipelines

## Performance Tips

1. **Vector Dimensions**: Use appropriate dimensions for your model
2. **Indexing**: PgVector automatically creates HNSW indexes
3. **Batch Operations**: Insert multiple documents at once
4. **Connection Pooling**: Reuse database connections

## Troubleshooting

### Docker Issues
```bash
# Check if Docker is running
docker ps

# Check Docker logs
docker logs <container_id>
```

### Ollama Issues
```bash
# Verify Ollama is running
curl http://localhost:11434/api/tags

# Check if model is available
ollama list
```

### Connection Issues
```bash
# Test PostgreSQL connection
psql -h localhost -p <port> -U postgres -d vectordb
```

## Comparison with Qdrant

| Feature | PgVector | Qdrant |
|---------|----------|--------|
| **Type** | PostgreSQL Extension | Dedicated Vector DB |
| **Setup** | Requires PostgreSQL | Standalone |
| **Scalability** | Good | Excellent |
| **Features** | Basic vector ops | Advanced filtering |
| **Integration** | SQL ecosystem | REST API |
| **Best For** | Existing PG apps | Pure vector workloads |

## Related Examples

- [Qdrant Advanced Features](../qdrant_advanced/) - Advanced vector operations
- [Knowledge Base](../../agents/knowledge_pdf/) - RAG implementation
- [Memory Search](../../agents/memory_example/) - Semantic memory

## Implementation Details

### Files
- `agno/vectordb/pgvector/pgvector.go` - PgVector implementation
- `cookbook/vectordb/pgvector_example/main.go` - This example

### Dependencies
- `github.com/pgvector/pgvector-go` - PgVector Go client
- `github.com/lib/pq` - PostgreSQL driver
- `github.com/testcontainers/testcontainers-go` - Container management

## Next Steps

1. Try [Qdrant advanced features](../qdrant_advanced/)
2. Implement [RAG with vector search](../../agents/knowledge_pdf/)
3. Explore [hybrid search strategies](../../agents/knowledge_filters_example/)
