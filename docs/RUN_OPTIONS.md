# Run Options

Agno-Golang supports per-run options via `agent.Run(..., opts...)`.

These options allow you to change behavior **per request** without rebuilding the agent.

## Common options

- `agent.WithUserID(userID)`: sets the user identity for this run.
- `agent.WithSessionID(sessionID)`: sets the session identity for this run.
- `agent.WithRetries(n)`: configures retry attempts for model calls.
- `agent.WithMetadata(map[string]interface{}{})`: attaches arbitrary metadata to the run.
- `agent.WithAddHistoryToContext(true|false)`: includes chat history in context.

## Knowledge filters

Use `agent.WithKnowledgeFilters(filters)` to scope knowledge retrieval:

```go
filters := map[string]interface{}{
    "language": "en",
    "domain":   "golang",
    "level":    "advanced",
}

resp, err := ag.Run("Explain Go interfaces", agent.WithKnowledgeFilters(filters))
```

If your vector DB supports metadata filters (Qdrant, PGVector, etc.), the agent will apply them during retrieval when possible.

## Learning Loop integration

If you configured `AgentConfig.LearningManager`, the Learning Loop will:

- **Pre-run**: retrieve relevant memories using the current `KnowledgeFilters` and inject them into the prompt.
- **Post-run**: observe the turn and optionally persist a canonical artifact back into the knowledge store, also tagged with the current `KnowledgeFilters`.

See `docs/learning/README.md` and `cookbook/agents/learning_loop/`.

