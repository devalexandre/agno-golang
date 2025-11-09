# Reasoning with Persistence - Cookbook Example

## Overview

This example demonstrates how to use the reasoning persistence factory with agents. The reasoning persistence allows agents to store and retrieve reasoning steps, enabling analysis and debugging of the agent's decision-making process.

## How the Agent Uses Persistence Internally

### 1. **Storing Reasoning Steps**
When an agent with `Reasoning` enabled executes:
- Each reasoning step is automatically captured
- Steps include: title, reasoning, action, result, confidence, tokens used, duration
- These steps are stored in the configured database via the `ReasoningPersistence`

### 2. **Retrieving Reasoning History**
After execution, you can retrieve the complete reasoning history:
- Access all steps taken during reasoning
- Analyze the agent's thought process
- Debug decision-making logic
- Measure performance metrics (tokens, duration, confidence)

### 3. **Analyzing Agent Behavior**
The persistence enables:
- **Debugging**: Understand why the agent made certain decisions
- **Optimization**: Identify bottlenecks in reasoning
- **Auditing**: Track all reasoning steps for compliance
- **Learning**: Analyze patterns in successful vs failed reasoning

## Utility for the Agent

### 1. **Transparency**
- See exactly how the agent arrived at its conclusion
- Understand the reasoning chain step-by-step
- Identify where the agent might have gone wrong

### 2. **Performance Analysis**
- Track token usage across reasoning steps
- Measure execution time per step
- Identify expensive reasoning operations
- Optimize model selection based on performance

### 3. **Quality Assurance**
- Verify reasoning quality with confidence scores
- Ensure reasoning follows expected patterns
- Detect anomalies in agent behavior
- Validate reasoning against business rules

### 4. **Continuous Improvement**
- Learn from past reasoning patterns
- Identify common failure modes
- Improve prompts based on reasoning analysis
- Fine-tune reasoning parameters

## Usage Examples

### Example 1: Basic Usage with SQLite

```go
// Create persistence
config := &reasoning.DatabaseConfig{
    Type:     reasoning.DatabaseTypeSQLite,
    Database: "/tmp/agno_reasoning.db",
}
persistence, err := reasoning.NewReasoningPersistence(config)

// Create agent with persistence
ag, err := agent.NewAgent(agent.AgentConfig{
    Context:              ctx,
    Model:                model,
    Reasoning:            true,
    ReasoningModel:       model,
    ReasoningPersistence: persistence,
})

// Run agent - reasoning steps are automatically stored
response, err := ag.Run("Solve this problem...")

// Later: Retrieve and analyze reasoning history
history, err := persistence.GetReasoningHistory(ctx, runID)
for _, step := range history.Steps {
    fmt.Printf("Step %d: %s\n", step.StepNumber, step.Title)
    fmt.Printf("  Reasoning: %s\n", step.Reasoning)
    fmt.Printf("  Confidence: %.2f\n", step.Confidence)
    fmt.Printf("  Tokens: %d\n", step.ReasoningTokens)
}
```

### Example 2: Environment-Based Configuration

```go
// Load configuration from environment
config := &reasoning.DatabaseConfig{
    Type:     reasoning.DatabaseType(os.Getenv("DB_TYPE")),
    Database: os.Getenv("DB_NAME"),
    Host:     os.Getenv("DB_HOST"),
    User:     os.Getenv("DB_USER"),
    Password: os.Getenv("DB_PASSWORD"),
}

persistence, err := reasoning.NewReasoningPersistence(config)

// Use with agent
ag, err := agent.NewAgent(agent.AgentConfig{
    // ... other config
    ReasoningPersistence: persistence,
})
```

### Example 3: Analyzing Reasoning Performance

```go
// After running agent
history, err := persistence.GetReasoningHistory(ctx, runID)

// Analyze performance
fmt.Printf("Total Steps: %d\n", len(history.Steps))
fmt.Printf("Total Tokens: %d\n", history.TotalTokens)
fmt.Printf("Reasoning Tokens: %d\n", history.ReasoningTokens)
fmt.Printf("Total Duration: %dms\n", history.TotalDuration)

// Analyze individual steps
for _, step := range history.Steps {
    fmt.Printf("Step %d: %s\n", step.StepNumber, step.Title)
    fmt.Printf("  Confidence: %.2f\n", step.Confidence)
    fmt.Printf("  Duration: %dms\n", step.Duration)
    fmt.Printf("  Tokens: %d\n", step.ReasoningTokens)
}
```

## Supported Databases

The factory supports multiple database backends:

- **SQLite**: Local file-based, perfect for development
- **PostgreSQL**: Enterprise-grade, production-ready
- **MySQL**: Open-source, widely available
- **MariaDB**: MySQL-compatible alternative
- **Oracle**: Enterprise database
- **SQL Server**: Microsoft database

## Key Benefits

1. **Transparency**: Understand agent decision-making
2. **Debugging**: Identify issues in reasoning
3. **Performance**: Measure and optimize reasoning
4. **Auditing**: Track all reasoning steps
5. **Learning**: Improve agents based on analysis
6. **Compliance**: Maintain records of reasoning

## Running the Examples

```bash
# Set Ollama API key
export OLLAMA_API_KEY=your_api_key

# Run the example
go run main.go
```

## Next Steps

1. Analyze reasoning patterns in your use case
2. Optimize reasoning parameters based on performance
3. Implement custom analysis on stored reasoning data
4. Use insights to improve agent prompts and configuration
5. Monitor reasoning quality over time

## Related Documentation

- [Persistence Factory Documentation](../../agno/reasoning/PERSISTENCE_FACTORY.md)
- [Factory Implementation Summary](../../agno/reasoning/FACTORY_IMPLEMENTATION_SUMMARY.md)
- [Reasoning Documentation](../../agno/reasoning/README.md)
