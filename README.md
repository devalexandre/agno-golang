# Agno Golang

Implementacao em Go do Agno para criar agentes de IA com modelos, ferramentas, memoria, conhecimento, workflows, times multiagente e uma camada HTTP opcional com AgentOS.

O foco do projeto e oferecer uma base idiomatica em Go para construir agentes que possam:

- conversar com diferentes provedores de LLM;
- executar ferramentas com schemas Go;
- consultar bases de conhecimento e bancos vetoriais;
- manter memoria de usuario, historico de sessoes e estado;
- coordenar agentes especializados;
- executar workflows duraveis e reprocessaveis;
- expor agentes, times e workflows por API REST/WebSocket.

## Status

O projeto esta em evolucao ativa. A documentacao abaixo descreve os recursos presentes no repositorio e aponta para cookbooks executaveis em `cookbook/` e guias mais detalhados em `docs/`.

## Requisitos

- Go `1.25+`
- Um provedor de modelo configurado, como Ollama local ou uma API externa
- Docker, opcional, para exemplos com Qdrant, PgVector e Testcontainers
- `poppler-utils`/`pdftotext`, opcional, para ingestao de PDFs

Para comecar com Ollama local:

```bash
ollama serve
ollama pull llama3.2:latest
```

Instale o modulo em outro projeto:

```bash
go get github.com/devalexandre/agno-golang
```

Ou rode os exemplos direto neste repositorio:

```bash
go mod download
go run ./cookbook/getting_started/01_basic_agent
```

## Conceitos Principais

**Agent** e a unidade central. Ele combina modelo, instrucoes, ferramentas, conhecimento, memoria, storage, schemas, guardrails, hooks e opcoes de execucao.

**Model Provider** implementa `models.AgnoModelInterface`. Isso permite alternar entre OpenAI, Ollama, Anthropic, Groq e outros provedores mantendo a mesma API de agente.

**Tool** e uma acao que o agente pode chamar. Ferramentas usam structs Go como schema de parametros e sao registradas via `toolkit.Toolkit`.

**Knowledge** e a camada de RAG. Ela guarda documentos, faz busca textual/vetorial e pode usar Qdrant, PgVector, Chroma, Pinecone, Milvus ou Weaviate.

**Memory** guarda fatos reutilizaveis sobre usuarios e sessoes. Storage guarda historico de runs, sessoes e estado operacional.

**Workflow V2** organiza steps, paralelismo, roteamento, condicoes, loops, streaming e checkpoints duraveis.

**Team** coordena varios agentes especializados em modos de roteamento, coordenacao ou colaboracao.

**Skills** adicionam conhecimento procedural ao agente. Diferente de tools, uma skill ensina quando e como agir usando instrucoes, scripts e referencias sob demanda.

**AgentOS** expoe agentes, times, workflows, conhecimento, memoria e metricas por API HTTP e WebSocket.

## Recursos Disponiveis

### Modelos

Provedores implementados:

- OpenAI e endpoints OpenAI-like
- Ollama local e Ollama Cloud
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

Exemplo rapido com Ollama:

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
		Name:         "Assistente",
		Instructions: "Responda de forma objetiva e pratica.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	resp, err := ag.Run("Explique em uma frase por que Go combina com agentes de IA.")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.TextContent)
}
```

Exemplo com OpenAI:

```go
model, err := chat.NewOpenAIChat(models.WithID("gpt-4o"))
ag, err := agent.NewAgent(agent.AgentConfig{
	Model:       model,
	Description: "Voce e um assistente tecnico.",
})
resp, err := ag.Run("Resuma RAG em 3 bullets.")
```

## Agents

O `agent.AgentConfig` concentra os principais recursos:

- `Instructions`, `Role`, `Goal`, `ExpectedOutput` e `ContextData`
- streaming, markdown, debug e exibicao de tool calls
- tools, skills, guardrails e hooks
- input/output schemas e parser/output model
- knowledge e filtros por run
- memoria de usuario, resumo de sessao e historico
- estado de sessao, dependencias e contexto adicional
- retries, backoff, limite de tool calls e escolha de tool

### Agent com ferramentas

```go
searchTool := tools.NewDuckDuckGoTool()
mathTool := tools.NewMathTool()

ag, err := agent.NewAgent(agent.AgentConfig{
	Context:      context.Background(),
	Model:        model,
	Name:         "Researcher",
	Instructions: "Use ferramentas quando elas ajudarem a responder melhor.",
	Tools:        []toolkit.Tool{searchTool, mathTool},
	ShowToolsCall: true,
})
if err != nil {
	log.Fatal(err)
}

resp, err := ag.Run("Pesquise sobre Go 1.25 e calcule 25 * 4.")
```

Ferramentas prontas incluem:

- busca e web: DuckDuckGo, Google Search, Exa, Tavily, Serper, SerpAPI, Firecrawl, Crawl4AI, Wikipedia, Hacker News, PubMed, arXiv, Reddit, YouTube, Newspaper
- arquivos e sistema: FileTool, ShellTool, SystemTools, OSCommandExecutor, Go build/test, Docker, Kubernetes, Git
- dados: SQL, PostgreSQL, DuckDB, CSV/Excel, YFinance, database helpers
- comunicacao: Slack, Gmail, Email, Telegram, Discord, WhatsApp, Google Calendar
- produtividade: GitHub, Jira, Notion, Confluence, Google Drive, Google Sheets
- cloud: AWS, GCP e Azure
- utilitarios: Math, Calculator, Weather, Cache, Monitoring, API client, Webhook, temporal planner, dependency inspector, performance profiler, self-validation gate
- MCP: descoberta e execucao de ferramentas fornecidas por servidores Model Context Protocol

### Criando uma ferramenta customizada

```go
type StatusTool struct {
	toolkit.Toolkit
}

type StatusParams struct {
	Service string `json:"service" description:"Nome do servico"`
}

func (t *StatusTool) Check(params StatusParams) (string, error) {
	return "servico " + params.Service + " esta operacional", nil
}

status := &StatusTool{Toolkit: toolkit.NewToolkit()}
status.Name = "status"
status.Description = "Consulta status de servicos internos"
status.Register("Check", "Verifica status de um servico", status, status.Check, StatusParams{})

ag, _ := agent.NewAgent(agent.AgentConfig{
	Model: model,
	Tools: []toolkit.Tool{status},
})
```

## Saida Estruturada

Use `OutputSchema` e `ParseResponse` quando o resultado precisar voltar como struct Go em vez de texto livre.

```go
type Plano struct {
	Titulo string   `json:"titulo" description:"Titulo curto do plano"`
	Passos []string `json:"passos" description:"Passos objetivos"`
}

plano := &Plano{}

ag, err := agent.NewAgent(agent.AgentConfig{
	Context:       context.Background(),
	Model:         model,
	Instructions:  "Gere respostas em JSON compativel com o schema.",
	OutputSchema:  plano,
	ParseResponse: true,
})
if err != nil {
	log.Fatal(err)
}

run, err := ag.Run("Crie um plano de 3 passos para revisar um PR.")
if err != nil {
	log.Fatal(err)
}

fmt.Println(plano.Titulo)
fmt.Println(run.Output.(*Plano).Passos)
```

Tambem ha suporte para `InputSchema`, ponteiro para slice em `OutputSchema`, `OutputModel` e `ParserModel`. Veja `docs/agent/INPUT_OUTPUT_SCHEMA.md` e `docs/agent/OUTPUT_MODEL.md`.

## Knowledge, RAG e Vector DBs

Tipos de conhecimento disponiveis:

- `BaseKnowledge`
- `DocumentKnowledgeBase`
- `TextKnowledgeBase`
- `JSONKnowledgeBase`
- `PDFKnowledgeBase`
- `RAGPipeline` com interface de reranker

Bancos vetoriais:

- Qdrant
- PgVector
- Chroma
- Pinecone
- Milvus
- Weaviate

Embedders:

- OpenAI
- Ollama
- Mock embedder para testes

Exemplo com documentos em memoria:

```go
kb := knowledge.NewDocumentKnowledgeBase("docs", nil)

doc := document.NewDocument("Agno permite criar agentes com tools, knowledge e memory em Go.")
doc.Name = "intro"
doc.AddMetadata("source", "manual")

kb.AddDocument(doc)

ag, err := agent.NewAgent(agent.AgentConfig{
	Context:               context.Background(),
	Model:                 model,
	Knowledge:             kb,
	KnowledgeMaxDocuments: 3,
	Instructions:          "Use a base de conhecimento quando ela for relevante.",
})
```

Exemplo com Qdrant e Ollama embeddings:

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

## Memory e Storage

Use `memory.Memory` quando quiser guardar preferencias, fatos ou resumos sobre usuarios. Use `storage.DB` quando quiser persistir sessoes, runs e historico operacional.

Exemplo de memoria de usuario com SQLite:

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

resp, err := ag.Run("Sou vegetariano e quero ganhar forca sem forcar o joelho.")
if err == nil {
	_, _ = memoryManager.CreateMemory(context.Background(), "user_123", "perfil inicial", resp.TextContent)
}
```

Storage de sessoes esta disponivel em SQLite e PostgreSQL. Veja `cookbook/getting_started/06_agent_with_storage` e `agno/storage/`.

## Workflows

Workflow V2 suporta steps sequenciais, steps paralelos, condicoes, loops, roteadores, streaming, WebSocket e checkpoints duraveis.

```go
step1, _ := v2.NewStep(
	v2.WithName("coletar-dados"),
	v2.WithExecutor(func(input *v2.StepInput) (*v2.StepOutput, error) {
		return &v2.StepOutput{Content: "dados coletados"}, nil
	}),
)

step2, _ := v2.NewStep(
	v2.WithName("gerar-relatorio"),
	v2.WithAgent(writerAgent),
)

workflow := v2.NewWorkflow(
	v2.WithWorkflowName("Relatorio"),
	v2.WithWorkflowSteps([]*v2.Step{step1, step2}),
	v2.WithStreaming(true, true),
)

workflow.PrintResponse("Crie um relatorio executivo", true)
```

Workflows duraveis usam `WithStorage`, `WithSessionID` e `WithDurable(true)` para salvar checkpoints e retomar do ultimo step concluido. Veja `cookbook/durable_workflow/main.go`.

## Teams Multiagente

Times permitem combinar agentes especializados. Modos disponiveis:

- `team.RouteMode`: roteia para o membro mais adequado
- `team.CoordinateMode`: delega tarefas e sintetiza respostas
- `team.CollaborateMode`: todos trabalham no mesmo problema e o lider sintetiza

```go
contentTeam := team.NewTeam(team.TeamConfig{
	Context:     context.Background(),
	Name:        "Content Team",
	Description: "Time para pesquisa, escrita e revisao",
	Model:       model,
	Members:     []*agent.Agent{researchAgent, writerAgent, editorAgent},
	Mode:        team.CoordinateMode,
	Async:       true,
})

resp, err := contentTeam.Run("Crie um artigo curto sobre Go para agentes de IA.")
```

## Skills

Skills ficam em diretorios com `SKILL.md`, scripts e referencias opcionais:

```text
my-skill/
  SKILL.md
  scripts/
  references/
```

Skills embutidas no repositorio:

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

Ativando skills especificas:

```go
ag, err := agent.NewAgent(agent.AgentConfig{
	Context:     context.Background(),
	Model:       model,
	Name:        "Dev Agent",
	SkillsToUse: []string{"github", "summarize"},
})
```

Adicionando skills locais:

```go
loader := skill.NewLocalSkills("./my-custom-skills")

ag, err := agent.NewAgent(agent.AgentConfig{
	Model:              model,
	SkillsToUse:        []string{"github"},
	CustomSkillsLoader: loader,
})
```

## MCP

O pacote `agno/tools/mcp` conecta agentes a servidores MCP e registra as ferramentas descobertas dinamicamente como `toolkit.Tool`.

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

Exemplo completo: `cookbook/mcp/main.go`.

## Learning Loop e Culture Manager

O Learning Loop adiciona aprendizado continuo sobre uma base `knowledge.Knowledge`:

- antes do run, recupera memorias relevantes via RAG;
- depois do run, decide se deve salvar um artefato reutilizavel;
- deduplica com busca vetorial e SimHash;
- promove candidatos para verificados com feedback positivo.

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

O Culture Manager guarda perfil leve de usuario, como idioma preferido, timezone, estilo de comunicacao e interesses, para personalizar respostas sem misturar isso com conhecimento factual.

## AgentOS

AgentOS expoe agentes, times e workflows como API HTTP, dashboard e WebSocket.

```go
assistant, _ := agent.NewAgent(agent.AgentConfig{
	Context:      context.Background(),
	Name:         "Assistant",
	Model:        model,
	Instructions: "Voce e um assistente util.",
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

Endpoints principais:

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

Veja `agno/os/README.md` e `cookbook/os-example/main.go`.

## Guardrails, Hooks e Execucao Segura

O agente suporta:

- input, output e tool guardrails;
- protecao contra prompt injection, limite de tamanho, rate limit, loop detection e similaridade semantica;
- `PreHooks`, `PostHooks`, `ToolBeforeHooks` e `ToolAfterHooks`;
- `ToolCallLimit`, `ToolChoice`, retries e exponential backoff;
- `FileTool` com escrita desabilitada por padrao;
- ferramentas separadas para shell/OS, que devem ser usadas com politica clara em ambientes produtivos.

## Cookbooks Recomendados

Comece por estes exemplos:

```bash
go run ./cookbook/getting_started/01_basic_agent
go run ./cookbook/getting_started/02_agent_with_tools
go run ./cookbook/getting_started/03_agent_with_knowledge
go run ./cookbook/agents/input_and_output/output
go run ./cookbook/workflow_prompt/basic
go run ./cookbook/mcp
```

Outros diretorios uteis:

- `cookbook/agents/`: agentes, tools, guardrails, memory, state, skills e teams
- `cookbook/getting_started/`: exemplos progressivos de uso basico
- `cookbook/vectordb/`: Qdrant, PgVector, Chroma e Pinecone
- `cookbook/tools/`: ferramentas prontas e integracoes
- `cookbook/models/`: provedores de modelo
- `cookbook/durable_workflow/`: checkpoints e resume
- `cookbook/observability/`: OpenTelemetry
- `cookbook/agentos-ollama-cloud/`: AgentOS com Ollama Cloud

## Documentacao

- `docs/agent/`: agentes, schemas e output model
- `docs/RUN_OPTIONS.md`: opcoes de execucao por run
- `docs/tools/`: ferramentas, seguranca e MCP
- `docs/knowledge/`: knowledge bases e PDFs
- `docs/vectordb/`: bancos vetoriais
- `docs/embedder/`: embeddings
- `docs/learning/`: learning loop e diferenca para culture
- `docs/skills/`: sistema de skills
- `docs/flow/` e `docs/chain/`: fluxos e chain tools
- `agno/os/README.md`: AgentOS

## Variaveis de Ambiente Frequentes

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

## Estrutura do Repositorio

```text
agno/
  agent/        agentes, schemas, guardrails, reasoning, tools default
  models/       provedores de LLM
  tools/        ferramentas e toolkits
  knowledge/    RAG, documentos, PDFs e pipeline
  vectordb/     Qdrant, PgVector, Chroma, Pinecone, Milvus, Weaviate
  embedder/     OpenAI, Ollama e mock embeddings
  memory/       memoria de usuario e sumarios
  storage/      SQLite e PostgreSQL para sessoes/runs
  workflow/v2/  workflows, steps, paralelo, durable e websocket
  team/         times multiagente
  skill/        carregamento e execucao de skills
  learning/     aprendizado continuo sobre knowledge
  culture/      perfil leve de usuario
  os/           AgentOS HTTP/WebSocket
cookbook/       exemplos executaveis
docs/           guias de uso
skills/         skills embutidas
```
