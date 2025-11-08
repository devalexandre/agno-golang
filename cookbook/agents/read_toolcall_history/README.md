# Read Tool Call History Tool Example

This example demonstrates the **ReadToolCallHistory** default tool, which allows agents to track and analyze their own tool usage.

## Overview

The `ReadToolCallHistory` tool provides two methods:
- `tool_history.read(limit)` - Read the N most recent tool calls
- `tool_history.stats()` - Get statistics about tool usage

## Features

### 1. **tool_history.read(limit)**
Retrieves the most recent tool calls from the conversation.

**Parameters:**
- `limit` (int): Number of recent tool calls to retrieve

**Returns:**
```json
{
  "tool_calls": [
    {
      "name": "Add",
      "arguments": {"a": 15, "b": 27},
      "timestamp": "2024-01-15T10:30:00Z"
    }
  ]
}
```

### 2. **tool_history.stats()**
Provides statistics about tool usage patterns.

**Returns:**
```json
{
  "total_calls": 10,
  "unique_tools": 3,
  "tool_counts": {
    "Add": 4,
    "Multiply": 3,
    "Divide": 3
  },
  "most_used": "Add"
}
```

## Requirements

- **Tools**: Agent must have tools registered
- **Enable Flag**: `agent.WithEnableReadToolCallHistoryTool(true)`
- **No Storage**: Works with in-memory message history (no storage backend needed)

## Use Cases

1. **Self-Monitoring**: Agent can track which tools it's using
2. **Debugging**: Identify tool usage patterns and issues
3. **Optimization**: Detect overuse or underuse of specific tools
4. **Reporting**: Generate usage reports for analysis
5. **Learning**: Agent can reflect on its tool usage

## Running the Example

```bash
# Set your Ollama API key
export OLLAMA_API_KEY=your_api_key

# Run the example
go run main.go
```

## Expected Output

The agent will:
1. Perform several calculator operations (add, multiply, divide)
2. Use `tool_history.read()` to retrieve recent tool calls
3. Use `tool_history.stats()` to get usage statistics
4. Demonstrate self-awareness of tool usage patterns

## Implementation Details

### Agent Configuration
```go
ag, err := agent.NewAgent(
    "CalculatorAssistant",
    model,
    agent.WithTools(calcTool),
    agent.WithEnableReadToolCallHistoryTool(true), // Enable the tool
)
```

### Tool Registration
The example uses a simple calculator toolkit:
```go
type CalculatorToolkit struct{}

func (ct *CalculatorToolkit) Add(a, b float64) float64 {
    return a + b
}

func (ct *CalculatorToolkit) Multiply(a, b float64) float64 {
    return a * b
}

func (ct *CalculatorToolkit) Divide(a, b float64) (float64, error) {
    if b == 0 {
        return 0, fmt.Errorf("division by zero")
    }
    return a / b, nil
}
```

### Tool Usage
The agent automatically has access to:
- `tool_history.read(limit)` - Read recent tool calls
- `tool_history.stats()` - Get usage statistics

## Comparison with Other Default Tools

| Feature | ReadToolCallHistory | ReadChatHistory | UpdateKnowledge |
|---------|-------------------|-----------------|-----------------|
| **Purpose** | Track tool usage | Read conversations | Manage knowledge |
| **Storage** | In-memory | Storage backend | Vector database |
| **Scope** | Tool calls only | All messages | Domain knowledge |
| **Methods** | read, stats | read, search | add, search |
| **Use Case** | Monitoring | Context recall | RAG |

## Best Practices

1. **Limit Results**: Use the `limit` parameter to avoid excessive data
2. **Regular Checks**: Have agent check stats periodically for long conversations
3. **Error Tracking**: Monitor tool failures and error patterns
4. **Performance**: Track tool execution times if extended
5. **Reporting**: Generate usage reports for debugging and optimization

## Data Structure

### Tool Call Entry
```go
type ToolCallEntry struct {
    Name       string                 // Tool name
    Arguments  map[string]interface{} // Call arguments
    Result     interface{}            // Tool result
    Error      string                 // Error if any
    Timestamp  time.Time              // When called
}
```

### Statistics
```go
type ToolStats struct {
    TotalCalls   int                // Total tool calls
    UniqueTools  int                // Number of unique tools used
    ToolCounts   map[string]int     // Count per tool
    MostUsed     string             // Most frequently used tool
    ErrorRate    float64            // Percentage of failed calls
}
```

## Extended Use Cases

### 1. **Rate Limiting**
```go
stats := tool_history.stats()
if stats.tool_counts["expensive_api"] > 10 {
    // Suggest using cache or alternative
}
```

### 2. **Error Analysis**
```go
recent := tool_history.read(20)
error_count := count_errors(recent)
if error_count > 5 {
    // Alert or suggest debugging
}
```

### 3. **Workflow Optimization**
```go
stats := tool_history.stats()
if stats.most_used == "search" {
    // Suggest adding more context upfront
}
```

## Notes

- The tool is automatically registered when `EnableReadToolCallHistoryTool` is true
- Tool history is tracked in the agent's message list
- No external storage required (uses in-memory message history)
- Statistics are computed on-demand from the message history
- Tool calls from all roles (assistant, tool) are tracked
- History persists only for the current agent instance

## Future Enhancements

Potential extensions for this tool:
- **Time-based filtering**: Filter by time range
- **Tool-specific history**: Get history for specific tools only
- **Performance metrics**: Track execution time per tool
- **Success rate**: Calculate success/failure rates
- **Trend analysis**: Identify usage patterns over time
