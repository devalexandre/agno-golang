# Max Tool Calls from History Example

This example demonstrates how to use the `MaxToolCallsFromHistory` feature to limit the number of tool calls included in the agent's context.

## Feature Description

The `MaxToolCallsFromHistory` parameter allows you to control how many tool calls from previous conversations are included when the agent processes new requests. This is useful for:

- Managing context size and API costs
- Preventing very long conversation histories from overwhelming the model
- Maintaining focus on recent tool interactions

## Usage

```go
package main

import (
    "context"
    "log"
    
    "github.com/devalexandre/agno-golang/agno/agent"
    "github.com/devalexandre/agno-golang/agno/models/ollama"
    "github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
    ctx := context.Background()
    
    // Create model
    model, err := ollama.NewOllamaChat(
        models.WithID("llama3.2"),
        models.WithBaseURL("http://localhost:11434"),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // Create agent with tool call history limit
    agent, err := agent.NewAgent(agent.AgentConfig{
        Context:                 ctx,
        Model:                   model,
        Name:                    "Assistant",
        Instructions:            "You are a helpful assistant.",
        Tools:                   []toolkit.Tool{/* your tools */},
        AddHistoryToMessages:    true,
        MaxToolCallsFromHistory: 3, // Only keep 3 most recent tool calls
        Debug:                   true,
    })
    if err != nil {
        log.Fatal(err)
    }
    
    // Use the agent
    response, err := agent.Run("Hello!")
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println(response.TextContent)
}
```

## Configuration Options

- `MaxToolCallsFromHistory: 0` - No limit, all tool calls are included (default behavior)
- `MaxToolCallsFromHistory: N` - Only keep the N most recent tool calls in context
- The feature only affects messages with tool calls; regular conversation messages are always included

## Running the Example

Make sure you have Ollama running with llama3.2:latest model:

```bash
# Start Ollama
ollama serve

# Pull the model (if not already available)
ollama pull llama3.2:latest

# Run the example
cd examples/max_tool_calls_example
go run main.go
```

## How It Works

1. When `MaxToolCallsFromHistory` is set to a positive number, the agent filters the message history
2. It counts tool calls from the most recent messages backwards
3. Once the limit is reached, older messages containing tool calls are excluded from the context
4. Messages without tool calls (regular conversation) are always included
5. This helps keep the context focused on recent tool interactions while maintaining conversation flow

## Example Output

The example will show a table with:
- **Run**: Sequential number of the request
- **City**: City being queried
- **Current**: Tool calls in the current response
- **In Context**: Total tool calls available to the model (limited by MaxToolCallsFromHistory)
- **Response Preview**: Truncated response from the agent

## Benefits

- **Cost Control**: Reduces token usage by limiting historical tool calls
- **Performance**: Smaller context size leads to faster processing
- **Focus**: Keeps the agent focused on recent tool interactions
- **Flexibility**: Can be adjusted based on your specific use case