# 🧠 Embedder Module

The Embedder module provides text embedding generation capabilities for Agno-Golang, supporting multiple embedding providers with a unified interface for vector generation and text processing.

## 🚀 Features

### ✅ Core Capabilities
- **Multi-Provider Support**: OpenAI, Ollama, Sentence Transformers, and custom embedders
- **Unified Interface**: Single `Embedder` interface for all providers
- **Batch Processing**: Efficient bulk embedding generation
- **Caching Support**: Optional embedding caching for performance
- **Rate Limiting**: Built-in rate limiting for API providers
- **Error Handling**: Robust retry logic with exponential backoff
- **Async Processing**: Non-blocking embedding generation

## 🔧 Architecture

```
┌─────────────────────┐    ┌──────────────────┐    ┌─────────────────────┐
│   Knowledge Base    │───▶│   Embedder       │───▶│   Provider          │
│                     │    │   Interface      │    │   (OpenAI/Ollama)   │
└─────────────────────┘    └──────────────────┘    └─────────────────────┘
                                    │
                           ┌──────────────────┐
                           │   Vector Cache   │
                           │   (Optional)     │
                           └──────────────────┘
```

## 📖 Interface Definition

### Core Embedder Interface
```go
type Embedder interface {
    // Single text embedding
    GetEmbedding(text string) ([]float64, error)
    
    // Batch text embeddings
    GetEmbeddings(texts []string) ([][]float64, error)
    
    // Provider information
    GetInfo() EmbedderInfo
    
    // Vector dimension
    GetDimension() int
}
```

### Provider Information
```go
type EmbedderInfo struct {
    Provider   string `json:"provider"`
    Model      string `json:"model"`
    Dimension  int    `json:"dimension"`
    MaxTokens  int    `json:"max_tokens"`
    RateLimit  int    `json:"rate_limit"`
}
```

## 🤖 OpenAI Embedder

### Configuration
```go
type OpenAIConfig struct {
    APIKey    string
    Model     string        // Default: "text-embedding-ada-002"
    BaseURL   string        // Optional custom endpoint
    Timeout   time.Duration // Default: 30s
    MaxRetries int          // Default: 3
    RateLimit  int          // Requests per minute
}
```

### Usage Example
```go
package main

import (
    "fmt"
    "github.com/devalexandre/agno-golang/agno/embedder"
)

func main() {
    // Create OpenAI embedder with default settings
    embedder := embedder.NewOpenAIEmbedder()
    
    // Or with custom configuration
    config := embedder.OpenAIConfig{
        APIKey:     "your-api-key",
        Model:      "text-embedding-ada-002",
        Timeout:    60 * time.Second,
        MaxRetries: 5,
    }
    embedder = embedder.NewOpenAIEmbedderWithConfig(config)
    
    // Generate single embedding
    vector, err := embedder.GetEmbedding("This is a sample text")
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Vector dimension: %d\n", len(vector))
    fmt.Printf("First 5 components: %v\n", vector[:5])
    
    // Generate batch embeddings
    texts := []string{
        "First document",
        "Second document",
        "Third document",
    }
    
    vectors, err := embedder.GetEmbeddings(texts)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Generated %d embeddings\n", len(vectors))
}
```

### Environment Configuration
```bash
# Set OpenAI API key
export OPENAI_API_KEY="your-api-key-here"

# Optional: Custom base URL
export OPENAI_BASE_URL="https://api.openai.com/v1"
```

## 🦙 Ollama Embedder

### Configuration
```go
type OllamaConfig struct {
    Host      string // Default: "localhost"
    Port      int    // Default: 11434
    Model     string // Default: "nomic-embed-text"
    Timeout   time.Duration
}
```

### Usage Example
```go
// Create Ollama embedder with default settings
embedder := embedder.NewOllamaEmbedder()

// Or with custom configuration
config := embedder.OllamaConfig{
    Host:    "localhost",
    Port:    11434,
    Model:   "nomic-embed-text",
    Timeout: 30 * time.Second,
}
embedder = embedder.NewOllamaEmbedderWithConfig(config)

// Generate embeddings
vector, err := embedder.GetEmbedding("Local embedding test")
if err != nil {
    panic(err)
}

fmt.Printf("Local vector dimension: %d\n", len(vector))
```

### Available Models
```bash
# Popular Ollama embedding models
ollama pull nomic-embed-text      # 768 dimensions
ollama pull mxbai-embed-large     # 1024 dimensions
ollama pull sentence-transformers # Various dimensions
```

## 🔧 Batch Processing

### Efficient Batch Operations
```go
func ProcessDocumentsBatch(embedder embedder.Embedder, documents []string) error {
    const batchSize = 100
    
    for i := 0; i < len(documents); i += batchSize {
        end := i + batchSize
        if end > len(documents) {
            end = len(documents)
        }
        
        batch := documents[i:end]
        vectors, err := embedder.GetEmbeddings(batch)
        if err != nil {
            return fmt.Errorf("batch processing error: %w", err)
        }
        
        // Process vectors
        for j, vector := range vectors {
            fmt.Printf("Document %d: %d dimensions\n", i+j, len(vector))
        }
        
        // Rate limiting
        time.Sleep(100 * time.Millisecond)
    }
    
    return nil
}
```

### Parallel Processing
```go
func ProcessDocumentsParallel(embedder embedder.Embedder, documents []string, workers int) error {
    docChan := make(chan string, len(documents))
    resultChan := make(chan []float64, len(documents))
    errorChan := make(chan error, workers)
    
    // Send documents to channel
    for _, doc := range documents {
        docChan <- doc
    }
    close(docChan)
    
    // Start workers
    for i := 0; i < workers; i++ {
        go func() {
            for doc := range docChan {
                vector, err := embedder.GetEmbedding(doc)
                if err != nil {
                    errorChan <- err
                    return
                }
                resultChan <- vector
            }
            errorChan <- nil
        }()
    }
    
    // Collect results
    var results [][]float64
    var errors []error
    
    for i := 0; i < workers; i++ {
        if err := <-errorChan; err != nil {
            errors = append(errors, err)
        }
    }
    
    for i := 0; i < len(documents); i++ {
        select {
        case vector := <-resultChan:
            results = append(results, vector)
        case <-time.After(30 * time.Second):
            return fmt.Errorf("timeout waiting for embeddings")
        }
    }
    
    if len(errors) > 0 {
        return fmt.Errorf("embedding errors: %v", errors)
    }
    
    fmt.Printf("Generated %d embeddings\n", len(results))
    return nil
}
```

## 💾 Caching

### Memory Cache
```go
type CachedEmbedder struct {
    embedder embedder.Embedder
    cache    map[string][]float64
    mutex    sync.RWMutex
}

func NewCachedEmbedder(embedder embedder.Embedder) *CachedEmbedder {
    return &CachedEmbedder{
        embedder: embedder,
        cache:    make(map[string][]float64),
    }
}

func (c *CachedEmbedder) GetEmbedding(text string) ([]float64, error) {
    // Check cache first
    c.mutex.RLock()
    if vector, exists := c.cache[text]; exists {
        c.mutex.RUnlock()
        return vector, nil
    }
    c.mutex.RUnlock()
    
    // Generate new embedding
    vector, err := c.embedder.GetEmbedding(text)
    if err != nil {
        return nil, err
    }
    
    // Cache result
    c.mutex.Lock()
    c.cache[text] = vector
    c.mutex.Unlock()
    
    return vector, nil
}
```

### Persistent Cache
```go
type PersistentCache struct {
    embedder embedder.Embedder
    db       *sql.DB
}

func (p *PersistentCache) GetEmbedding(text string) ([]float64, error) {
    // Check database cache
    var vectorJSON string
    err := p.db.QueryRow("SELECT vector FROM embeddings WHERE text_hash = ?", hash(text)).Scan(&vectorJSON)
    
    if err == nil {
        var vector []float64
        json.Unmarshal([]byte(vectorJSON), &vector)
        return vector, nil
    }
    
    // Generate new embedding
    vector, err := p.embedder.GetEmbedding(text)
    if err != nil {
        return nil, err
    }
    
    // Store in database
    vectorBytes, _ := json.Marshal(vector)
    p.db.Exec("INSERT INTO embeddings (text_hash, vector) VALUES (?, ?)", hash(text), string(vectorBytes))
    
    return vector, nil
}
```

## 🚀 Performance Optimization

### Rate Limiting
```go
type RateLimitedEmbedder struct {
    embedder  embedder.Embedder
    limiter   *rate.Limiter
}

func NewRateLimitedEmbedder(embedder embedder.Embedder, requestsPerMinute int) *RateLimitedEmbedder {
    limit := rate.Limit(float64(requestsPerMinute) / 60.0) // Convert to per-second
    return &RateLimitedEmbedder{
        embedder: embedder,
        limiter:  rate.NewLimiter(limit, requestsPerMinute),
    }
}

func (r *RateLimitedEmbedder) GetEmbedding(text string) ([]float64, error) {
    // Wait for rate limit
    ctx := context.Background()
    err := r.limiter.Wait(ctx)
    if err != nil {
        return nil, err
    }
    
    return r.embedder.GetEmbedding(text)
}
```

### Connection Pooling
```go
type PooledEmbedder struct {
    embedders []embedder.Embedder
    index     int64
    mutex     sync.Mutex
}

func NewPooledEmbedder(configs []OpenAIConfig) *PooledEmbedder {
    embedders := make([]embedder.Embedder, len(configs))
    for i, config := range configs {
        embedders[i] = embedder.NewOpenAIEmbedderWithConfig(config)
    }
    
    return &PooledEmbedder{
        embedders: embedders,
    }
}

func (p *PooledEmbedder) GetEmbedding(text string) ([]float64, error) {
    // Round-robin selection
    p.mutex.Lock()
    embedder := p.embedders[p.index%int64(len(p.embedders))]
    p.index++
    p.mutex.Unlock()
    
    return embedder.GetEmbedding(text)
}
```

## 🧪 Testing

### Unit Tests
```go
func TestEmbedder(t *testing.T) {
    embedder := embedder.NewMockEmbedder() // Test embedder
    
    vector, err := embedder.GetEmbedding("test text")
    assert.NoError(t, err)
    assert.Equal(t, 1536, len(vector)) // OpenAI ada-002 dimension
    
    // Test batch processing
    texts := []string{"text1", "text2", "text3"}
    vectors, err := embedder.GetEmbeddings(texts)
    assert.NoError(t, err)
    assert.Len(t, vectors, 3)
}
```

### Integration Tests
```go
func TestOpenAIIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    embedder := embedder.NewOpenAIEmbedder()
    
    vector, err := embedder.GetEmbedding("integration test")
    assert.NoError(t, err)
    assert.Equal(t, 1536, len(vector))
    
    // Test that similar texts have similar embeddings
    vector1, _ := embedder.GetEmbedding("machine learning")
    vector2, _ := embedder.GetEmbedding("artificial intelligence")
    
    similarity := cosineSimilarity(vector1, vector2)
    assert.Greater(t, similarity, 0.5) // Should be similar
}
```

### Benchmark Tests
```go
func BenchmarkEmbedding(b *testing.B) {
    embedder := embedder.NewOpenAIEmbedder()
    text := "This is a benchmark test for embedding generation"
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := embedder.GetEmbedding(text)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

## 🔧 Custom Embedders

### Creating Custom Embedder
```go
type CustomEmbedder struct {
    model     *YourModel
    dimension int
}

func NewCustomEmbedder(modelPath string) *CustomEmbedder {
    model := LoadModel(modelPath)
    return &CustomEmbedder{
        model:     model,
        dimension: model.GetDimension(),
    }
}

func (c *CustomEmbedder) GetEmbedding(text string) ([]float64, error) {
    // Preprocess text
    tokens := c.tokenize(text)
    
    // Generate embedding using your model
    vector, err := c.model.Embed(tokens)
    if err != nil {
        return nil, err
    }
    
    return vector, nil
}

func (c *CustomEmbedder) GetEmbeddings(texts []string) ([][]float64, error) {
    vectors := make([][]float64, len(texts))
    
    for i, text := range texts {
        vector, err := c.GetEmbedding(text)
        if err != nil {
            return nil, err
        }
        vectors[i] = vector
    }
    
    return vectors, nil
}

func (c *CustomEmbedder) GetInfo() embedder.EmbedderInfo {
    return embedder.EmbedderInfo{
        Provider:  "custom",
        Model:     "your-model",
        Dimension: c.dimension,
        MaxTokens: 8192,
    }
}

func (c *CustomEmbedder) GetDimension() int {
    return c.dimension
}
```

## 🔧 Configuration Examples

### Development Setup
```go
// Simple OpenAI setup for development
embedder := embedder.NewOpenAIEmbedder()

// With custom timeout
config := embedder.OpenAIConfig{
    Timeout: 60 * time.Second,
}
embedder = embedder.NewOpenAIEmbedderWithConfig(config)
```

### Production Setup
```go
// Production setup with retry and rate limiting
config := embedder.OpenAIConfig{
    APIKey:     os.Getenv("OPENAI_API_KEY"),
    Model:      "text-embedding-ada-002",
    Timeout:    30 * time.Second,
    MaxRetries: 5,
    RateLimit:  1000, // RPM
}

baseEmbedder := embedder.NewOpenAIEmbedderWithConfig(config)

// Add caching
cachedEmbedder := NewCachedEmbedder(baseEmbedder)

// Add rate limiting
rateLimitedEmbedder := NewRateLimitedEmbedder(cachedEmbedder, 1000)
```

### Multi-Provider Setup
```go
// Fallback embedder with multiple providers
primary := embedder.NewOpenAIEmbedder()
fallback := embedder.NewOllamaEmbedder()

embedder := NewFallbackEmbedder(primary, fallback)
```

## 🔧 Troubleshooting

### Common Issues

#### 1. API Rate Limits
```go
// Implement exponential backoff
func embeddingWithRetry(embedder embedder.Embedder, text string, maxRetries int) ([]float64, error) {
    for attempt := 0; attempt < maxRetries; attempt++ {
        vector, err := embedder.GetEmbedding(text)
        if err == nil {
            return vector, nil
        }
        
        if strings.Contains(err.Error(), "rate limit") {
            backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
            time.Sleep(backoff)
            continue
        }
        
        return nil, err
    }
    
    return nil, fmt.Errorf("max retries exceeded")
}
```

#### 2. Large Text Handling
```go
// Split large texts into chunks
func embedLargeText(embedder embedder.Embedder, text string, maxTokens int) ([]float64, error) {
    if len(text) <= maxTokens {
        return embedder.GetEmbedding(text)
    }
    
    // Split into chunks
    chunks := splitTextIntoChunks(text, maxTokens)
    vectors, err := embedder.GetEmbeddings(chunks)
    if err != nil {
        return nil, err
    }
    
    // Average the vectors
    return averageVectors(vectors), nil
}
```

#### 3. Memory Management
```go
// Process large batches with streaming
func processLargeBatch(embedder embedder.Embedder, texts []string) error {
    const batchSize = 100
    
    for i := 0; i < len(texts); i += batchSize {
        end := i + batchSize
        if end > len(texts) {
            end = len(texts)
        }
        
        batch := texts[i:end]
        _, err := embedder.GetEmbeddings(batch)
        if err != nil {
            return err
        }
        
        // Force garbage collection periodically
        if i%1000 == 0 {
            runtime.GC()
        }
    }
    
    return nil
}
```

---

The Embedder module provides a robust, efficient, and extensible foundation for text embedding generation in Agno-Golang, supporting multiple providers while maintaining high performance and reliability.
