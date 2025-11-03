# Input and Output Schema

Esta documentação explica como usar Input e Output Schemas no Agno-Golang para obter respostas estruturadas e validar entradas.

## Visão Geral

O Agno-Golang suporta schemas de entrada e saída para:
- **Input Schema**: Validar e estruturar dados de entrada
- **Output Schema**: Forçar o LLM a retornar JSON estruturado que corresponde a um schema específico
- **Type Safety**: Trabalhar com structs Go tipadas em vez de strings ou `interface{}`

## Output Schema

### Output Schema - Objeto Único

Use quando você quer que o agent retorne **um único objeto** estruturado.

#### Exemplo Completo

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

// Define seu struct de saída
type MovieScript struct {
    Setting    string   `json:"setting" description:"Provide a nice setting for a blockbuster movie."`
    Ending     string   `json:"ending" description:"Ending of the movie. If not available, provide a happy ending."`
    Genre      string   `json:"genre" description:"Genre of the movie. If not available, select action, thriller or romantic comedy."`
    Name       string   `json:"name" description:"Give a name to this movie"`
    Characters []string `json:"characters" description:"Name of characters for this movie."`
    Storyline  string   `json:"storyline" description:"3 sentence storyline for the movie. Make it exciting!"`
}

func main() {
    // 1. Crie o modelo
    model, err := ollama.NewOllamaChat(
        models.WithID("llama3.2:latest"),
        models.WithBaseURL("http://localhost:11434"),
    )
    if err != nil {
        log.Fatalf("Failed to create model: %v", err)
    }

    // 2. Crie um ponteiro para o struct que será preenchido
    movieScript := &MovieScript{}

    // 3. Crie o agent com OutputSchema apontando para o struct
    structuredOutputAgent, err := agent.NewAgent(agent.AgentConfig{
        Context:       context.Background(),
        Model:         model,
        Description:   "You write movie scripts.",
        OutputSchema:  movieScript,  // Passa o ponteiro aqui
        ParseResponse: true,
    })
    if err != nil {
        log.Fatalf("Failed to create agent: %v", err)
    }

    // 4. Execute o agent
    run, err := structuredOutputAgent.Run("Create a movie script set in New York")
    if err != nil {
        log.Fatalf("Agent run failed: %v", err)
    }

    // 5. Acesse o resultado de duas formas:
    
    // Forma 1: Use o ponteiro original (já está preenchido!)
    fmt.Printf("Movie: %s\n", movieScript.Name)
    fmt.Printf("Genre: %s\n", movieScript.Genre)
    
    // Forma 2: Use run.Output (aponta para o mesmo dado)
    outputScript := run.Output.(*MovieScript)
    fmt.Printf("Same pointer? %v\n", movieScript == outputScript)  // true
}
```

**Saída exemplo:**
```
Movie: Big Apple Dreams
Genre: romantic comedy
Same pointer? true
```

### Output Schema - Array/Slice

Use quando você quer que o agent retorne **múltiplos objetos** em um array.

#### Exemplo Completo

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
    Setting    string   `json:"setting" description:"Provide a nice setting for a blockbuster movie."`
    Ending     string   `json:"ending" description:"Ending of the movie. If not available, provide a happy ending."`
    Genre      string   `json:"genre" description:"Genre of the movie. If not available, select action, thriller or romantic comedy."`
    Name       string   `json:"name" description:"Give a name to this movie"`
    Characters []string `json:"characters" description:"Name of characters for this movie."`
    Storyline  string   `json:"storyline" description:"3 sentence storyline for the movie. Make it exciting!"`
}

func main() {
    model, err := ollama.NewOllamaChat(
        models.WithID("llama3.2:latest"),
        models.WithBaseURL("http://localhost:11434"),
    )
    if err != nil {
        log.Fatalf("Failed to create model: %v", err)
    }

    // Crie um ponteiro para SLICE de structs
    movieScripts := &[]MovieScript{}

    structuredOutputAgent, err := agent.NewAgent(agent.AgentConfig{
        Context:       context.Background(),
        Model:         model,
        Description:   "You write movie scripts.",
        OutputSchema:  movieScripts,  // Ponteiro para slice
        ParseResponse: true,
    })
    if err != nil {
        log.Fatalf("Failed to create agent: %v", err)
    }

    // Peça múltiplos resultados
    _, err = structuredOutputAgent.Run("Create 3 movie scripts: one set in New York, one in Tokyo, and one in Paris")
    if err != nil {
        log.Fatalf("Agent run failed: %v", err)
    }

    // Acesse o slice preenchido
    fmt.Printf("Total movies: %d\n", len(*movieScripts))
    for i, movie := range *movieScripts {
        fmt.Printf("%d. %s (%s)\n", i+1, movie.Name, movie.Genre)
    }
}
```

**Saída exemplo:**
```
Total movies: 3
1. New York Undercover (Action, Thriller)
2. Tokyo Rising (Science Fiction, Action)
3. Paris Love Story (Romantic Comedy, Drama)
```

## Input Schema

Use Input Schema para **validar e estruturar** os dados de entrada enviados ao agent.

### Exemplo Completo

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

// Define o schema de entrada
type ResearchTopic struct {
    Topic           string   `json:"topic" description:"The main research topic"`
    FocusAreas      []string `json:"focus_areas" description:"Specific areas to focus on"`
    TargetAudience  string   `json:"target_audience" description:"Who this research is for"`
    SourcesRequired int      `json:"sources_required" description:"Number of sources needed"`
}

func main() {
    model, err := ollama.NewOllamaChat(
        models.WithID("llama3.2:latest"),
        models.WithBaseURL("http://localhost:11434"),
    )
    if err != nil {
        log.Fatalf("Failed to create model: %v", err)
    }

    // Crie o agent com InputSchema
    researchAgent, err := agent.NewAgent(agent.AgentConfig{
        Context:     context.Background(),
        Model:       model,
        Name:        "Research Agent",
        Role:        "Extract key insights and content from research topics",
        InputSchema: ResearchTopic{},  // Define o schema de entrada
    })
    if err != nil {
        log.Fatalf("Failed to create agent: %v", err)
    }

    // Crie a entrada como struct
    topic := ResearchTopic{
        Topic:           "Artificial Intelligence",
        FocusAreas:      []string{"Machine Learning", "Deep Learning", "Neural Networks"},
        TargetAudience:  "Software Developers",
        SourcesRequired: 5,
    }

    // Passe o struct diretamente - não precisa fazer marshal!
    run, err := researchAgent.Run(topic)
    if err != nil {
        log.Fatalf("Agent run failed: %v", err)
    }

    fmt.Println(run.TextContent)
}
```

## Input + Output Schema Juntos

Combine ambos para **entrada estruturada** e **saída estruturada**.

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

// Schema de entrada
type ResearchTopic struct {
    Topic           string `json:"topic" description:"The main research topic"`
    SourcesRequired int    `json:"sources_required" description:"Number of sources needed"`
}

// Schema de saída
type ResearchOutput struct {
    Summary      string   `json:"summary" description:"Executive summary of the research"`
    Insights     []string `json:"insights" description:"Key insights from the topic"`
    TopStories   []string `json:"top_stories" description:"Most relevant and popular stories"`
    Technologies []string `json:"technologies" description:"Technologies mentioned"`
    Sources      []string `json:"sources" description:"Links or references to relevant sources"`
}

func main() {
    model, err := ollama.NewOllamaChat(
        models.WithID("llama3.2:latest"),
        models.WithBaseURL("http://localhost:11434"),
    )
    if err != nil {
        log.Fatalf("Failed to create model: %v", err)
    }

    // Crie ponteiro para output
    researchOutput := &ResearchOutput{}

    // Crie agent com AMBOS schemas
    researchAgent, err := agent.NewAgent(agent.AgentConfig{
        Context:       context.Background(),
        Model:         model,
        Name:          "Research Agent",
        Role:          "Technical Research Specialist",
        Instructions:  "Research topics and provide comprehensive insights with sources",
        InputSchema:   ResearchTopic{},      // Schema de entrada
        OutputSchema:  researchOutput,       // Schema de saída
        ParseResponse: true,
    })
    if err != nil {
        log.Fatalf("Failed to create agent: %v", err)
    }

    // Entrada estruturada
    topic := ResearchTopic{
        Topic:           "Artificial Intelligence and Machine Learning",
        SourcesRequired: 5,
    }

    // Execute
    _, err = researchAgent.Run(topic)
    if err != nil {
        log.Fatalf("Agent run failed: %v", err)
    }

    // Saída estruturada já preenchida
    fmt.Println("=== Summary ===")
    fmt.Println(researchOutput.Summary)
    fmt.Println()
    
    fmt.Println("=== Key Insights ===")
    for i, insight := range researchOutput.Insights {
        fmt.Printf("%d. %s\n", i+1, insight)
    }
    
    fmt.Println()
    fmt.Println("=== Technologies ===")
    for i, tech := range researchOutput.Technologies {
        fmt.Printf("%d. %s\n", i+1, tech)
    }
}
```

## Como Funciona Internamente

### Output Schema

1. **Geração do Schema JSON**: O Agno converte seu struct Go em JSON Schema
2. **Injeção no Prompt**: O schema é adicionado ao system prompt com instruções para o LLM
3. **Resposta do LLM**: O modelo retorna JSON que corresponde ao schema
4. **Parsing Automático**: O JSON é parseado e o ponteiro original é preenchido
5. **Acesso aos Dados**: Você pode usar o ponteiro original ou `run.Output`

### Input Schema

1. **Validação**: O struct de entrada é validado contra o schema
2. **Serialização**: O struct é convertido para JSON
3. **Envio**: O JSON é enviado como prompt para o LLM

## Convenções e Boas Práticas

### 1. Sempre Use Ponteiros para OutputSchema

```go
// ✅ CORRETO - Use ponteiro
movieScript := &MovieScript{}
agent.NewAgent(agent.AgentConfig{
    OutputSchema: movieScript,
})

// ❌ ERRADO - Não use valor
movieScript := MovieScript{}
agent.NewAgent(agent.AgentConfig{
    OutputSchema: movieScript,  // O original não será preenchido!
})
```

### 2. Use Tags JSON com Descriptions

As tags `description` são usadas para gerar o JSON Schema e ajudam o LLM a entender o que preencher:

```go
type Movie struct {
    Name  string `json:"name" description:"Give a creative name to this movie"`
    Genre string `json:"genre" description:"Genre of the movie (action, comedy, drama, etc)"`
}
```

### 3. Use `omitempty` para Campos Opcionais

```go
type Person struct {
    Name  string `json:"name" description:"Person's name"`              // Obrigatório
    Email string `json:"email,omitempty" description:"Optional email"`  // Opcional
}
```

### 4. ParseResponse: true

Sempre defina `ParseResponse: true` quando usar `OutputSchema`:

```go
agent.NewAgent(agent.AgentConfig{
    OutputSchema:  &myStruct,
    ParseResponse: true,  // Necessário!
})
```

## Tratamento de Erros

### Erro de Parsing

Se o LLM não retornar JSON válido, você receberá um erro:

```go
run, err := agent.Run("prompt")
if err != nil {
    // Pode ser erro de parsing se o JSON estiver malformado
    log.Printf("Error: %v", err)
}
```

### Validação de Input

Se o input não corresponder ao InputSchema, receberá erro de validação:

```go
topic := ResearchTopic{
    Topic: "AI",
    // Faltando campos obrigatórios
}

_, err := agent.Run(topic)
if err != nil {
    log.Printf("Input validation failed: %v", err)
}
```

## Exemplos Completos

Todos os exemplos funcionais estão disponíveis em:

```
examples/input-output/
├── output/         # Output schema com objeto único
├── output-slice/   # Output schema com array
├── input/          # Input schema
└── both/           # Input + Output juntos
```

Para executar qualquer exemplo:

```bash
cd examples/input-output/output
go run main.go
```

## Comparação com Python

### Python (agno-python)

```python
from agno.agent import Agent
from pydantic import BaseModel

class MovieScript(BaseModel):
    name: str
    genre: str

agent = Agent(
    model=model,
    output_schema=MovieScript
)

run = agent.run("Create a movie")
print(run.content.name)  # Acesso direto
```

### Go (agno-golang)

```go
type MovieScript struct {
    Name  string `json:"name"`
    Genre string `json:"genre"`
}

movieScript := &MovieScript{}
agent, _ := agent.NewAgent(agent.AgentConfig{
    Model:         model,
    OutputSchema:  movieScript,
    ParseResponse: true,
})

agent.Run("Create a movie")
fmt.Println(movieScript.Name)  // Acesso direto - sem type assertion!
```

A principal diferença é que em Go você passa um ponteiro que será preenchido automaticamente.

## Troubleshooting

### "unexpected end of JSON input"

O LLM pode estar gerando JSON incompleto. Soluções:
1. Use um modelo maior/melhor
2. Simplifique o schema
3. Adicione mais contexto no prompt

### "run.Output is nil"

Certifique-se de:
1. `ParseResponse: true` está configurado
2. O OutputSchema não é nil
3. O modelo retornou uma resposta válida

### Type Assertion Panic

Use sempre type assertion com verificação:

```go
if run.Output != nil {
    if movieScript, ok := run.Output.(*MovieScript); ok {
        fmt.Println(movieScript.Name)
    }
}
```

Ou melhor ainda, use o ponteiro original que você passou no OutputSchema - ele já está preenchido!
