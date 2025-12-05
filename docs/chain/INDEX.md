# ChainTool Documentation Index

## Overview

Complete documentation for the ChainTool feature in Agno, including error handling strategies, caching, and dynamic tool management.

## Files

### 1. [README.md](./README.md)
Main ChainTool documentation covering:
- Overview and architecture
- Basic usage
- Error handling strategies (RollbackNone, RollbackToStart, RollbackToPrevious, RollbackSkip)
- Conditional execution
- Caching
- Best practices
- Examples

### 2. [DYNAMIC_TOOLS.md](./DYNAMIC_TOOLS.md)
Complete guide for dynamic tool management:
- AddTool() - Add tools at runtime
- RemoveTool() - Remove tools by name
- GetTools() - List all tools
- GetToolByName() - Retrieve specific tool
- Use cases: feature flags, A/B testing, progressive enhancement
- Integration with ChainTool and error handling

## Quick Start

### Enable ChainTool

```go
agent, _ := agent.NewAgent(agent.AgentConfig{
    EnableChainTool: true,
    Tools: []toolkit.Tool{tool1, tool2, tool3},
    ChainToolErrorConfig: &agent.ChainToolErrorConfig{
        Strategy: agent.RollbackToPrevious,
        MaxRetries: 1,
    },
})
```

### Add/Remove Tools Dynamically

```go
// Add tool at runtime
newTool := tools.NewToolFromFunction(...)
agent.AddTool(newTool)

// Remove tool by name
agent.RemoveTool("Tool Name")

// Get all tools
tools := agent.GetTools()

// Get specific tool
tool := agent.GetToolByName("Tool Name")
```

## Error Handling Strategies

| Strategy | Behavior | Use Case |
|----------|----------|----------|
| **RollbackNone** | Stop on error | Critical operations |
| **RollbackToStart** | Restart pipeline | Transient failures |
| **RollbackToPrevious** | Use last success | Optional steps |
| **RollbackSkip** | Continue with current | Best-effort |

## Execution Flow

### Without ChainTool
```
Model decides 4 times → Tool 1 → Tool 2 → Tool 3 → Tool 4
```

### With ChainTool
```
Model decides once → [Tool 1 → Tool 2 → Tool 3 → Tool 4]
```

## Examples

- `cookbook/agents/chaintool_error_handling` - Error handling demo
- `cookbook/agents/chaintool_caching` - Caching demo
- `cookbook/agents/chaintool_parallel` - Parallel execution
- `cookbook/agents/chaintool_complete` - All features combined
- `cookbook/agents/chaintool_dynamic` - Dynamic tool management

## API Reference

### Agent Methods

```go
// Dynamic tool management
func (a *Agent) AddTool(tool toolkit.Tool) error
func (a *Agent) RemoveTool(toolName string) error
func (a *Agent) GetTools() []toolkit.Tool
func (a *Agent) GetToolByName(name string) toolkit.Tool

// Execution
func (a *Agent) Run(input interface{}, opts ...interface{}) (models.RunResponse, error)
```

### Configuration

```go
type AgentConfig struct {
    // ChainTool Configuration
    EnableChainTool       bool
    ChainToolErrorConfig  *ChainToolErrorConfig
    ChainToolCache        ChainToolCache
    
    // ... other fields
}

type ChainToolErrorConfig struct {
    Strategy   RollbackStrategy
    MaxRetries int
}
```

## Best Practices

1. **Order matters** - Validate early, expensive operations last
2. **Choose right strategy** - Match error handling to use case
3. **Validate input** - Check at first tool
4. **Handle errors gracefully** - Use error handling strategies
5. **Log changes** - Track dynamic tool modifications
6. **Test thoroughly** - Verify pipeline behavior

## Common Patterns

### Progressive Enhancement
```go
agent.AddTool(basicTool)
if advancedMode {
    agent.AddTool(advancedTool)
}
```

### Feature Flags
```go
if feature.IsEnabled("processing") {
    agent.AddTool(processingTool)
}
```

### Tool Swapping
```go
agent.RemoveTool("OldImplementation")
agent.AddTool(newImplementation)
```

## Key Features

✅ Sequential tool execution  
✅ Automatic data propagation  
✅ Error recovery strategies  
✅ Result caching  
✅ Dynamic tool management  
✅ Compatible with all ChainTool features  

## See Also

- [Agno Documentation](../../README.md)
- [Agent Documentation](../agent/)
- [Tools Documentation](../tools/)

---

**Last Updated:** December 2025
