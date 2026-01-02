# Learning Loop Example (Continuous Learning)

This example demonstrates the **Learning Loop**: a lightweight continuous learning layer built on top of the existing `knowledge.Knowledge` store.

## What it does

- **Before the model call (RAG)**: retrieves relevant memories from the knowledge store and injects them into the prompt.
- **After the model call (continuous learning)**: decides whether to persist a reusable, canonical memory (not the full answer), with basic deduplication and safety heuristics.
- **Promotion**: by default memories are saved as `candidate` and are promoted to `verified` when the user confirms success (e.g., "That worked").
- **Evidence-based promotion**: repeated retrieval without negative feedback can auto-promote candidates (configurable thresholds).
- **Filters**: respects `agent.WithKnowledgeFilters(...)` so you can isolate memories by `language`, `domain`, `level`, tenant, etc.

## Learning Loop vs Update Knowledge

- **Learning Loop**: automatic, heuristic-based, and designed to store reusable “artifacts”.
- **Update Knowledge**: manual/tool-driven (`knowledge.add`, `knowledge.search`) and best for explicit user-controlled knowledge updates.

## Requirements

- Docker (to run Qdrant via Testcontainers)
- Ollama running locally at `http://localhost:11434`
- `nomic-embed-text` embed model (768 dims)

## Run

```bash
ollama serve
ollama pull nomic-embed-text
go run main.go
```

## Enabling the Learning Loop

You can enable it via AgentConfig:

```go
ag, _ := agent.NewAgent(agent.AgentConfig{
    Learning: learningManager, // alias for LearningManager
})
```

Or via an Agent option:

```go
ag, _ := agent.NewAgentWithOptions(
    agent.AgentConfig{},
    agent.WithLearningLoop(learningManager),
)
```

## Notes

- The Learning Loop persists items into the same vector store used by `knowledge.Knowledge`.
- Saved items are tagged with metadata (type, status, confidence, timestamps, source IDs if provided).
- The write-gate blocks sensitive data (tokens, cookies, emails, CPF/CNPJ, etc.) and tries to avoid unstable facts.
