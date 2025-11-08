# Agent Run Options Examples

This folder contains practical examples demonstrating the different options available in the agent's `Run()` method.

## 📚 Available Examples

### 1. **session_state_example** - Session State Management
**File:** `session_state_example/main.go`

Demonstrates how to maintain state across multiple conversations using:
- `WithSessionID` - Identify and isolate user sessions
- `WithSessionState` - Persist custom data between runs
- `WithAddSessionStateToContext` - Include state in LLM context
- `WithAddHistoryToContext` - Control conversation history

**Use cases:**
- Maintain user preferences between conversations
- Track session information (current page, permissions)
- Implement chatbots with context memory
- Multi-user support with isolated contexts

```go
response, err := ag.Run(
    "What page am I on?",
    agent.WithSessionID("user_123_session"),
    agent.WithSessionState(sessionState),
    agent.WithAddSessionStateToContext(true),
)
```

---

### 2. **knowledge_filters_example** - Knowledge Filters
**File:** `knowledge_filters_example/main.go`

Demonstrates precise knowledge base search using metadata:
- `WithKnowledgeFilters` - Filter by metadata fields
- Simple filters (category, language, level)
- Combined multiple filters
- Language separation (en/pt)

**Use cases:**
- Multilingual documentation
- Skill level segmentation (beginner/advanced)
- Category organization (programming, devops, database)
- Topic-specific searches

```go
filters := map[string]interface{}{
    "category": "programming",
    "language": "go",
    "level":    "intermediate",
}

response, err := ag.Run(
    "What do you know about Go?",
    agent.WithKnowledgeFilters(filters),
)
```

**Example document with metadata:**
```go
document.Document{
    ID:      "go_001",
    Content: "Go channels are typed conduits...",
    Metadata: map[string]interface{}{
        "category":  "programming",
        "language":  "go",
        "topic":     "concurrency",
        "level":     "intermediate",
        "lang_code": "en",
    },
}
```

---

### 3. **context_control_example** - Context Control
**File:** `context_control_example/main.go`

Demonstrates granular control over what's included in context:
- `WithDependencies` - Pass external resources (DB, APIs)
- `WithAddHistoryToContext` - Control conversation history
- `WithAddDependenciesToContext` - Expose dependencies to LLM
- `WithAddSessionStateToContext` - Include session state

**Use cases:**
- Tools accessing databases and APIs
- Stateless vs stateful conversations
- Context-aware responses (current page, permissions)
- Environment-specific behavior (dev/prod)

```go
dependencies := map[string]interface{}{
    "database":    userDB,
    "api_key":     "sk-test-123",
    "environment": "production",
}

response, err := ag.Run(
    "Summarize my session",
    agent.WithDependencies(dependencies),
    agent.WithAddHistoryToContext(true),
    agent.WithAddDependenciesToContext(true),
    agent.WithAddSessionStateToContext(true),
)
```

---

### 4. **metadata_debug_example** - Metadata and Debugging
**File:** `metadata_debug_example/main.go`

Demonstrates request tracking and debugging:
- `WithMetadata` - Attach custom tracking data
- `WithDebugMode` - Enable/disable detailed logging
- Metadata for analytics (user tracking, A/B testing)
- Metadata for monitoring (SLA, cost centers, regions)

**Use cases:**
- Request tracing across microservices
- A/B testing and feature flag tracking
- Cost allocation and billing
- Performance monitoring and SLA compliance
- Debugging production issues
- Analytics and user behavior tracking

```go
metadata := map[string]interface{}{
    "request_id":   "req_abc_001",
    "user_id":      "user_123",
    "experiment_id": "exp_2024_11",
    "variant":      "variant_B",
    "region":       "us-east-1",
    "cost_center":  "engineering",
}

response, err := ag.Run(
    "Explain quantum computing",
    agent.WithMetadata(metadata),
    agent.WithDebugMode(true),
)
```

---

### 5. **retries_example** - Resilience with Retries
**File:** `retries_example/main.go`

Demonstrates automatic retry on failures:
- `WithRetries(n)` - Retry up to n times
- Resilience against transient errors
- Different strategies by criticality

**Use cases:**
- Network instability (connection timeout)
- API rate limiting (HTTP 429)
- Transient service outages (HTTP 5xx)
- Load balancer failovers
- Database connection pool exhaustion

```go
response, err := ag.Run(
    "Critical operation",
    agent.WithRetries(10), // 10 tentativas para operações críticas
)
```

**Retry Guidelines:**
- **0 retries:** Interactive user-facing operations (fast fail)
- **3-5 retries:** Most production scenarios
- **10+ retries:** Critical operations that must succeed
- Consider exponential backoff (future enhancement)

---

### 6. **update_knowledge** - Update Knowledge Tool
**File:** `update_knowledge/main.go`

Demonstrates the `update_knowledge` default tool:
- `EnableUpdateKnowledgeTool: true` - Enable default tool
- `knowledge_add` - Add information to knowledge base
- `knowledge_search` - Search stored information
- Integration with Qdrant vector database

**Use cases:**
- Chatbots that learn during conversations
- Assistants that memorize preferences
- Dynamic documentation systems
- Collaborative knowledge base

```go
ag, err := agent.NewAgent(agent.AgentConfig{
    Name:                      "Knowledge Assistant",
    Model:                     model,
    Knowledge:                 kb,
    EnableUpdateKnowledgeTool: true,
})

// Agent can add information:
response, err := ag.Run(
    "Add to knowledge: Go best practice - always handle errors explicitly",
)

// And search later:
response, err := ag.Run(
    "Search for Go best practices",
)
```

---

## 🚀 How to Run

Each example can be run independently:

```bash
# Session State
cd cookbook/agents/session_state_example
go run main.go

# Knowledge Filters (requer Qdrant container)
cd cookbook/agents/knowledge_filters_example
go run main.go

# Context Control (Dependencies)
cd cookbook/agents/context_control_example
go run main.go

# Metadata e Debug
cd cookbook/agents/metadata_debug_example
go run main.go

# Retries
cd cookbook/agents/retries_example
go run main.go

# Update Knowledge (requer Qdrant container)
cd cookbook/agents/update_knowledge
go run main.go
```

## 📋 Prerequisites

**All examples:**
- Go 1.21+
- Ollama Cloud API (https://ollama.com) with `kimi-k2:1t-cloud` model

**Examples with Knowledge Base:**
- Docker (for Qdrant container)
- Local Ollama with `gemma:2b` model for embeddings

```bash
# Install local embeddings model
ollama pull gemma:2b
```

## 🔗 Combining Options

The `Run()` options can be freely combined:

```go
response, err := ag.Run(
    "Complex query",
    agent.WithSessionID("session_123"),
    agent.WithUserID("user_456"),
    agent.WithSessionState(sessionState),
    agent.WithKnowledgeFilters(filters),
    agent.WithDependencies(deps),
    agent.WithMetadata(metadata),
    agent.WithRetries(5),
    agent.WithAddHistoryToContext(true),
    agent.WithAddSessionStateToContext(true),
    agent.WithAddDependenciesToContext(true),
    agent.WithDebugMode(true),
)
```

## 📊 Quick Reference

| Option | Type | Description | Example Use Case |
|--------|------|-------------|------------------|
| `WithSessionID` | `string` | Session identifier | Multi-user chatbots |
| `WithUserID` | `string` | User identifier | User tracking |
| `WithSessionState` | `map[string]interface{}` | Persistent state | Preferences, context |
| `WithKnowledgeFilters` | `map[string]interface{}` | Metadata filters | Categorical search |
| `WithDependencies` | `map[string]interface{}` | External resources | DB, APIs, config |
| `WithMetadata` | `map[string]interface{}` | Tracking data | Analytics, billing |
| `WithRetries` | `int` | Retry attempts on failure | Critical operations |
| `WithAddHistoryToContext` | `bool` | Include history | Contextual conversations |
| `WithAddSessionStateToContext` | `bool` | Include session state | Context-aware responses |
| `WithAddDependenciesToContext` | `bool` | Include dependencies | Environment-specific |
| `WithDebugMode` | `bool` | Detailed logs | Troubleshooting |
| `WithImages` | `[]models.Image` | Image inputs | Multimodal analysis |
| `WithAudio` | `[]models.Audio` | Audio inputs | Transcription |
| `WithVideos` | `[]models.Video` | Video inputs | Video analysis |
| `WithFiles` | `[]models.File` | File inputs | Document processing |

## 💡 Best Practices

1. **Session Management:**
   - Use `WithSessionID` to isolate users
   - Persist `SessionState` in database for durability
   - Limit state size for performance

2. **Knowledge Filtering:**
   - Structure metadata consistently
   - Use filters to reduce noise in results
   - Combine category + language for better precision

3. **Retries:**
   - 3-5 retries for most cases
   - 0 retries for interactive operations
   - 10+ retries only for critical operations

4. **Metadata:**
   - Include `request_id` for tracing
   - Use `user_id` and `session_id` for analytics
   - Add tags for categorization

5. **Debug Mode:**
   - Enable only in development
   - Use with `Metadata` to correlate logs
   - Disable in production (performance)

## 🎯 Next Steps

After exploring these examples, see:
- **[Knowledge PDF Example](../knowledge_pdf/)** - RAG with large PDFs
- **[Memory Example](../memory_example/)** - Persistent memory with SQLite
- **[Streaming Examples](../simple_ollama_stream/)** - Streaming responses
- **[MCP Integration](../mcp/)** - Model Context Protocol

## 📖 Documentation

For complete API documentation:
- [Agent Documentation](../../../docs/agent/)
- [Knowledge Documentation](../../../docs/knowledge/)
- [Tools Documentation](../../../docs/tools/)
