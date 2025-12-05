# Dynamic Tool Management in ChainTool

## Overview

Dynamic Tool Management allows you to add, remove, and modify tools in an Agent at runtime, enabling flexible and adaptive pipelines.

## Features

- ✅ **AddTool** - Add new tools after agent creation
- ✅ **RemoveTool** - Remove tools by name
- ✅ **GetTools** - List all available tools
- ✅ **GetToolByName** - Retrieve specific tool
- ✅ **ChainTool Compatible** - Works seamlessly with ChainTool
- ✅ **Error Handling Compatible** - Maintains error recovery strategies

## API Reference

### AddTool

```go
func (a *Agent) AddTool(tool toolkit.Tool) error
```

Adds a new tool to the agent dynamically.

**Example:**
```go
newTool := tools.NewToolFromFunction(...)
err := agent.AddTool(newTool)
if err != nil {
    log.Fatal(err)
}
```

### RemoveTool

```go
func (a *Agent) RemoveTool(toolName string) error
```

Removes a tool by its name.

**Example:**
```go
err := agent.RemoveTool("Process Data")
```

### GetTools

```go
func (a *Agent) GetTools() []toolkit.Tool
```

Returns all tools currently in the agent.

**Example:**
```go
tools := agent.GetTools()
fmt.Printf("Agent has %d tools\n", len(tools))
```

### GetToolByName

```go
func (a *Agent) GetToolByName(name string) toolkit.Tool
```

Retrieves a specific tool by name.

**Example:**
```go
tool := agent.GetToolByName("Validate Data")
if tool != nil {
    fmt.Println("Tool found:", tool.GetName())
}
```

## Use Cases

### 1. Progressive Tool Enablement

```go
agent, _ := agent.NewAgent(agent.AgentConfig{
    Tools: []toolkit.Tool{basicTool},
})

if userHasPermission("advanced") {
    agent.AddTool(advancedTool)
}

response, _ := agent.Run(input)
```

### 2. A/B Testing

```go
if experimentGroup == "A" {
    agent.AddTool(algorithmA)
} else {
    agent.AddTool(algorithmB)
}
```

### 3. Conditional Tool Addition

```go
if dataType == "json" {
    agent.AddTool(jsonParserTool)
} else if dataType == "xml" {
    agent.AddTool(xmlParserTool)
}
```

### 4. Tool Swapping

```go
agent.RemoveTool("Old Transformer")
agent.AddTool(newTransformerTool)
```

### 5. Feature Flags

```go
if featureFlag.IsEnabled("advanced_processing") {
    agent.AddTool(advancedProcessingTool)
}
```

## Integration with ChainTool

Dynamic tools work seamlessly with ChainTool:

```go
agent, _ := agent.NewAgent(agent.AgentConfig{
    Tools:           []toolkit.Tool{tool1, tool2},
    EnableChainTool: true,
})

// Add tool dynamically - automatically integrated into chain
agent.AddTool(tool3)

// Chain now includes tool3 in sequence
response, _ := agent.Run(input)
```

## Integration with Error Handling

Error recovery strategies apply to dynamically added tools:

```go
agent, _ := agent.NewAgent(agent.AgentConfig{
    EnableChainTool: true,
    ChainToolErrorConfig: &agent.ChainToolErrorConfig{
        Strategy: agent.RollbackToPrevious,
    },
})

// New tool inherits error handling strategy
agent.AddTool(newTool)
```

## Best Practices

### 1. Validate Before Adding

```go
if tool == nil {
    return fmt.Errorf("cannot add nil tool")
}

if err := agent.AddTool(tool); err != nil {
    log.Printf("Failed to add tool: %v", err)
}
```

### 2. Check Existence

```go
existingTool := agent.GetToolByName("Process")
if existingTool != nil {
    agent.RemoveTool("Process")
}

agent.AddTool(newProcessTool)
```

### 3. Log Changes

```go
fmt.Printf("Before: %d tools\n", len(agent.GetTools()))
agent.AddTool(newTool)
fmt.Printf("After: %d tools\n", len(agent.GetTools()))
```

### 4. Handle Errors

```go
if err := agent.AddTool(tool); err != nil {
    utils.ErrorPanel(err)
    agent.AddTool(fallbackTool)
}
```

## Performance Considerations

- **AddTool:** O(1) - Constant time
- **RemoveTool:** O(n) - Linear search
- **GetTools:** O(1) - Direct access
- **GetToolByName:** O(n) - Linear search

---

**See Also:**
- [ChainTool Documentation](./README.md)
- [Error Handling Strategies](./README.md#error-handling-strategies)
