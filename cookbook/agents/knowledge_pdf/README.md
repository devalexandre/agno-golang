# Knowledge Base with PDF - RAG Example

Este exemplo demonstra como usar a base de conhecimento (Knowledge Base) do Agno com:
- **Qdrant** rodando em container Docker
- **PDF grande** carregado e indexado rapidamente
- **Embeddings locais** usando Ollama (gemma:2b)
- **LLM na cloud** para geração (kimi-k2:1t-cloud)

## Arquitetura

```
┌─────────────────┐      ┌──────────────────┐      ┌─────────────────┐
│   Large PDF     │─────▶│ Local Ollama     │─────▶│ Qdrant Vector  │
│   Document      │      │ Embeddings       │      │ Database        │
│                 │      │ (gemma:2b)       │      │ (Container)     │
└─────────────────┘      └──────────────────┘      └─────────────────┘
                                                             │
                                                             ▼
                         ┌──────────────────┐      ┌─────────────────┐
                         │ Ollama Cloud     │◀─────│ Agent with      │
                         │ (kimi-k2:1t)     │      │ RAG             │
                         └──────────────────┘      └─────────────────┘
```

## Pré-requisitos

1. **Docker** instalado e rodando
2. **Ollama local** com o modelo `gemma:2b`:
   ```bash
   ollama pull gemma:2b
   ```
3. **API Key** do Ollama Cloud:
   ```bash
   export OLLAMA_API_KEY=your-api-key
   ```
4. **PDF** no caminho especificado

## Como Funciona

### 1. Inicialização do Qdrant
```go
qdrantContainer, err := qdrantcontainer.Run(ctx, "qdrant/qdrant:v1.7.4")
```
- Inicia um container Qdrant automaticamente
- Obtém o endpoint HTTP para conexão
- Cleanup automático ao finalizar

### 2. Embeddings Locais
```go
localEmbedder := embedder.NewOllamaEmbedder(
    embedder.WithModel("gemma:2b"),
    embedder.WithBaseURL("http://localhost:11434"),
)
```
- Usa Ollama local para gerar embeddings
- Modelo `gemma:2b` (2048 dimensões)
- **Vantagem**: Embeddings rápidos e sem custo de API

### 3. Vector Database
```go
vectorDB := qdrant.New(
    qdrant.WithURL(qdrantHost),
    qdrant.WithCollectionName("mistral_knowledge"),
    qdrant.WithDimension(2048),
    qdrant.WithDistance("cosine"),
)
```
- Cria coleção no Qdrant
- Usa distância cosine para similaridade
- Dimensão 2048 (compatível com gemma:2b)

### 4. Carregamento do PDF
```go
err = kb.LoadDocumentFromPath(ctx, pdfPath, nil)
```
- Carrega e processa o PDF automaticamente
- Divide em chunks otimizados
- **Otimização automática**:
  - PDFs pequenos (< 500 chunks): Processamento sequencial com progressbar
  - PDFs grandes (≥ 500 chunks): **Processamento paralelo** com 5 workers
  - Batches de 50 documentos por vez
  - Progressbar visual mostrando: `[████░░] 45% (23/50 batches, 1150/2500 docs)`
- Gera embeddings para cada chunk
- Armazena no Qdrant com metadata

### 5. Agent com RAG
```go
ag, err := agent.NewAgent(agent.AgentConfig{
    Knowledge:             kb,
    AddKnowledgeToContext: true,
    KnowledgeMaxDocuments: 5,
})
```
- Agent busca documentos relevantes automaticamente
- Adiciona ao contexto da query
- LLM cloud gera resposta baseada no conhecimento

## Otimizações para PDFs Grandes

### Processamento Paralelo
O sistema processa chunks do PDF em paralelo para velocidade máxima.

### Batch Embeddings
Embeddings são gerados em batches para eficiência.

### Indexação Inteligente
Qdrant indexa os vetores automaticamente para buscas rápidas.

## Execução

```bash
# Certifique-se que Docker está rodando
docker ps

# Certifique-se que Ollama local está rodando
ollama list | grep gemma:2b

# Execute o exemplo
go run main.go
```

## Output Esperado

```
╔═══════════════════════════════════════════════════════════╗
║       Knowledge Base with PDF - RAG Example              ║
╚═══════════════════════════════════════════════════════════╝

🐳 Starting Qdrant container...
✅ Qdrant running at: http://localhost:6333

🔤 Initializing local Ollama embedder (gemma:2b)...
📊 Setting up Qdrant vector database...
📚 Creating knowledge base...
📄 Loading PDF: /path/to/pdf
⏳ This may take a few minutes for large PDFs...
✅ PDF loaded successfully in 2m30s

🤖 Creating agent with Ollama Cloud model (kimi-k2:1t-cloud)...
✅ Agent ready!

═══════════════════════════════════════════════════════════

📝 Query 1: What is Mistral AI and what are its main features?
───────────────────────────────────────────────────────────
🤖 Answer (took 3.2s):
[Resposta detalhada baseada no PDF]
```

## Perguntas Demonstradas

1. **Introdução**: O que é Mistral AI?
2. **RAG**: Como funciona RAG com Mistral?
3. **Embeddings**: Melhores práticas
4. **Deployment**: Deploy no AWS Bedrock
5. **Agents**: Papel dos agents no sistema

## Vantagens desta Arquitetura

✅ **Custo Reduzido**: Embeddings locais = sem custo de API  
✅ **Performance**: Qdrant é extremamente rápido  
✅ **Escalabilidade**: Container pode ser escalado facilmente  
✅ **Qualidade**: Cloud LLM (kimi-k2:1t) para respostas de alta qualidade  
✅ **Flexibilidade**: Troque componentes facilmente  

## Troubleshooting

### Erro: "Docker não está rodando"
```bash
# Linux
sudo systemctl start docker

# macOS
open -a Docker
```

### Erro: "Modelo gemma:2b não encontrado"
```bash
ollama pull gemma:2b
```

### PDF muito grande
Ajuste o chunk size no código se necessário:
```go
// Em knowledge.LoadPDF
chunkSize := 1000  // caracteres por chunk
```

## Próximos Passos

- Experimentar com outros modelos de embedding
- Adicionar filtros de metadata para buscas específicas
- Implementar cache de embeddings
- Testar com outros formatos (Markdown, HTML, etc.)
