# OutputModel - Two-Stage Processing

## Overview

`OutputModel` allows you to use **two different models** to process a request:

1. **Main Model** (can be expensive/powerful): Generates creative content with simple prompts
2. **OutputModel** (can be cheap/fast): Formats the content into structured JSON

## Benefits

### 💰 Cost Savings
- Use expensive model only for content generation (shorter prompts)
- Use cheap model for mechanical JSON formatting
- Reduce tokens sent to main model (no schema instructions)

### 📊 Dual Outputs
- `response.TextContent`: Original creative response from main model
- `response.Output` / `pointer`: Structured JSON formatted by OutputModel

### 🎯 Separation of Concerns
- Main model: Focus on creativity and content quality
- OutputModel: Focus on formatting and precise structuring

## How It Works

### Execution Flow

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

## Basic Usage

### Complete Example

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

    // Main model - can be more expensive/powerful
    mainModel, _ := ollama.NewOllamaChat(
        models.WithID("llama3.2:latest"),
        models.WithBaseURL("http://localhost:11434"),
    )

    // Output model - can be cheaper/faster
    outputModel, _ := ollama.NewOllamaChat(
        models.WithID("llama3.2:latest"),
        models.WithBaseURL("http://localhost:11434"),
    )

    movieScript := &MovieScript{}

    // Configure agent with OutputModel
    agent, _ := agent.NewAgent(agent.AgentConfig{
        Context:       ctx,
        Model:         mainModel,
        OutputModel:   outputModel,    // Separate model for formatting
        OutputSchema:  movieScript,    // Schema to structure data
        Description:   "You are a creative movie script writer.",
        ParseResponse: true,
    })

    // Execute with simple prompt
    response, _ := agent.Run("Create a sci-fi movie about AI")

    // OUTPUT 1: Original text from main model
    fmt.Println("Creative Content:")
    fmt.Println(response.TextContent)

    // OUTPUT 2: Structured JSON via OutputModel
    fmt.Println("\nStructured Data:")
    movieJSON, _ := json.MarshalIndent(movieScript, "", "  ")
    fmt.Println(string(movieJSON))

    // Also accessible via response.Output
    if script, ok := response.Output.(*MovieScript); ok {
        fmt.Printf("\nMovie: %s (%s)\n", script.Name, script.Genre)
    }
}
```

## Custom Prompt

You can customize the prompt used by OutputModel:

```go
customPrompt := `You are a JSON formatter. Convert the text into strict JSON.
Be extremely concise. Use short, punchy descriptions.

Return ONLY valid JSON. No explanations, no markdown.`

agent, _ := agent.NewAgent(agent.AgentConfig{
    Context:           ctx,
    Model:             mainModel,
    OutputModel:       outputModel,
    OutputModelPrompt: customPrompt,  // Custom instruction
    OutputSchema:      movieScript,
    ParseResponse:     true,
})
```

## Comparison: With vs Without OutputModel

### Without OutputModel (traditional)

```go
agent, _ := agent.NewAgent(agent.AgentConfig{
    Model:         mainModel,
    OutputSchema:  movieScript,
    ParseResponse: true,
})
```

**Flow:**
1. Main model receives: prompt + schema instructions
2. Main model returns: Structured JSON
3. Agent parses the JSON

**Problem:**
- Larger prompt (includes schema) = more tokens = more expensive
- Expensive model used for mechanical task (JSON formatting)

### With OutputModel (optimized)

```go
agent, _ := agent.NewAgent(agent.AgentConfig{
    Model:         mainModel,
    OutputModel:   outputModel,
    OutputSchema:  movieScript,
    ParseResponse: true,
})
```

**Flow:**
1. Main model receives: simple prompt (no schema)
2. Main model returns: creative text
3. OutputModel receives: text + schema
4. OutputModel returns: structured JSON

**Advantages:**
- ✅ Shorter prompt for expensive model
- ✅ Cheap model for formatting
- ✅ Two outputs available
- ✅ Better content quality

## Use Cases

### 1. Cost Reduction
```go
// GPT-4 for content, GPT-3.5 for formatting
mainModel := openai.NewOpenAI("gpt-4")
outputModel := openai.NewOpenAI("gpt-3.5-turbo")
```

### 2. Latency Optimization
```go
// Large model for quality, small model for speed
mainModel := ollama.NewOllama("llama3.2:70b")
outputModel := ollama.NewOllama("llama3.2:3b")
```

### 3. Specialization
```go
// Creative model for content, structured model for JSON
mainModel := anthropic.NewClaude("claude-3-opus")
outputModel := openai.NewOpenAI("gpt-4-structured")
```

## Internal Implementation

The `ApplyOutputFormatting` method follows the same pattern as `ApplySemanticCompression`:

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

## Related Fields

### AgentConfig

```go
type AgentConfig struct {
    // ... other fields ...
    
    // OutputSchema defines the expected output structure
    OutputSchema interface{}
    
    // OutputModel is the model used for JSON formatting
    // If nil, the main model handles formatting
    OutputModel models.AgnoModelInterface
    
    // OutputModelPrompt customizes the OutputModel prompt
    // If empty, uses default prompt
    OutputModelPrompt string
    
    // ParseResponse enables/disables automatic parsing
    ParseResponse bool
}
```

### RunResponse

```go
type RunResponse struct {
    // TextContent: original response from main model
    TextContent string
    
    // Output: structured data (filled pointer)
    Output interface{}
    
    // ParsedOutput: deprecated, use Output
    ParsedOutput interface{}
    
    // ... other fields ...
}
```

## Practical Examples

### Example 1: Data Analysis

```go
type DataAnalysis struct {
    Summary    string   `json:"summary"`
    KeyPoints  []string `json:"key_points"`
    Metrics    map[string]float64 `json:"metrics"`
}

analysis := &DataAnalysis{}

agent, _ := agent.NewAgent(agent.AgentConfig{
    Model:         expensiveModel,  // Deep analysis
    OutputModel:   cheapModel,      // Simple formatting
    OutputSchema:  analysis,
})

response, _ := agent.Run("Analyze this dataset: ...")

// Detailed analytical text
fmt.Println(response.TextContent)

// Structured metrics
fmt.Printf("Metrics: %v\n", analysis.Metrics)
```

### Example 2: Content Generation

```go
type BlogPost struct {
    Title    string   `json:"title"`
    Tags     []string `json:"tags"`
    Content  string   `json:"content"`
    WordCount int     `json:"word_count"`
}

post := &BlogPost{}

agent, _ := agent.NewAgent(agent.AgentConfig{
    Model:       creativeModel,  // Creative writing
    OutputModel: structuredModel, // Metadata extraction
    OutputSchema: post,
})

response, _ := agent.Run("Write a blog post about AI")

// Full creative content
saveToFile(response.TextContent)

// Structured metadata for database
saveMetadata(post.Title, post.Tags, post.WordCount)
```

## Best Practices

### 1. Model Selection

```go
// ✅ GOOD: Large model for creativity, small for structure
mainModel := "llama3.2:70b"
outputModel := "llama3.2:3b"

// ❌ AVOID: Same model for both (no benefit)
mainModel := "llama3.2:latest"
outputModel := "llama3.2:latest"
```

### 2. Schema Design

```go
// ✅ GOOD: Detailed schema with descriptions
type Movie struct {
    Name  string `json:"name" description:"Movie title"`
    Genre string `json:"genre" description:"Genre (action, drama, etc)"`
}

// ❌ AVOID: Schema without context
type Movie struct {
    Name  string `json:"name"`
    Genre string `json:"genre"`
}
```

### 3. Custom Prompts

```go
// ✅ GOOD: Specific prompt for use case
customPrompt := `Extract structured data from the text.
Focus on accuracy over creativity.
Return valid JSON only.`

// ❌ AVOID: Generic prompt (use default)
customPrompt := "Convert to JSON"
```

## Troubleshooting

### OutputModel Not Being Used

**Symptom:** Output incorrectly formatted

**Solution:**
```go
// Ensure all required fields are configured
agent, _ := agent.NewAgent(agent.AgentConfig{
    OutputModel:   outputModel,    // ✅ Set model
    OutputSchema:  schema,         // ✅ Set schema
    ParseResponse: true,           // ✅ Enable parsing
})
```

### Pointer Not Being Filled

**Symptom:** `movieScript` is empty after `Run()`

**Solution:**
```go
// ✅ CORRECT: Pass pointer
movieScript := &MovieScript{}
OutputSchema: movieScript

// ❌ WRONG: Pass value
OutputSchema: MovieScript{}
```

### Slow Dual Model Calls

**Symptom:** Execution too slow

**Solution:**
```go
// Use faster model for OutputModel
outputModel := ollama.NewOllama("llama3.2:3b")  // ✅ Small/fast model
```

## References

- [INPUT_OUTPUT_SCHEMA.md](./INPUT_OUTPUT_SCHEMA.md) - Documentação completa de schemas
- [RELEASE_INPUT_OUTPUT_SCHEMA.md](../../RELEASE_INPUT_OUTPUT_SCHEMA.md) - Release notes
- Exemplo: [examples/input-output/output-model/](../../examples/input-output/output-model/)
