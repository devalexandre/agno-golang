# ChainTool - Complete Guide

ChainTool is a powerful feature in Agno that allows you to execute multiple tools in sequence, with automatic data propagation from one tool to the next.

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Basic Usage](#basic-usage)
4. [Error Handling Strategies](#error-handling-strategies)
5. [Conditional Execution](#conditional-execution)
6. [Caching](#caching)
7. [Dynamic Tools](#dynamic-tools)
8. [Best Practices](#best-practices)
9. [Examples](#examples)

---

## Overview

### What is ChainTool?

ChainTool is a tool orchestration pattern where:
- **Multiple tools execute in sequence**
- **Output of Tool N becomes input of Tool N+1**
- **The Agent sees only the first tool** (hidden tool chaining)
- **The model makes one decision** that triggers the entire chain

### Why Use ChainTool?

✅ **Simplicity** - Agent deals with single interface  
✅ **Deterministic** - Same sequence always  
✅ **Efficient** - Single LLM call triggers all tools  
✅ **Maintainable** - Encapsulated pipeline logic  
✅ **Resilient** - Built-in error recovery  

---

## Architecture

### Without ChainTool

```
User Input
  ↓
Agent sees 4 tools
  ↓
Model chooses: Tool 1
  ↓
Agent executes Tool 1
  ↓
Model sees result, chooses: Tool 2
  ↓
... (continues for each tool)
```

**Problem:** Model needs to make 4 decisions.

### With ChainTool

```
User Input
  ↓
Agent sees 1 tool (ChainTool wrapper)
  ↓
Model chooses: Tool 1 (only option)
  ↓
ChainTool automatically:
  - Tool 1 output → Tool 2 input
  - Tool 2 output → Tool 3 input
  - Tool 3 output → Tool 4 input
  ↓
Result returned to Agent
```

**Benefit:** Model makes 1 decision, deterministic execution.

---

## Basic Usage

### Step 1: Create Individual Tools

```go
tool1 := tools.NewToolFromFunction(
    func(ctx context.Context, input string) (string, error) {
        return strings.ToUpper(input), nil
    },
    "Convert to uppercase",
)

tool2 := tools.NewToolFromFunction(
    func(ctx context.Context, input string) (string, error) {
        return "_" + input + "_", nil
    },
    "Add underscores",
)
```

### Step 2: Enable ChainTool in Agent

```go
agent, err := agent.NewAgent(agent.AgentConfig{
    Model:           model,
    Tools:           []toolkit.Tool{tool1, tool2},
    EnableChainTool: true,  // ← Enable ChainTool mode
})
```

### Step 3: Agent Executes

```go
response, err := agent.Run("Transform this: hello")
```

**Execution Flow:**
```
Input: "hello"
  ↓ Tool 1: "hello" → "HELLO"
  ↓ Tool 2: "HELLO" → "_HELLO_"
  ↓
Output: "_HELLO_"
```

---

## Error Handling Strategies

When a tool fails in a ChainTool pipeline, you need a recovery strategy.

### 1. RollbackNone - STOP ❌

**Behavior:** Stops execution immediately on error.

```go
ChainToolErrorConfig: &agent.ChainToolErrorConfig{
    Strategy:   agent.RollbackNone,
    MaxRetries: 0,
}
```

**When to use:** Critical operations (payments, security)

---

### 2. RollbackToStart - RESTART 🔄

**Behavior:** Reverts to original input and restarts entire pipeline.

```go
ChainToolErrorConfig: &agent.ChainToolErrorConfig{
    Strategy:   agent.RollbackToStart,
    MaxRetries: 3,
}
```

**When to use:** Transient failures, API calls

---

### 3. RollbackToPrevious - USE LAST SUCCESS ⬅️

**Behavior:** Uses result from last successful tool, skips failed tool.

```go
ChainToolErrorConfig: &agent.ChainToolErrorConfig{
    Strategy:   agent.RollbackToPrevious,
    MaxRetries: 1,
}
```

**When to use:** Data enrichment, optional steps

---

### 4. RollbackSkip - IGNORE & CONTINUE ⏭️

**Behavior:** Skips failed tool, continues with next tool.

```go
ChainToolErrorConfig: &agent.ChainToolErrorConfig{
    Strategy:   agent.RollbackSkip,
    MaxRetries: 0,
}
```

**When to use:** Best-effort processing, ML features

---

## Conditional Execution

Execute tools only when certain conditions are met.

```go
tool := tools.NewToolFromFunction(
    func(ctx context.Context, input string) (string, error) {
        if len(input) > 5 {
            return process(input), nil
        }
        return input, nil  // Skip processing
    },
    "Process if condition met",
)
```

---

## Caching

Cache tool results to avoid re-execution.

```go
cache := agent.NewMemoryCache(5*time.Minute, 100)

agent, _ := agent.NewAgent(agent.AgentConfig{
    Tools:           tools,
    EnableChainTool: true,
    ChainToolCache:  cache,
})
```

**Benefits:**
- ✅ Faster execution
- ✅ Reduced API calls
- ✅ Lower costs

---

## Dynamic Tools

Add, remove, or modify tools at runtime.

```go
// Add a tool dynamically
newTool := tools.NewToolFromFunction(...)
agent.AddTool(newTool)

// Remove a tool
agent.RemoveTool("Tool Name")

// Get all tools
tools := agent.GetTools()

// Get specific tool
tool := agent.GetToolByName("Process")
```

See [Dynamic Tools Documentation](./DYNAMIC_TOOLS.md) for details.

---

## Best Practices

### 1. Order Matters

Tools execute in the order they're provided:

```go
// ✅ GOOD - Validate first
[]toolkit.Tool{validateTool, transformTool, enrichTool}

// ❌ BAD - Expensive first
[]toolkit.Tool{enrichTool, validateTool, transformTool}
```

### 2. Strategy Selection

```
Payment processing       → RollbackNone
API calls               → RollbackToStart
Data enrichment         → RollbackToPrevious
ML features             → RollbackSkip
```

### 3. Input Validation

Validate at the first tool:

```go
firstTool := tools.NewToolFromFunction(
    func(ctx context.Context, input string) (string, error) {
        if input == "" {
            return "", fmt.Errorf("input cannot be empty")
        }
        return process(input), nil
    },
    "Validate and process",
)
```

---

## Examples

### Example 1: Data Processing Pipeline

```go
validateTool := tools.NewToolFromFunction(
    func(ctx context.Context, data string) (string, error) {
        if data == "" {
            return "", fmt.Errorf("empty data")
        }
        return fmt.Sprintf("VALIDATED_%s", data), nil
    },
    "Validate data",
)

transformTool := tools.NewToolFromFunction(
    func(ctx context.Context, data string) (string, error) {
        return fmt.Sprintf("TRANSFORMED[%s]", data), nil
    },
    "Transform data",
)

agent, _ := agent.NewAgent(agent.AgentConfig{
    Model:           model,
    Tools:           []toolkit.Tool{validateTool, transformTool},
    EnableChainTool: true,
    ChainToolErrorConfig: &agent.ChainToolErrorConfig{
        Strategy:   agent.RollbackToPrevious,
        MaxRetries: 1,
    },
})

response, _ := agent.Run("Process: mydata")
```

### Example 2: Progressive Tool Addition

```go
agent, _ := agent.NewAgent(agent.AgentConfig{
    Tools:           []toolkit.Tool{basicTool},
    EnableChainTool: true,
})

if advancedMode {
    agent.AddTool(advancedTool)
}

response, _ := agent.Run(input)
```

---

## References

- [Dynamic Tools Documentation](./DYNAMIC_TOOLS.md)
- [Architecture Diagrams](./ARCHITECTURE.md)
- Agent Documentation
- Tools Documentation

---

**Questions?** Open an issue on GitHub
