# 📚 Knowledge Base Module

The Knowledge Base module provides comprehensive support for PDF processing and document management in Agno-Golang, following the native compatibility pattern of Agno Python.

## 🚀 Features

### ✅ Core Capabilities
- **Native Compatibility**: Interface `knowledge.VectorDB = vectordb.VectorDB` eliminates adapter requirements
- **Local PDF Support**: Process PDF files from the filesystem
- **URL PDF Support**: Automatic download and processing of PDFs via HTTP/HTTPS
- **Parallel Processing**: Goroutine-based workers for optimized vector insertion
- **Progress Bars**: Real-time visual feedback with Unicode progress bars (█░▓▒)
- **Intelligent Chunking**: Text division with configurable overlap (500 chars default)
- **Rich Metadata**: Origin information and context preservation
- **Rate Limiting**: Configurable delays for API stability
- **Retry Logic**: Exponential backoff retry for robustness
- **Qdrant Integration**: Direct compatibility with Qdrant as vector backend

## 🔧 Components

### `base.go`
- `Knowledge` interface for knowledge bases
- `VectorDB = vectordb.VectorDB` type alias for native compatibility
- `BaseKnowledge` implementation with common functionalities
- Validation and sanitization utilities

### `pdf.go`
- `PDFKnowledgeBase`: PDF-specific implementation
- Local files and URL support
- `pdftotext` integration for text extraction
- Configurable chunking (default: 500 characters, overlap: 50)
- **Parallel Processing**: `LoadParallel()` method with configurable workers
- **Progress Tracking**: Visual progress bars with Unicode chars (📈 📊 🚀)
- **Rate Limiting**: 100ms delays between requests for stability
- **Retry Logic**: Up to 3 retries with exponential backoff
- Metadata processing and unique ID generation
- UTF-8 sanitization for extracted texts

## 🐍 Python Compatibility

The implementation follows the exact Agno Python pattern:

```python
# Agno Python
vector_db = Qdrant(...)
knowledge_base = PDFUrlKnowledgeBase(..., vector_db=vector_db)
```

```go
// Agno Golang (current implementation)
vectorDB := qdrant.NewQdrant(config)
knowledgeBase := knowledge.NewPDFKnowledgeBase("name", vectorDB)
```

### 🔄 Key Improvements

1. **Adapter Elimination**: Direct use of `vectordb.VectorDB` interface
2. **Native Compatibility**: `knowledge.VectorDB = vectordb.VectorDB`
3. **Unified Interface**: Same method signatures as Python
4. **Direct Integration**: Qdrant implements `vectordb.VectorDB` natively

## 📖 Usage Guide

### 1. Setup Qdrant
```go
openaiEmbedder := embedder.NewOpenAIEmbedder()
openaiEmbedder.Timeout = 60 * time.Second

qdrantConfig := qdrant.QdrantConfig{
    Host:       "localhost",
    Port:       6334, // gRPC port
    Collection: "pdf-knowledge-base",
    Embedder:   openaiEmbedder,
    SearchType: vectordb.SearchTypeVector,
    Distance:   vectordb.DistanceCosine,
}
vectorDB, err := qdrant.NewQdrant(qdrantConfig)
```

### 2. Create PDF Knowledge Base
```go
knowledgeBase := knowledge.NewPDFKnowledgeBase("pdf-knowledge", vectorDB)

// Configure PDFs
knowledgeBase.URLs = []string{
    "https://arxiv.org/pdf/2305.13245.pdf",
}

// Load documents with parallel processing
err := knowledgeBase.LoadParallel(ctx, true, 3) // 3 workers

// Or load sequentially with progress
err := knowledgeBase.Load(ctx, true)
```

### 3. Advanced Configuration
```go
// Adjust chunking
knowledgeBase.ChunkSize = 800
knowledgeBase.ChunkOverlap = 100

// Complex configurations with metadata
knowledgeBase.Configs = []PDFConfig{
    {
        URL: "https://example.com/doc.pdf",
        Metadata: map[string]interface{}{
            "category": "research",
            "priority": "high",
        },
    },
}
```

### 4. Agent Integration
```go
agentConfig := agent.AgentConfig{
    Model:       model,
    Name:        "PDF Agent",
    Role:        "PDF document analysis specialist",
    Instructions: "You have access to a PDF knowledge base. Use search_knowledge_base to find information.",
    // ... other configurations
}
agentObj := agent.NewAgent(agentConfig)

// Search in knowledge base
results, err := knowledgeBase.Search(ctx, "important concepts", 5)
```

### 5. Visual Progress Feedback
During loading, you'll see real-time progress bars:

```
📚 Parallel loading of 1 source(s) with 3 workers...
📈 [██████████████████████████████] 100.0% (1/1) Downloading: https://example.pdf ✅ 461 documents

📊 Total documents loaded: 461
🚀 Starting parallel vector database insertion with 3 workers...
⚡ Parallel processing with 3 workers of 461 documents...
🔄 [████████████████████████████▓▓] 93.5% (431/461) processed
✅ Parallel processing complete!
```

## 🛠️ Available Methods

### PDFKnowledgeBase

#### Loading
```go
// Sequential loading with progress
Load(ctx context.Context, recreate bool) error

// Parallel loading (recommended for large PDFs)
LoadParallel(ctx context.Context, recreate bool, numWorkers int) error

// Single document loading
LoadDocument(ctx context.Context, doc document.Document) error

// Loading by path/URL
LoadDocumentFromPath(ctx context.Context, pathOrURL string, metadata map[string]interface{}) error
```

#### Search
```go
// Semantic search
Search(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*SearchResult, error)

// Document search
SearchDocuments(ctx context.Context, query string, numDocuments int, filters map[string]interface{}) ([]document.Document, error)
```

#### Configuration
```go
// Configure chunking
kb.ChunkSize = 500
kb.ChunkOverlap = 50

// Configure sources
kb.Paths = []string{"/path/to/pdf"}
kb.URLs = []string{"https://example.com/doc.pdf"}
kb.Configs = []PDFConfig{{URL: "...", Metadata: map[string]interface{}{"tag": "value"}}}
```

## 📝 Advanced Examples

### 1. Multiple PDF Processing
```go
knowledgeBase := knowledge.NewPDFKnowledgeBase("multi-pdf", vectorDB)

knowledgeBase.Configs = []knowledge.PDFConfig{
    {
        URL: "https://arxiv.org/pdf/2305.13245.pdf",
        Metadata: map[string]interface{}{
            "category": "AI Research",
            "year": 2023,
        },
    },
    {
        Path: "/local/documents/manual.pdf",
        Metadata: map[string]interface{}{
            "category": "Documentation",
            "internal": true,
        },
    },
}

// Process with 3 workers
err := knowledgeBase.LoadParallel(ctx, true, 3)
```

### 2. Search with Filters
```go
// Search only research documents
filters := map[string]interface{}{
    "category": "AI Research",
}

results, err := knowledgeBase.Search(ctx, "neural networks", 5, filters)
for _, result := range results {
    fmt.Printf("Score: %.2f - %s\n", result.Score, result.Document.Content[:100])
}
```

### 3. Performance Configuration
```go
// For small PDFs
knowledgeBase.ChunkSize = 300
knowledgeBase.ChunkOverlap = 30

// For large PDFs with fast processing
knowledgeBase.ChunkSize = 800
knowledgeBase.ChunkOverlap = 80

// Use more workers for faster insertion (be careful with rate limits)
err := knowledgeBase.LoadParallel(ctx, true, 5)
```

## 🔧 Dependencies

### System Requirements
- `pdftotext` (part of `poppler-utils` package)
- Qdrant running on `localhost:6334` (gRPC)
- OpenAI API Key for embeddings

### Installation
```bash
# Ubuntu/Debian
sudo apt-get install poppler-utils

# macOS
brew install poppler

# Windows
# Download from https://poppler.freedesktop.org/
```

## 🎯 Architecture

```
┌─────────────────────┐    ┌──────────────────┐    ┌─────────────────────┐
│   Agent             │───▶│  Knowledge       │───▶│   VectorDB          │
│                     │    │  (PDF)           │    │   (Qdrant)          │
└─────────────────────┘    └──────────────────┘    └─────────────────────┘
                                    │
                           ┌──────────────────┐
                           │   Documents      │
                           │   (Chunked PDF)  │
                           └──────────────────┘
```

## ✅ Implementation Status

- [x] Native interface compatible with vectordb
- [x] Local PDF processing
- [x] URL PDF processing
- [x] Chunking with overlap
- [x] Qdrant integration
- [x] Complete functional example
- [x] Adapter elimination
- [x] Python pattern compatibility
- [x] **Parallel processing with workers**
- [x] **Visual progress bars**
- [x] **Rate limiting and retry logic**
- [x] **UTF-8 sanitization**
- [x] **Performance optimization**

## 🧪 Testing

To test the implementation:

```bash
cd examples/pdf_qdrant_agent
export OPENAI_API_KEY="your-key-here"
go run main.go
```

### Parallel Performance Test
```bash
cd examples/test_parallel
export OPENAI_API_KEY="your-key-here"
go run main.go
```

Expected results:
- PDF download and processing in seconds
- Visual progress bars
- Parallel processing with multiple workers
- Performance: ~461 documents in ~25 seconds

Make sure:
1. Qdrant is running on `localhost:6334` (gRPC)
2. `pdftotext` is installed
3. OpenAI API Key is configured
4. Internet connection for downloading example PDFs

## 🚀 Performance

### Completed Benchmarks
- **Large PDF (461 chunks)**: ~25 seconds with 2 workers
- **Processing Rate**: ~18 documents/second
- **Memory Usage**: Optimized with streaming
- **API Calls**: Rate limited (100ms delays)

### Recommended Settings
- **Workers**: 2-3 for external APIs (OpenAI)
- **Chunk Size**: 500-800 characters
- **Chunk Overlap**: 50-100 characters
- **Timeout**: 60 seconds for embeddings

## 🔧 Troubleshooting

### Common Issues

#### 1. "pdftotext not found"
```bash
# Ubuntu/Debian
sudo apt-get install poppler-utils

# macOS  
brew install poppler

# Verify installation
which pdftotext
```

#### 2. "Connection refused" (Qdrant)
```bash
# Check if Qdrant is running
docker ps | grep qdrant

# Start Qdrant if needed
docker run -p 6333:6333 -p 6334:6334 qdrant/qdrant
```

#### 3. "OpenAI API rate limit"
- Reduce number of workers: `LoadParallel(ctx, true, 1)`
- Increase delays in code (modify rate limiting)
- Check OpenAI API quota

#### 4. "Out of memory" (Large PDFs)
- Reduce chunk size: `knowledgeBase.ChunkSize = 300`
- Process in smaller batches
- Use `Load()` instead of `LoadParallel()`

### Performance Tips

1. **Ideal Workers**: 2-3 for external APIs, 5-10 for local processing
2. **Chunk Size**: 500-800 chars for technical texts, 300-500 for general texts
3. **Overlap**: 10-20% of chunk size
4. **Timeout**: 60s+ for long text embeddings

### Debug Logs
```go
// Enable detailed logs
log.SetLevel(log.DebugLevel)

// Check detailed progress
fmt.Printf("Processing document %d/%d\n", current, total)
```

---

This implementation ensures that Agno-Golang has complete parity with Agno Python for PDF processing, maintaining the same simplicity and interface compatibility, now with optimized performance and enhanced visual feedback.
