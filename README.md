# Agno Framework - Go Implementation 🚀

## Pre‑release – Implemented Features

### Implemented features (based on docs/IMPLEMENTATION_ROADMAP.md)

- **Run Tracker** – active run monitoring, cancellation, metrics (P1.1)
- **Advanced Tool Calling** – parallel execution, exponential back‑off retries, argument validation, error handling (P1.2)
- **Reasoning Steps Persistence** – store model reasoning steps, support for o1 models (P1.3)
- **Guardrails** – prompt‑injection protection, harmful content filtering, rate‑limiting, infinite‑loop detection, semantic similarity checks (P1.4)
- **Memory Management** – summarization, automatic classification, pruning, semantic & hybrid search (P2.1)
- **Knowledge Base** – similarity search, PDF/DOCX/TXT support, intelligent chunking, incremental updates (P2.2)
- **Team Collaboration** – inter‑agent communication, task delegation, response aggregation, conflict resolution (P2.3)
- **Workflow V2** – conditional loops, parallel steps, dynamic routing, error handling (P2.4)
- **Vector Databases** – Qdrant & PgVector integrations with filtered searches, reranking, batch ops, hybrid search, testcontainers support (P3.1)
- **Tool Ecosystem** – GitHub, Slack, Email, Database Query tools plus core tools (Echo, Shell, File, Web, Math, DuckDuckGo, Weather, HackerNews, Exa) (P3.2)
- **MCP (Model Context Protocol)** – discovery and execution of MCP‑provided tools (P3.3)
- **Embedders** – OpenAI & Ollama embedding generation (partial, P3.4)

A concise summary of all functionalities already implemented and ready for the pre‑release. See the detailed description in **[Pre‑release Description](docs/PRE_RELEASE_DESCRIPTION.md)**.

### 1️⃣ Agent
- Full run tracker (monitoring, cancellation, metrics)
- Advanced tool calling (parallel execution, exponential‑backoff retries, validation, error handling)
- Reasoning steps persistence
- Complete guardrails (prompt‑injection protection, harmful‑content filtering, rate‑limiting, infinite‑loop detection, semantic similarity checks)

### 2️⃣ Memory
- Advanced management (summarization, automatic classification, pruning of stale memory)
- Semantic search (embedding‑based, hybrid (semantic + keyword), keyword‑only)
- SQLite persistence for durable storage

### 3️⃣ Knowledge Base
- Full RAG pipeline (similarity‑based retrieval, PDF/DOCX/TXT support, intelligent chunking, incremental updates)
- Direct agent integration for real‑time querying and updating

### 4️⃣ Team Collaboration
- Inter‑agent communication, task delegation, response aggregation
- Automatic conflict detection and intelligent resolution

### 5️⃣ Workflow V2
- Conditional loops, parallel steps, dynamic routing, error handling

### 6️⃣ Vector Databases
- Qdrant & PgVector integrations (filtered searches, reranking, batch ops, hybrid search, testcontainers support)

### 7️⃣ Tools
- Expanded ecosystem: GitHub, Slack, Email, Database Query, plus core tools (Echo, Shell, File, Web, Math, DuckDuckGo, Weather, HackerNews, Exa)
- Full MCP (Model Context Protocol) support for discovery and execution of external tools

### 8️⃣ Embedders
- OpenAI & Ollama embedding generation (unified interface, mock implementation for testing)

### 9️⃣ REST API (OS)
- Key endpoints for agent management, sessions, knowledge, memory, basic metrics, and health checks

### 🔟 Tests & Examples
- >100 unit tests and benchmarks covering the above areas
- Cookbooks with practical examples for each feature (tool calling, memory, knowledge, team, workflow, vector DB, etc.)

All **critical (P1)** and **high‑priority (P2)** features are **100 % complete**. Vector DB and tool ecosystem (P3) already have functional implementations for Qdrant, PgVector, GitHub, Slack, and Email, ready for use.

---

## Quick Start

### Installation
```bash
git clone https://github.com/devalexandre/agno-golang.git
cd agno-golang
go mod download
```

### Simple Agent Example
```go
package main

import (
    "github.com/devalexandre/agno-golang/agno/agent"
    "github.com/devalexandre/agno-golang/agno/models/openai/chat"
    "github.com/devalexandre/agno-golang/agno/tools"
)

func main() {
    ag := agent.NewAgent(chat.NewOpenAIChat("gpt-4o"))
    ag.AddTool(tools.NewWebTool())
    ag.AddTool(tools.NewMathTool())
    ag.PrintResponse("What is 15 + 25 and search for AI news?", false, true)
}
```

### Knowledge Base with Vector Search
```go
import (
    "github.com/devalexandre/agno-golang/agno/knowledge"
    "github.com/devalexandre/agno-golang/agno/vectordb/qdrant"
    "github.com/devalexandre/agno-golang/agno/embedder"
)

emb := embedder.NewOpenAIEmbedder()
vecDB, _ := qdrant.NewQdrant(qdrant.QdrantConfig{
    Host: "localhost", Port: 6333,
    Collection: "knowledge", Embedder: emb,
})

kb := knowledge.NewKnowledgeBase(vecDB)
_ = kb.LoadFromPDFs([]string{"manual.pdf", "docs.pdf"})
results, _ := kb.Search("How to configure the system?", 5)
```

### Reasoning Agent Example
```go
import (
    "github.com/devalexandre/agno-golang/agno/agent"
    "github.com/devalexandre/agno-golang/agno/models/openai"
)

ctx := context.WithValue(context.Background(), "reasoning", true)
openaiChat := openai.NewOpenAIChat("o1-preview")
reasoner := agent.NewReasoningAgent(ctx, openaiChat, tools, 5, 3)
steps, _ := reasoner.Reason("Analyze quarterly sales data and give strategic recommendations")
for _, s := range steps {
    fmt.Printf("Step: %s (Confidence: %.2f)\n", s.Reasoning, s.Confidence)
}
```

---

## Architecture Overview
```
agno-golang/
├── agno/
│   ├── agent/           # 🤖 Agent system
│   ├── models/          # 🧠 LLM providers (OpenAI, Ollama, Gemini)
│   ├── tools/           # 🛠️ 8‑tool suite + expanded ecosystem
│   │   ├── toolkit/     # 🔧 Tool registration
│   │   └── exa/         # 🔍 Advanced web search
│   ├── knowledge/       # 📚 Knowledge base with RAG
│   ├── vectordb/        # 🗄️ Vector storage (Qdrant, pgvector)
│   ├── embedder/        # 🧠 Embedding generation
│   └── utils/           # 🔨 Utilities
└── docs/                # 📖 Documentation
```

---

## License
This project is licensed under the MPL‑2.0 License – see the [LICENSE](LICENSE) file.

---

*Building the future of AI agents, one goroutine at a time.* 🚀
