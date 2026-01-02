# Learning Loop (Continuous Learning)

The Learning Loop is a small, plugin-friendly layer that turns your existing `knowledge.Knowledge` store into a **continuous learning system**:

- **Pre-run (RAG)**: retrieve relevant memories and inject them into the prompt.
- **Post-run (learning)**: decide if a reusable memory should be saved back into the knowledge store.

This keeps your *profile/style* (culture) lightweight, while placing real learned content on top of the vector store.
Learning Loop does not replace your existing knowledge ingestion (PDFs, docs, manuals); it complements it by continuously learning from conversations.

Learning Loop stores its artifacts under `learning_namespace=learning` so they don't mix with your domain documents.

## Design goals

- **Reusable**: store canonical, reusable summaries/snippets (not full chat transcripts).
- **Governable**: add metadata (`type`, `status`, timestamps, confidence) to keep the store manageable.
- **Safe by default**: block obvious sensitive content and avoid unstable facts.
- **Multi-tenant safe**: always tag and retrieve by `learning_user_id`.
- **Pluggable**: no changes required to model providers; it runs inside the agent's context-building and post-run flow.

## Key types

- `learning.Manager`
  - `RetrieveContext(ctx, userID, query) (string, error)`
  - `RetrieveContextWithFilters(ctx, userID, query, filters) (string, error)`
  - `ObserveAndLearn(ctx, userID, userMsg, assistantMsg string, meta map[string]interface{}) error`

The Learning Loop persists to your configured `knowledge.Knowledge` implementation (Qdrant, PGVector, etc.).

## How it works

### 1) RetrieveContext (before the model call)

`RetrieveContext` searches the knowledge store and formats the results into short bullet points:

```
<learning_memories>
Relevant memories (from your history):
- [verified/snippet] ...
- [verified/procedure] ...
</learning_memories>
```

Retrieval is filter-aware: if you pass `agent.WithKnowledgeFilters(...)` to a run, those filters are applied to Learning Loop queries and saved into the memory metadata for future isolation.

Retrieval defaults:

- Prefer `verified` over `candidate`
- Never return `deprecated`
- Hard limits: `TopK=6`, `MaxItems=6`, `MaxChars=1500`
- Ranking priority: status > semantic score > recency > hits

### 2) ObserveAndLearn (after the model call)

`ObserveAndLearn`:

1. Builds a canonical **artifact** (procedure/snippet/pattern) from the conversation turn.
2. Runs a write-gate to decide if it should be saved (blocks sensitive/unstable/too-specific content).
3. Deduplicates near-duplicates using a SimHash-based check + vector search.
4. Applies a deterministic dedupe action:
   - `skip`: if the artifact is effectively identical
   - `merge`: update the existing item (version++, updated_at, hits++)
   - `new_version`: save a new item and mark the old one as deprecated
5. Persists a `candidate` item by default when saving new artifacts.

Why both vector search + SimHash:

- Vector search catches semantic duplicates.
- SimHash catches near-identical text to avoid store bloat.

### Promotion (candidate -> verified)

If the user confirms success (e.g. "That worked"), the Learning Loop can promote the last candidate memory to `verified`.

Feedback detection can be driven by:

- Simple keyword heuristics (default)
- Explicit signals in `meta` (e.g., `validation_passed: true`)
 
Implementations may expose explicit APIs such as `PromoteLastCandidate(userID)` or `Deprecate(docID)` for UI-driven governance.

### Evidence-based auto-promotion

When a candidate memory is retrieved repeatedly without negative feedback, it can be promoted automatically:

- Streak-based: `learning_streak >= 3` -> promote with lower confidence
- Hit-based: `learning_hits >= 5` -> promote with lower confidence

These thresholds are configurable in `learning.ManagerConfig`.

### Demotion / deprecation

If the user reports failure (e.g. "didn't work", "wrong", "changed"), the Learning Loop can mark the relevant memory as `deprecated` so it is not surfaced again.

## Metadata schema (recommended)

Each memory is stored as a normal `document.Document` with metadata keys (examples):

- `learning_namespace`: `"learning"`
- `learning_user_id`: user ID
- `learning_type`: `faq | pattern | snippet | decision | procedure`
- `learning_topic`: short topic string
- `learning_tags`: []string (always an array)
- `learning_status`: `candidate | verified | deprecated`
- `learning_confidence`: float
- `learning_hits`: int
- `learning_streak`: int (consecutive positive retrievals)
- `learning_version`: int
- `learning_created_at`, `learning_updated_at`: int64 unix seconds
- `learning_simhash64`: unsigned 64-bit (uint64), hex-encoded (16 hex chars)
- `learning_source_*`: optional message/session IDs (if you have them)

## Usage

### 1) Create a knowledge store

Any `knowledge.Knowledge` store works. You typically want a vector DB + embedder (Qdrant, PGVector, etc.).

### 2) Create a LearningManager

```go
kb := knowledge.NewBaseKnowledge("my_kb", vectorDB)
lm := learning.NewManager(kb, learning.DefaultManagerConfig())
```

### 3) Attach it to the Agent

```go
ag, _ := agent.NewAgent(agent.AgentConfig{
    UserID:          "user123",
    Knowledge:       kb,
    Learning:        lm, // alias for LearningManager
})
```

Or use the option helper:

```go
ag, _ := agent.NewAgentWithOptions(
    agent.AgentConfig{
        UserID:    "user123",
        Knowledge: kb,
    },
    agent.WithLearningLoop(lm),
)
```

## Cookbook example

See `cookbook/agents/learning_loop/`.

## Learning Loop vs Culture

See `docs/learning/LEARNING_VS_CULTURE.md`.

## Memory isolation (multi-tenant)

All learning artifacts are stored with `learning_user_id` and optionally `learning_source_session_id`.
Retrieval always applies these filters to prevent cross-user leakage.

## Recommended admin operations (optional)

For production governance, you may want to expose:

- `Promote(id)` / `PromoteLastCandidate(userID)`
- `Deprecate(docID, reason)`
- `ListRecent(userID, limit)`
- `PurgeDeprecated(olderThan)`
