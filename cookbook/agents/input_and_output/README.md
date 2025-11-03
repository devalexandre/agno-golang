# Input and Output Examples

Esta pasta contém exemplos de como usar Input e Output Schemas com agents Agno.

## Exemplos

### Structured Output
Demonstra como obter saídas estruturadas do agent usando `OutputSchema`.

```bash
cd structured_output
go run main.go
```

### Input Schema
Demonstra como validar e estruturar entradas do agent usando `InputSchema`.

```bash
cd input_schema_on_agent
go run main.go
```

### Both Schemas
Demonstra como usar `InputSchema` e `OutputSchema` juntos (equivalente ao exemplo Python).

```bash
cd both_schemas
go run main.go
```

## Como Funciona

### Output Schema

```go
type MovieScript struct {
    Name    string   `json:"name" description:"Movie title"`
    Genre   string   `json:"genre" description:"Movie genre"`
    Characters []string `json:"characters" description:"Main characters"`
}

agent, _ := agent.NewAgent(agent.AgentConfig{
    Model:         model,
    OutputSchema:  MovieScript{},
    ParseResponse: true, // Enable automatic parsing
})

run, _ := agent.Run("Create a movie set in New York")
movieScript := run.ParsedOutput.(*MovieScript)
```

### Input Schema

```go
type ResearchTopic struct {
    Topic          string   `json:"topic"`
    FocusAreas     []string `json:"focus_areas"`
    TargetAudience string   `json:"target_audience"`
}

agent, _ := agent.NewAgent(agent.AgentConfig{
    Model:       model,
    InputSchema: ResearchTopic{},
})

// Pass struct directly (no need to marshal to JSON)
myTopic := ResearchTopic{
    Topic: "AI",
    FocusAreas: []string{"ML", "DL"},
}
run, _ := agent.Run(myTopic)
```

### Both Together (Like Python)

```go
// Define schemas
type ResearchTopic struct {
    Topic           string `json:"topic"`
    SourcesRequired int    `json:"sources_required"`
}

type ResearchOutput struct {
    Summary      string   `json:"summary"`
    Insights     []string `json:"insights"`
    Technologies []string `json:"technologies"`
}

// Create agent with both schemas
agent, _ := agent.NewAgent(agent.AgentConfig{
    Model:         model,
    InputSchema:   ResearchTopic{},
    OutputSchema:  ResearchOutput{},
    ParseResponse: true,
})

// Run with struct input (like Python: input=ResearchTopic(...))
run, _ := agent.Run(ResearchTopic{
    Topic: "AI",
    SourcesRequired: 5,
})

// Access parsed output (like Python: response.content)
result := run.ParsedOutput.(*ResearchOutput)
fmt.Println(result.Summary)
```

## Diferenças do Python

No Python:
```python
response = agent.run(input=ResearchTopic(topic="AI", sources_required=5))
print(response.content)
```

No Go (equivalente):
```go
run, _ := agent.Run(ResearchTopic{Topic: "AI", SourcesRequired: 5})
result := run.ParsedOutput.(*ResearchOutput)
```

A principal diferença é que em Go você passa o struct diretamente sem `input=`, e acessa o resultado via `run.ParsedOutput` com type assertion.
