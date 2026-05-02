# Agno Golang

Go implementation of Agno for building AI agents with models, tools, memory, knowledge, workflows, multi-agent teams, and an optional HTTP layer through AgentOS.

The project focuses on providing an idiomatic Go foundation for agents that can:

- talk to different LLM providers;
- execute tools with Go schemas;
- query knowledge bases and vector databases;
- keep user memory, session history, and runtime state;
- coordinate specialized agents;
- execute durable and resumable workflows;
- expose agents, teams, and workflows through REST/WebSocket APIs.

## Status

This project is under active development. The documentation below describes the resources currently available in the repository and points to runnable cookbooks in `cookbook/` and deeper guides in `docs/`.

## Requirements

- Go `1.25+`
- A configured model provider, such as local Ollama or an external API
- Docker, optional, for Qdrant, PgVector, and Testcontainers examples
- `poppler-utils`/`pdftotext`, optional, for PDF ingestion

To start with local Ollama:

```bash
ollama serve
ollama pull llama3.2:latest
```

Install the module in another project:

```bash
go get github.com/devalexandre/agno-golang
```

Or run examples directly from this repository:

```bash
go mod download
go run ./cookbook/getting_started/01_basic_agent
```

## Core Concepts

**Agent** is the central unit. It combines a model, instructions, tools, knowledge, memory, storage, schemas, guardrails, hooks, and run options.

**Model Provider** implements `models.AgnoModelInterface`. This lets you switch between OpenAI, Ollama, Anthropic, Groq, and other providers while keeping the same agent API.

**Tool** is an action the agent can call. Tools use Go structs as parameter schemas and are registered through `toolkit.Toolkit`.

**Knowledge** is the RAG layer. It stores documents, performs text/vector search, and can use Qdrant, PgVector, Chroma, Pinecone, Milvus, or Weaviate.

**Memory** stores reusable facts about users and sessions. Storage stores run history, sessions, and operational state.

**Workflow V2** organizes steps, parallel execution, routing, conditions, loops, streaming, and durable checkpoints.

**Flow** is a fluent API on top of Workflow V2. It lets you build workflows with chainable calls like `Step`, `If/Else`, `Loop`, `Parallel`, and `Router`.

**Team** coordinates multiple specialized agents in routing, coordination, or collaboration modes.

**Skills** add procedural knowledge to an agent. Unlike tools, a skill teaches the agent when and how to act using instructions, scripts, and references loaded on demand.

**AgentOS** exposes agents, teams, workflows, knowledge, memory, and metrics through HTTP and WebSocket APIs.

## Available Features

### Models

Implemented providers:

- OpenAI and OpenAI-like endpoints
- Local Ollama and Ollama Cloud
- Google Gemini
- Anthropic
- DeepSeek
- Groq
- AWS Bedrock
- Azure OpenAI
- OpenRouter
- Together AI
- DashScope
- vLLM

Quick Ollama example:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
)

func main() {
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      context.Background(),
		Model:        model,
		Name:         "Assistant",
		Instructions: "Answer in a practical and concise way.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	resp, err := ag.Run("Explain in one sentence why Go works well for AI agents.")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.TextContent)
}
```

OpenAI example:

```go
model, err := chat.NewOpenAIChat(models.WithID("gpt-4o"))
ag, err := agent.NewAgent(agent.AgentConfig{
	Model:       model,
	Description: "You are a technical assistant.",
})
resp, err := ag.Run("Summarize RAG in 3 bullets.")
```

## Agents

`agent.AgentConfig` concentrates the main capabilities:

- `Instructions`, `Role`, `Goal`, `ExpectedOutput`, and `ContextData`
- streaming, markdown, debug, and tool-call display
- tools, skills, guardrails, and hooks
- input/output schemas and parser/output model
- knowledge and per-run filters
- user memory, session summaries, and history
- session state, dependencies, and extra context
- retries, backoff, tool-call limits, and tool choice

### Agent With Tools

```go
searchTool := tools.NewDuckDuckGoTool()
mathTool := tools.NewMathTool()

ag, err := agent.NewAgent(agent.AgentConfig{
	Context:       context.Background(),
	Model:         model,
	Name:          "Researcher",
	Instructions:  "Use tools whenever they help you answer better.",
	Tools:         []toolkit.Tool{searchTool, mathTool},
	ShowToolsCall: true,
})
if err != nil {
	log.Fatal(err)
}

resp, err := ag.Run("Research Go 1.25 and calculate 25 * 4.")
```

Built-in tools include:

- search and web: DuckDuckGo, Google Search, Exa, Tavily, Serper, SerpAPI, Firecrawl, Crawl4AI, Wikipedia, Hacker News, PubMed, arXiv, Reddit, YouTube, Newspaper
- files and system: FileTool, ShellTool, SystemTools, OSCommandExecutor, Go build/test, Docker, Kubernetes, Git
- data: SQL, PostgreSQL, DuckDB, CSV/Excel, YFinance, database helpers
- communication: Slack, Gmail, Email, Telegram, Discord, WhatsApp, Google Calendar
- productivity: GitHub, Jira, Notion, Confluence, Google Drive, Google Sheets
- cloud: AWS, GCP, and Azure
- utilities: Math, Calculator, Weather, Cache, Monitoring, API client, Webhook, temporal planner, dependency inspector, performance profiler, self-validation gate
- MCP: discovery and execution of tools provided by Model Context Protocol servers

### Creating a Custom Tool

```go
type StatusTool struct {
	toolkit.Toolkit
}

type StatusParams struct {
	Service string `json:"service" description:"Service name"`
}

func (t *StatusTool) Check(params StatusParams) (string, error) {
	return "service " + params.Service + " is operational", nil
}

status := &StatusTool{Toolkit: toolkit.NewToolkit()}
status.Name = "status"
status.Description = "Checks internal service status"
status.Register("Check", "Checks a service status", status, status.Check, StatusParams{})

ag, _ := agent.NewAgent(agent.AgentConfig{
	Model: model,
	Tools: []toolkit.Tool{status},
})
```

## Structured Output

Use `OutputSchema` and `ParseResponse` when the result must come back as a Go struct instead of free-form text.

```go
type Plan struct {
	Title string   `json:"title" description:"Short plan title"`
	Steps []string `json:"steps" description:"Objective steps"`
}

plan := &Plan{}

ag, err := agent.NewAgent(agent.AgentConfig{
	Context:       context.Background(),
	Model:         model,
	Instructions:  "Generate JSON responses compatible with the schema.",
	OutputSchema:  plan,
	ParseResponse: true,
})
if err != nil {
	log.Fatal(err)
}

run, err := ag.Run("Create a 3-step plan to review a PR.")
if err != nil {
	log.Fatal(err)
}

fmt.Println(plan.Title)
fmt.Println(run.Output.(*Plan).Steps)
```

`InputSchema`, pointer-to-slice `OutputSchema`, `OutputModel`, and `ParserModel` are also supported. See `docs/agent/INPUT_OUTPUT_SCHEMA.md` and `docs/agent/OUTPUT_MODEL.md`.

## Knowledge, RAG, and Vector DBs

Available knowledge types:

- `BaseKnowledge`
- `DocumentKnowledgeBase`
- `TextKnowledgeBase`
- `JSONKnowledgeBase`
- `PDFKnowledgeBase`
- `RAGPipeline` with a reranker interface

Vector databases:

- Qdrant
- PgVector
- Chroma
- Pinecone
- Milvus
- Weaviate

Embedders:

- OpenAI
- Ollama
- Mock embedder for tests

In-memory document example:

```go
kb := knowledge.NewDocumentKnowledgeBase("docs", nil)

doc := document.NewDocument("Agno lets you build agents with tools, knowledge, and memory in Go.")
doc.Name = "intro"
doc.AddMetadata("source", "manual")

kb.AddDocument(doc)

ag, err := agent.NewAgent(agent.AgentConfig{
	Context:               context.Background(),
	Model:                 model,
	Knowledge:             kb,
	KnowledgeMaxDocuments: 3,
	Instructions:          "Use the knowledge base when it is relevant.",
})
```

Qdrant with Ollama embeddings:

```go
emb := embedder.NewOllamaEmbedder(
	embedder.WithOllamaModel("nomic-embed-text", 768),
	embedder.WithOllamaHost("http://localhost:11434"),
)

vectorDB, err := qdrant.NewQdrant(qdrant.QdrantConfig{
	Host:       "localhost",
	Port:       6334,
	Collection: "knowledge",
	Embedder:   emb,
	SearchType: vectordb.SearchTypeVector,
	Distance:   vectordb.DistanceCosine,
})
if err != nil {
	log.Fatal(err)
}

kb := knowledge.NewPDFKnowledgeBase("pdfs", vectorDB)
kb.URLs = []string{"https://arxiv.org/pdf/2305.13245.pdf"}

err = kb.LoadParallel(context.Background(), true, 3)
```

## Memory and Storage

Use `memory.Memory` when you want to store user preferences, facts, or summaries. Use `storage.DB` when you want to persist sessions, runs, and operational history.

SQLite user memory example:

```go
memoryDB, err := memorysqlite.NewSqliteMemoryDb("user_memories", "agent_memory.db")
if err != nil {
	log.Fatal(err)
}

memoryManager := memory.NewMemory(model, memoryDB)

ag, err := agent.NewAgent(agent.AgentConfig{
	Context:   context.Background(),
	Model:     model,
	Name:      "PersonalCoach",
	UserID:    "user_123",
	SessionID: "session_001",
	Memory:    memoryManager,
})
if err != nil {
	log.Fatal(err)
}

resp, err := ag.Run("I am vegetarian and want to build strength without stressing my knee.")
if err == nil {
	_, _ = memoryManager.CreateMemory(context.Background(), "user_123", "initial profile", resp.TextContent)
}
```

Session storage is available in SQLite and PostgreSQL. See `cookbook/getting_started/06_agent_with_storage` and `agno/storage/`.

## Workflows and Flow

Workflow V2 supports sequential steps, parallel steps, conditions, loops, routers, streaming, WebSocket, and durable checkpoints.

```go
step1, _ := v2.NewStep(
	v2.WithName("collect-data"),
	v2.WithExecutor(func(input *v2.StepInput) (*v2.StepOutput, error) {
		return &v2.StepOutput{Content: "data collected"}, nil
	}),
)

step2, _ := v2.NewStep(
	v2.WithName("generate-report"),
	v2.WithAgent(writerAgent),
)

workflow := v2.NewWorkflow(
	v2.WithWorkflowName("Report"),
	v2.WithWorkflowSteps([]*v2.Step{step1, step2}),
	v2.WithStreaming(true, true),
)

workflow.PrintResponse("Create an executive report", true)
```

The new `agno/flow` package provides a fluent API on top of Workflow V2 for simpler workflow construction:

```go
workflow := flow.New("Report Flow").
	Description("Collects data and generates a report").
	Step("collect-data", func(input *v2.StepInput) (*v2.StepOutput, error) {
		return &v2.StepOutput{Content: "data collected"}, nil
	}).
	If(flow.IfSuccess(),
		func(input *v2.StepInput) (*v2.StepOutput, error) {
			return &v2.StepOutput{Content: "report generated"}, nil
		},
	).
	Build()

workflow.PrintResponse("Start report", true)
```

Durable workflows use `WithStorage`, `WithSessionID`, and `WithDurable(true)` to save checkpoints and resume from the last completed step. See `cookbook/durable_workflow/main.go`.

## Multi-Agent Teams

Teams let you combine specialized agents. Available modes:

- `team.RouteMode`: routes to the most appropriate member
- `team.CoordinateMode`: delegates tasks and synthesizes responses
- `team.CollaborateMode`: all members work on the same problem and the leader synthesizes

```go
contentTeam := team.NewTeam(team.TeamConfig{
	Context:     context.Background(),
	Name:        "Content Team",
	Description: "Team for research, writing, and review",
	Model:       model,
	Members:     []*agent.Agent{researchAgent, writerAgent, editorAgent},
	Mode:        team.CoordinateMode,
	Async:       true,
})

resp, err := contentTeam.Run("Create a short article about Go for AI agents.")
```

## Skills

Skills live in directories with `SKILL.md`, optional scripts, and optional references:

```text
my-skill/
  SKILL.md
  scripts/
  references/
```

Built-in skills in this repository:

- `github`
- `slack`
- `discord`
- `notion`
- `trello`
- `weather`
- `summarize`
- `obsidian`
- `coding-agent`
- `skill-creator`

Activating specific skills:

```go
ag, err := agent.NewAgent(agent.AgentConfig{
	Context:     context.Background(),
	Model:       model,
	Name:        "Dev Agent",
	SkillsToUse: []string{"github", "summarize"},
})
```

Adding local skills:

```go
loader := skill.NewLocalSkills("./my-custom-skills")

ag, err := agent.NewAgent(agent.AgentConfig{
	Model:              model,
	SkillsToUse:        []string{"github"},
	CustomSkillsLoader: loader,
})
```

## MCP

The `agno/tools/mcp` package connects agents to MCP servers and registers dynamically discovered tools as `toolkit.Tool`.

```go
workspace, err := os.Getwd()
if err != nil {
	log.Fatal(err)
}

mcpTool, err := mcp.NewMCPTool(
	"MCPTools",
	fmt.Sprintf("docker run --rm -i --mount type=bind,src=%s,dst=/workspace mcp/filesystem /workspace", workspace),
)
if err != nil {
	log.Fatal(err)
}
defer mcpTool.Close()

if err := mcpTool.Connect(context.Background()); err != nil {
	log.Fatal(err)
}

ag, err := agent.NewAgent(agent.AgentConfig{
	Model: model,
	Tools: []toolkit.Tool{mcpTool},
})
```

Full example: `cookbook/mcp/main.go`.

## Learning Loop and Culture Manager

The Learning Loop adds continuous learning on top of a `knowledge.Knowledge` store:

- before the run, it retrieves relevant memories via RAG;
- after the run, it decides whether to save a reusable artifact;
- it deduplicates with vector search and SimHash;
- it promotes candidates to verified memories with positive feedback.

```go
kb := knowledge.NewBaseKnowledge("learning", vectorDB)
lm := learning.NewManager(kb, learning.DefaultManagerConfig())

ag, err := agent.NewAgent(agent.AgentConfig{
	Model:     model,
	UserID:    "user_123",
	Knowledge: kb,
	Learning:  lm,
})
```

The Culture Manager stores a lightweight user profile, such as preferred language, timezone, communication style, and interests, to personalize responses without mixing that data with factual knowledge.

## AgentOS

AgentOS exposes agents, teams, and workflows as an HTTP API, dashboard, and WebSocket server.

```go
assistant, _ := agent.NewAgent(agent.AgentConfig{
	Context:      context.Background(),
	Name:         "Assistant",
	Model:        model,
	Instructions: "You are a helpful assistant.",
})

osInstance, err := agentOS.NewAgentOS(agentOS.AgentOSOptions{
	OSID:   "my-os",
	Agents: []*agent.Agent{assistant},
	Settings: &agentOS.AgentOSSettings{
		Port:       8080,
		Host:       "0.0.0.0",
		EnableCORS: true,
	},
})
if err != nil {
	log.Fatal(err)
}

log.Fatal(osInstance.Serve())
```

Main endpoints:

- `GET /health`
- `GET /config`
- `GET /api/v1/agents`
- `POST /api/v1/agents/:id/chat`
- `GET /api/v1/teams`
- `POST /api/v1/teams/:id/chat`
- `GET /api/v1/workflows`
- `POST /api/v1/workflows/:id/run`
- `GET /api/v1/knowledge`
- `GET /api/v1/memory`
- `GET /api/v1/metrics`

See `agno/os/README.md` and `cookbook/os-example/main.go`.

## Guardrails, Hooks, and Safe Execution

The agent supports:

- input, output, and tool guardrails;
- prompt-injection protection, input length limits, rate limiting, loop detection, and semantic similarity checks;
- `PreHooks`, `PostHooks`, `ToolBeforeHooks`, and `ToolAfterHooks`;
- `ToolCallLimit`, `ToolChoice`, retries, and exponential backoff;
- `FileTool` with writes disabled by default;
- separate shell/OS tools, which should be used with a clear policy in production environments.

## Recommended Cookbooks

Start with these examples:

```bash
go run ./cookbook/getting_started/01_basic_agent
go run ./cookbook/getting_started/02_agent_with_tools
go run ./cookbook/getting_started/03_agent_with_knowledge
go run ./cookbook/agents/input_and_output/output
go run ./cookbook/flow
go run ./cookbook/workflow_prompt/basic
go run ./cookbook/mcp
```

Useful directories:

- `cookbook/agents/`: agents, tools, guardrails, memory, state, skills, and teams
- `cookbook/getting_started/`: progressive getting-started examples
- `cookbook/flow/`: Flow fluent API example
- `cookbook/vectordb/`: Qdrant, PgVector, Chroma, and Pinecone
- `cookbook/tools/`: ready-to-use tools and integrations
- `cookbook/models/`: model providers
- `cookbook/durable_workflow/`: checkpoints and resume
- `cookbook/observability/`: OpenTelemetry
- `cookbook/agentos-ollama-cloud/`: AgentOS with Ollama Cloud

## Documentation

- `docs/agent/`: agents, schemas, and output model
- `docs/RUN_OPTIONS.md`: per-run execution options
- `docs/tools/`: tools, security, and MCP
- `docs/knowledge/`: knowledge bases and PDFs
- `docs/vectordb/`: vector databases
- `docs/embedder/`: embeddings
- `docs/learning/`: learning loop and the difference from culture
- `docs/skills/`: skills system
- `docs/flow/`: Flow fluent API
- `docs/chain/`: chain tools
- `agno/os/README.md`: AgentOS

## Common Environment Variables

```bash
export OPENAI_API_KEY="..."
export ANTHROPIC_API_KEY="..."
export DEEPSEEK_API_KEY="..."
export GROQ_API_KEY="..."
export TOGETHER_API_KEY="..."
export OPENROUTER_API_KEY="..."
export AZURE_OPENAI_API_KEY="..."
export AZURE_OPENAI_ENDPOINT="..."
export AZURE_OPENAI_DEPLOYMENT_NAME="..."
export EXA_API_KEY="..."
export OLLAMA_API_KEY="..."
```

## Repository Structure

```text
agno/
  agent/        agents, schemas, guardrails, reasoning, default tools
  models/       LLM providers
  tools/        tools and toolkits
  knowledge/    RAG, documents, PDFs, and pipeline
  vectordb/     Qdrant, PgVector, Chroma, Pinecone, Milvus, Weaviate
  embedder/     OpenAI, Ollama, and mock embeddings
  memory/       user memory and summaries
  storage/      SQLite and PostgreSQL for sessions/runs
  workflow/v2/  workflows, steps, parallel execution, durable mode, websocket
  flow/         fluent API for Workflow V2
  team/         multi-agent teams
  skill/        skill loading and execution
  learning/     continuous learning over knowledge
  culture/      lightweight user profile
  os/           AgentOS HTTP/WebSocket
cookbook/       runnable examples
docs/           usage guides
skills/         built-in skills
```
