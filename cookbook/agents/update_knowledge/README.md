# Update Knowledge Tool Example

This example demonstrates the **UpdateKnowledge** default tool, which allows agents to add information to and search their knowledge base.

## Update Knowledge vs Learning Loop

- **Update Knowledge** is explicit/manual: the model chooses to call `knowledge.add(...)` and you control what is stored.
- **Learning Loop** is automatic: it can retrieve memories before the run and persist canonical “artifacts” after the run (with heuristics, dedupe, and promotion).

## Overview

The `UpdateKnowledge` tool provides two methods:
- `knowledge.add(content, metadata)` - Add information to the knowledge base
- `knowledge.search(query, limit)` - Search the knowledge base for relevant information

## Features

### 1. **knowledge.add(content, metadata)**
Adds new information to the knowledge base with optional metadata.

**Parameters:**
- `content` (string): The information to store
- `metadata` (map[string]interface{}, optional): Additional metadata for the document

**Example:**
```go
agent.WithEnableUpdateKnowledgeTool(true)
```

The agent can then use: `knowledge.add("Go uses goroutines for concurrency", {"topic": "concurrency"})`

### 2. **knowledge.search(query, limit)**
Searches the knowledge base for relevant information.

**Parameters:**
- `query` (string): Search query to find relevant documents
- `limit` (int, optional): Maximum number of results to return (default: 5)

**Example:**
The agent can use: `knowledge.search("concurrency", 3)` to find top 3 relevant documents.

## Requirements

- **Knowledge Base**: Must be configured with `agent.WithKnowledge(kb)`
- **Vector Database**: Requires a vector database backend (Qdrant, PGVector, etc.)
- **Embedder**: Needs an embedder for semantic search
- **Enable Flag**: `agent.WithEnableUpdateKnowledgeTool(true)`

## Use Cases

1. **Dynamic Knowledge**: Agent can learn and store new information during conversations
2. **Self-Documentation**: Agent can document its own actions and decisions
3. **User Preferences**: Store user-specific information for personalization
4. **Context Building**: Build up domain knowledge over time
5. **RAG Enhancement**: Augment retrieval with runtime-added information

## Running the Example

```bash
# Set your Ollama API key
export OLLAMA_API_KEY=your_api_key

# Run the example
go run main.go
```

## Expected Output

The agent will:
1. Store Go best practices in the knowledge base
2. Add concurrency information
3. Store framework documentation
4. Search and retrieve relevant information
5. Demonstrate semantic search capabilities

## Implementation Details

### Agent Configuration
```go
ag, err := agent.NewAgent(
    "KnowledgeAssistant",
    model,
    agent.WithKnowledge(kb),
    agent.WithEnableUpdateKnowledgeTool(true), // Enable the tool
)
```

### Knowledge Base Setup
The example uses Qdrant in-memory with Ollama embeddings:
```go
// Create embedder
emb := embedder.NewOllamaEmbedder("nomic-embed-text")

// Create vector database
vectorDB, err := qdrant.NewQdrantDB(ctx, ":memory:", "collection")

// Create knowledge base
kb := knowledge.NewKnowledgeBase(vectorDB)
```

### Tool Usage
The agent automatically has access to:
- `knowledge.add(content, metadata)` - No manual registration needed
- `knowledge.search(query, limit)` - Automatically available

## Comparison with ReadChatHistory

| Feature | UpdateKnowledge | ReadChatHistory |
|---------|----------------|-----------------|
| **Purpose** | Manage knowledge base | Read conversation history |
| **Storage** | Vector database | Chat storage |
| **Search** | Semantic search | Keyword/content search |
| **Scope** | Domain knowledge | Conversation history |
| **Methods** | add, search | read, search |
| **Use Case** | RAG, learning | Context recall |

## Best Practices

1. **Structured Content**: Add well-formatted, coherent information
2. **Meaningful Metadata**: Include topic, source, timestamp in metadata
3. **Search Limits**: Use limit parameter to control result size
4. **Embedder Choice**: Use appropriate embedder for your domain
5. **Deduplication**: Be aware that similar content may be stored multiple times
6. **Vector DB**: Choose the right vector database for your scale

## Vector Database Options

The example uses Qdrant in-memory, but you can use:
- **Qdrant**: High-performance vector search
- **PGVector**: PostgreSQL extension for vectors
- **Custom**: Implement the `vectordb.VectorDB` interface

## Metadata Usage

Add metadata to enhance search and retrieval:
```go
knowledge.add("content", {
    "topic": "Go concurrency",
    "source": "user",
    "timestamp": "2024-01-15",
    "confidence": 0.95
})
```

## Notes

- The tool is automatically registered when `EnableUpdateKnowledgeTool` is true
- Requires a Knowledge base that implements the `knowledge.Knowledge` interface
- Search uses semantic similarity via vector embeddings
- Knowledge persists only in the vector database (not in chat history)
- In-memory vector databases reset on restart
