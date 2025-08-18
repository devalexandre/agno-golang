# 📚 Agno-Golang PDF Knowledge Base

Esta implementação adiciona suporte completo para bases de conhecimento PDF ao Agno-Golang, seguindo o padrão de compatibilidade nativa do Agno Python.

## 🚀 Recursos Implementados

### ✅ Funcionalidades Principais
- **Compatibilidade Nativa**: Interface `knowledge.VectorDB = vectordb.VectorDB` elimina necessidade de adapters
- **PDFs Locais**: Suporte para arquivos PDF do sistema de arquivos
- **PDFs de URLs**: Download e processamento automático de PDFs via HTTP/HTTPS
- **Processamento Paralelo**: Workers paralelos com goroutines para inserção vetorial otimizada
- **Barras de Progresso**: Feedback visual em tempo real com Unicode progress bars (█░▓▒)
- **Chunking Inteligente**: Divisão de texto com sobreposição configurável (500 chars padrão)
- **Metadados Ricos**: Preservação de informações de origem e contexto
- **Rate Limiting**: Controle de taxa com delays configuráveis para APIs
- **Retry Logic**: Lógica de retry com backoff exponencial para robustez
- **Integração Qdrant**: Compatibilidade direta com Qdrant como backend vetorial

### 🔧 Componentes Implementados

#### `agno/knowledge/base.go`
- Interface `Knowledge` para bases de conhecimento
- Tipo alias `VectorDB = vectordb.VectorDB` para compatibilidade nativa
- Implementação base `BaseKnowledge` com funcionalidades comuns
- Utilitários para validação e sanitização

#### `agno/knowledge/pdf.go`
- `PDFKnowledgeBase`: Implementação específica para PDFs
- Suporte para arquivos locais e URLs
- Integração com `pdftotext` para extração de texto
- Chunking configurável (tamanho padrão: 500 caracteres, sobreposição: 50)
- **Processamento Paralelo**: Método `LoadParallel()` com workers configuráveis
- **Progress Tracking**: Barras de progresso visuais com Unicode chars (📈 📊 🚀)
- **Rate Limiting**: Delays de 100ms entre requisições para estabilidade
- **Retry Logic**: Até 3 tentativas com backoff exponencial
- Processamento de metadados e geração de IDs únicos
- Sanitização UTF-8 para textos extraídos

#### `examples/pdf_qdrant_agent/main.go`
- Exemplo completo seguindo o padrão do Agno Python
- Uso direto do Qdrant sem adapters
- Demonstração de carregamento de PDF via URL
- Integração com agente para respostas baseadas no conteúdo

## 🐍 Compatibilidade com Agno Python

A implementação segue exatamente o padrão do Agno Python:

```python
# Agno Python
vector_db = Qdrant(...)
knowledge_base = PDFUrlKnowledgeBase(..., vector_db=vector_db)
```

```go
// Agno Golang (implementação atual)
vectorDB := qdrant.NewQdrant(config)
knowledgeBase := knowledge.NewPDFKnowledgeBase("name", vectorDB)
```

### 🔄 Principais Melhorias

1. **Eliminação de Adapters**: Uso direto da interface `vectordb.VectorDB`
2. **Compatibilidade Nativa**: `knowledge.VectorDB = vectordb.VectorDB`
3. **Interface Unificada**: Mesma assinatura de métodos que o Python
4. **Integração Direta**: Qdrant implementa `vectordb.VectorDB` nativamente

## 📖 Como Usar

### 1. Configurar Qdrant
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

### 2. Criar Base de Conhecimento PDF
```go
knowledgeBase := knowledge.NewPDFKnowledgeBase("pdf-knowledge", vectorDB)

// Configurar PDFs
knowledgeBase.URLs = []string{
    "https://arxiv.org/pdf/2305.13245.pdf",
}

// Carregar documentos com processamento paralelo
err := knowledgeBase.LoadParallel(ctx, true, 3) // 3 workers

// Ou carregar sequencialmente com progresso
err := knowledgeBase.Load(ctx, true)
```

### 3. Configurações Avançadas
```go
// Ajustar chunking
knowledgeBase.ChunkSize = 800
knowledgeBase.ChunkOverlap = 100

// Configurações complexas com metadados
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

### 3. Integrar com Agente
```go
agentConfig := agent.AgentConfig{
    Model:       model,
    Name:        "PDF Agent",
    Role:        "PDF Document Analysis Specialist",
    Instructions: "You have access to a PDF knowledge base. Use search_knowledge_base to search for information.",
    // ... other configurations
}
agentObj := agent.NewAgent(agentConfig)

// Buscar na base de conhecimento
results, err := knowledgeBase.Search(ctx, "conceitos importantes", 5)
```

### 4. Feedback Visual de Progresso
Durante o carregamento, você verá barras de progresso em tempo real:

```
📚 Carregamento paralelo de 1 fonte(s) com 3 workers...
📈 [██████████████████████████████] 100.0% (1/1) Baixando: https://example.pdf ✅ 461 documentos

📊 Total de documentos carregados: 461
🚀 Iniciando inserção paralela no banco vetorial com 3 workers...
⚡ Processamento paralelo com 3 workers de 461 documentos...
🔄 [████████████████████████████▓▓] 93.5% (431/461) processados
✅ Processamento paralelo completo!
```

## �️ Métodos Disponíveis

### PDFKnowledgeBase

#### Carregamento
```go
// Carregamento sequencial com progresso
Load(ctx context.Context, recreate bool) error

// Carregamento paralelo (recomendado para PDFs grandes)
LoadParallel(ctx context.Context, recreate bool, numWorkers int) error

// Carregamento de documento único
LoadDocument(ctx context.Context, doc document.Document) error

// Carregamento por path/URL
LoadDocumentFromPath(ctx context.Context, pathOrURL string, metadata map[string]interface{}) error
```

#### Busca
```go
// Busca semântica
Search(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*SearchResult, error)

// Busca de documentos
SearchDocuments(ctx context.Context, query string, numDocuments int, filters map[string]interface{}) ([]document.Document, error)
```

#### Configuração
```go
// Configurar chunking
kb.ChunkSize = 500
kb.ChunkOverlap = 50

// Configurar fontes
kb.Paths = []string{"/path/to/pdf"}
kb.URLs = []string{"https://example.com/doc.pdf"}
kb.Configs = []PDFConfig{{URL: "...", Metadata: map[string]interface{}{"tag": "value"}}}
```

### Exemplos de Uso Avançado

#### 1. Processamento de Múltiplos PDFs
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

// Processar com 3 workers
err := knowledgeBase.LoadParallel(ctx, true, 3)
```

#### 2. Busca com Filtros
```go
// Buscar apenas documentos de pesquisa
filters := map[string]interface{}{
    "category": "AI Research",
}

results, err := knowledgeBase.Search(ctx, "neural networks", 5, filters)
for _, result := range results {
    fmt.Printf("Score: %.2f - %s\n", result.Score, result.Document.Content[:100])
}
```

#### 3. Configuração de Performance
```go
// Para PDFs pequenos
knowledgeBase.ChunkSize = 300
knowledgeBase.ChunkOverlap = 30

// Para PDFs grandes com processamento rápido
knowledgeBase.ChunkSize = 800
knowledgeBase.ChunkOverlap = 80

// Usar mais workers para inserção mais rápida (cuidado com rate limits)
err := knowledgeBase.LoadParallel(ctx, true, 5)
```

## �🔧 Dependências

### Sistemas
- `pdftotext` (parte do pacote `poppler-utils`)
- Qdrant rodando em `localhost:6334` (gRPC)
- OpenAI API Key para embeddings

### Instalação pdftotext
```bash
# Ubuntu/Debian
sudo apt-get install poppler-utils

# macOS
brew install poppler

# Windows
# Download from https://poppler.freedesktop.org/
```

### Go Packages
- Todos os packages existentes do Agno-Golang
- Sem dependências adicionais externas

## 🎯 Arquitetura

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

## ✅ Status de Implementação

- [x] Interface nativa compatível com vectordb
- [x] Processamento de PDFs locais
- [x] Processamento de PDFs via URL
- [x] Chunking com sobreposição
- [x] Integração com Qdrant
- [x] Exemplo funcional completo
- [x] Eliminação de adapters
- [x] Compatibilidade com padrão Python
- [x] **Processamento paralelo com workers**
- [x] **Barras de progresso visuais**
- [x] **Rate limiting e retry logic**
- [x] **Sanitização UTF-8**
- [x] **Otimização de performance**

## 🧪 Teste

Para testar a implementação:

```bash
cd examples/pdf_qdrant_agent
export OPENAI_API_KEY="sua-chave-aqui"
go run main.go
```

### Teste de Performance Paralela
```bash
cd examples/test_parallel
export OPENAI_API_KEY="sua-chave-aqui"
go run main.go
```

Resultado esperado:
- Download e processamento de PDF em segundos
- Barras de progresso visuais
- Processamento paralelo com múltiplos workers
- Performance: ~461 documentos em ~25 segundos

Certifique-se de que:
1. Qdrant esteja rodando em `localhost:6334` (gRPC)
2. `pdftotext` esteja instalado
3. OpenAI API Key configurada
4. Conexão com internet para baixar PDFs de exemplo

## 📝 Próximos Passos

1. ✅ **Implementação Base Completa**
2. ✅ **Compatibilidade Nativa com VectorDB**  
3. ✅ **Integração com Agente**
4. ✅ **Processamento Paralelo**
5. ✅ **Barras de Progresso**
6. ✅ **Rate Limiting e Retry Logic**
7. 🔄 **Testes Unitários**
8. 🔄 **Documentação Expandida**
9. 🔄 **Métricas de Performance**
10. 🔄 **Suporte a Outros Formatos (DOCX, TXT)**

## 🚀 Performance

### Benchmarks Realizados
- **PDF Grande (461 chunks)**: ~25 segundos com 2 workers
- **Taxa de Processamento**: ~18 documentos/segundo
- **Memory Usage**: Otimizado com streaming
- **API Calls**: Rate limited (100ms delays)

### Configurações Recomendadas
- **Workers**: 2-3 para APIs externas (OpenAI)
- **Chunk Size**: 500-800 caracteres
- **Chunk Overlap**: 50-100 caracteres
- **Timeout**: 60 segundos para embeddings

## 🔧 Troubleshooting

### Problemas Comuns

#### 1. "pdftotext not found"
```bash
# Ubuntu/Debian
sudo apt-get install poppler-utils

# macOS  
brew install poppler

# Verificar instalação
which pdftotext
```

#### 2. "Connection refused" (Qdrant)
```bash
# Verificar se Qdrant está rodando
docker ps | grep qdrant

# Iniciar Qdrant se necessário
docker run -p 6333:6333 -p 6334:6334 qdrant/qdrant
```

#### 3. "OpenAI API rate limit"
- Reduzir número de workers: `LoadParallel(ctx, true, 1)`
- Aumentar delays no código (modify rate limiting)
- Verificar quota da API OpenAI

#### 4. "Out of memory" (PDFs grandes)
- Reduzir chunk size: `knowledgeBase.ChunkSize = 300`
- Processar em batches menores
- Usar `Load()` ao invés de `LoadParallel()`

### Dicas de Performance

1. **Workers Ideais**: 2-3 para APIs externas, 5-10 para processamento local
2. **Chunk Size**: 500-800 chars para textos técnicos, 300-500 para textos gerais
3. **Overlap**: 10-20% do chunk size
4. **Timeout**: 60s+ para embeddings de textos longos

### Logs de Debug
```go
// Ativar logs detalhados
log.SetLevel(log.DebugLevel)

// Verificar progresso detalhado
fmt.Printf("Processando documento %d/%d\n", current, total)
```

---

Esta implementação garante que o Agno-Golang tenha paridade completa com o Agno Python para processamento de PDFs, mantendo a mesma simplicidade e compatibilidade de interface, agora com performance otimizada e feedback visual aprimorado.
