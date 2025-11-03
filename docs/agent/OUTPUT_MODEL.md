# OutputModel - Two-Stage Processing

## Overview

`OutputModel` permite usar **dois modelos diferentes** para processar uma requisição:

1. **Modelo Principal** (pode ser caro/potente): Gera conteúdo criativo com prompt simples
2. **OutputModel** (pode ser barato/rápido): Formata o conteúdo em JSON estruturado

## Vantagens

### 💰 Economia de Custos
- Use modelo caro apenas para geração de conteúdo (prompt menor)
- Use modelo barato para formatação mecânica de JSON
- Reduza tokens enviados ao modelo principal (sem instruções de schema)

### 📊 Duas Saídas
- `response.TextContent`: Resposta original criativa do modelo principal
- `response.Output` / `pointer`: JSON estruturado formatado pelo OutputModel

### 🎯 Separação de Responsabilidades
- Modelo principal: Foco em criatividade e qualidade de conteúdo
- OutputModel: Foco em formatação e estruturação precisa

## Como Funciona

### Fluxo de Execução

```
┌─────────────────────────────────────────────────────────────┐
│ 1. User Input                                               │
│    "Create a sci-fi movie about AI"                         │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. Main Model (expensive)                                   │
│    Receives: Simple prompt only                             │
│    Returns: Creative text content                           │
│    Example: "In the year 2157, an AI named Atlas..."       │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. OutputModel (cheap)                                      │
│    Receives: Main model's response + JSON schema            │
│    Returns: Structured JSON matching schema                 │
│    Example: {"name": "Atlas", "genre": "sci-fi", ...}      │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│ 4. Two Outputs Available                                    │
│    - response.TextContent: Original creative text           │
│    - response.Output: Structured data (filled pointer)      │
└─────────────────────────────────────────────────────────────┘
```

## Uso Básico

### Exemplo Completo

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"

    "github.com/devalexandre/agno-golang/agno/agent"
    "github.com/devalexandre/agno-golang/agno/models"
    "github.com/devalexandre/agno-golang/agno/models/ollama"
)

type MovieScript struct {
    Name       string   `json:"name"`
    Genre      string   `json:"genre"`
    Setting    string   `json:"setting"`
    Characters []string `json:"characters"`
    Storyline  string   `json:"storyline"`
}

func main() {
    ctx := context.Background()

    // Modelo principal - pode ser modelo mais caro/potente
    mainModel, _ := ollama.NewOllamaChat(
        models.WithID("llama3.2:latest"),
        models.WithBaseURL("http://localhost:11434"),
    )

    // Modelo de output - pode ser modelo mais barato/rápido
    outputModel, _ := ollama.NewOllamaChat(
        models.WithID("llama3.2:latest"),
        models.WithBaseURL("http://localhost:11434"),
    )

    movieScript := &MovieScript{}

    // Configurar agente com OutputModel
    agent, _ := agent.NewAgent(agent.AgentConfig{
        Context:       ctx,
        Model:         mainModel,
        OutputModel:   outputModel,    // Modelo separado para formatação
        OutputSchema:  movieScript,    // Schema para estruturar dados
        Description:   "You are a creative movie script writer.",
        ParseResponse: true,
    })

    // Executar com prompt simples
    response, _ := agent.Run("Create a sci-fi movie about AI")

    // OUTPUT 1: Texto original do modelo principal
    fmt.Println("Creative Content:")
    fmt.Println(response.TextContent)

    // OUTPUT 2: JSON estruturado via OutputModel
    fmt.Println("\nStructured Data:")
    movieJSON, _ := json.MarshalIndent(movieScript, "", "  ")
    fmt.Println(string(movieJSON))

    // Também acessível via response.Output
    if script, ok := response.Output.(*MovieScript); ok {
        fmt.Printf("\nMovie: %s (%s)\n", script.Name, script.Genre)
    }
}
```

## Prompt Customizado

Você pode customizar o prompt usado pelo OutputModel:

```go
customPrompt := `You are a JSON formatter. Convert the text into strict JSON.
Be extremely concise. Use short, punchy descriptions.

Return ONLY valid JSON. No explanations, no markdown.`

agent, _ := agent.NewAgent(agent.AgentConfig{
    Context:           ctx,
    Model:             mainModel,
    OutputModel:       outputModel,
    OutputModelPrompt: customPrompt,  // Instrução customizada
    OutputSchema:      movieScript,
    ParseResponse:     true,
})
```

## Comparação: Com vs Sem OutputModel

### Sem OutputModel (tradicional)

```go
agent, _ := agent.NewAgent(agent.AgentConfig{
    Model:         mainModel,
    OutputSchema:  movieScript,
    ParseResponse: true,
})
```

**Fluxo:**
1. Modelo principal recebe: prompt + instruções de schema
2. Modelo principal retorna: JSON estruturado
3. Agent faz parse do JSON

**Problema:**
- Prompt maior (inclui schema) = mais tokens = mais caro
- Modelo caro usado para tarefa mecânica (formatação JSON)

### Com OutputModel (otimizado)

```go
agent, _ := agent.NewAgent(agent.AgentConfig{
    Model:         mainModel,
    OutputModel:   outputModel,
    OutputSchema:  movieScript,
    ParseResponse: true,
})
```

**Fluxo:**
1. Modelo principal recebe: prompt simples (sem schema)
2. Modelo principal retorna: texto criativo
3. OutputModel recebe: texto + schema
4. OutputModel retorna: JSON estruturado

**Vantagem:**
- ✅ Prompt menor para modelo caro
- ✅ Modelo barato para formatação
- ✅ Duas saídas disponíveis
- ✅ Melhor qualidade de conteúdo

## Casos de Uso

### 1. Redução de Custos
```go
// GPT-4 para conteúdo, GPT-3.5 para formatação
mainModel := openai.NewOpenAI("gpt-4")
outputModel := openai.NewOpenAI("gpt-3.5-turbo")
```

### 2. Otimização de Latência
```go
// Modelo grande para qualidade, modelo pequeno para velocidade
mainModel := ollama.NewOllama("llama3.2:70b")
outputModel := ollama.NewOllama("llama3.2:3b")
```

### 3. Especialização
```go
// Modelo criativo para conteúdo, modelo estruturado para JSON
mainModel := anthropic.NewClaude("claude-3-opus")
outputModel := openai.NewOpenAI("gpt-4-structured")
```

## Implementação Interna

O método `ApplyOutputFormatting` segue o mesmo padrão de `ApplySemanticCompression`:

```go
// ApplyOutputFormatting applies output formatting using OutputModel if configured
func (a *Agent) ApplyOutputFormatting(response string) (interface{}, error) {
    if a.outputSchema == nil || !a.parseResponse {
        return response, nil
    }

    // If OutputModel is configured, use it for JSON formatting
    if a.outputModel != nil {
        return a.formatWithOutputModel(response)
    }

    // Otherwise, parse directly from the response
    return a.parseOutputWithSchema(response)
}
```

## Campos Relacionados

### AgentConfig

```go
type AgentConfig struct {
    // ... outros campos ...
    
    // OutputSchema define a estrutura esperada da saída
    OutputSchema interface{}
    
    // OutputModel é o modelo usado para formatação JSON
    // Se nil, o modelo principal faz a formatação
    OutputModel models.AgnoModelInterface
    
    // OutputModelPrompt customiza o prompt do OutputModel
    // Se vazio, usa prompt padrão
    OutputModelPrompt string
    
    // ParseResponse ativa/desativa parsing automático
    ParseResponse bool
}
```

### RunResponse

```go
type RunResponse struct {
    // TextContent: resposta original do modelo principal
    TextContent string
    
    // Output: dados estruturados (pointer preenchido)
    Output interface{}
    
    // ParsedOutput: deprecated, use Output
    ParsedOutput interface{}
    
    // ... outros campos ...
}
```

## Exemplos Práticos

### Exemplo 1: Análise de Dados

```go
type DataAnalysis struct {
    Summary    string   `json:"summary"`
    KeyPoints  []string `json:"key_points"`
    Metrics    map[string]float64 `json:"metrics"`
}

analysis := &DataAnalysis{}

agent, _ := agent.NewAgent(agent.AgentConfig{
    Model:         expensiveModel,  // Análise profunda
    OutputModel:   cheapModel,      // Formatação simples
    OutputSchema:  analysis,
})

response, _ := agent.Run("Analyze this dataset: ...")

// Texto analítico detalhado
fmt.Println(response.TextContent)

// Métricas estruturadas
fmt.Printf("Metrics: %v\n", analysis.Metrics)
```

### Exemplo 2: Geração de Conteúdo

```go
type BlogPost struct {
    Title    string   `json:"title"`
    Tags     []string `json:"tags"`
    Content  string   `json:"content"`
    WordCount int     `json:"word_count"`
}

post := &BlogPost{}

agent, _ := agent.NewAgent(agent.AgentConfig{
    Model:       creativeModel,  // Escrita criativa
    OutputModel: structuredModel, // Extração de metadados
    OutputSchema: post,
})

response, _ := agent.Run("Write a blog post about AI")

// Conteúdo completo e criativo
saveToFile(response.TextContent)

// Metadados estruturados para database
saveMetadata(post.Title, post.Tags, post.WordCount)
```

## Melhores Práticas

### 1. Escolha de Modelos

```go
// ✅ BOM: Modelo grande para criatividade, pequeno para estrutura
mainModel := "llama3.2:70b"
outputModel := "llama3.2:3b"

// ❌ EVITE: Mesmo modelo em ambos (não há benefício)
mainModel := "llama3.2:latest"
outputModel := "llama3.2:latest"
```

### 2. Design de Schemas

```go
// ✅ BOM: Schema detalhado com descriptions
type Movie struct {
    Name  string `json:"name" description:"Movie title"`
    Genre string `json:"genre" description:"Genre (action, drama, etc)"`
}

// ❌ EVITE: Schema sem contexto
type Movie struct {
    Name  string `json:"name"`
    Genre string `json:"genre"`
}
```

### 3. Prompts Customizados

```go
// ✅ BOM: Prompt específico para o caso de uso
customPrompt := `Extract structured data from the text.
Focus on accuracy over creativity.
Return valid JSON only.`

// ❌ EVITE: Prompt genérico (use default)
customPrompt := "Convert to JSON"
```

## Troubleshooting

### OutputModel não está sendo usado

**Sintoma:** Saída formatada incorretamente

**Solução:**
```go
// Certifique-se de configurar todos os campos necessários
agent, _ := agent.NewAgent(agent.AgentConfig{
    OutputModel:   outputModel,    // ✅ Definir modelo
    OutputSchema:  schema,         // ✅ Definir schema
    ParseResponse: true,           // ✅ Ativar parsing
})
```

### Pointer não está sendo preenchido

**Sintoma:** `movieScript` está vazio após `Run()`

**Solução:**
```go
// ✅ CORRETO: Passar pointer
movieScript := &MovieScript{}
OutputSchema: movieScript

// ❌ ERRADO: Passar valor
OutputSchema: MovieScript{}
```

### Duas chamadas de modelo lentas

**Sintoma:** Execução muito lenta

**Solução:**
```go
// Use modelo mais rápido para OutputModel
outputModel := ollama.NewOllama("llama3.2:3b")  // ✅ Modelo pequeno/rápido
```

## Referências

- [INPUT_OUTPUT_SCHEMA.md](./INPUT_OUTPUT_SCHEMA.md) - Documentação completa de schemas
- [RELEASE_INPUT_OUTPUT_SCHEMA.md](../../RELEASE_INPUT_OUTPUT_SCHEMA.md) - Release notes
- Exemplo: [examples/input-output/output-model/](../../examples/input-output/output-model/)
