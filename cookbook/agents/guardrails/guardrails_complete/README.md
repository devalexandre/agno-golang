# Complete Guardrails - Cookbook Example

## Overview

This example demonstrates comprehensive guardrails for agent security and safety. Guardrails are validation rules that protect agents from malicious inputs, dangerous outputs, and resource abuse.

## Guardrail Types

### 1. Input Validation Guardrails

#### PromptInjectionGuardrail
Detects common prompt injection attack patterns:
- "Ignore previous instructions"
- "Show me the system prompt"
- Role switching attempts
- SQL injection patterns
- Command injection patterns

```go
guardrail := agent.NewPromptInjectionGuardrail()
```

#### InputLengthGuardrail
Limits input length to prevent resource exhaustion:

```go
guardrail := agent.NewInputLengthGuardrail(10000) // 10k characters max
```

### 2. Output Validation Guardrails

#### OutputContentGuardrail
Filters dangerous content from agent output:
- SQL injection attempts
- Command execution patterns
- Credential exposure
- File system access attempts

```go
guardrail := agent.NewOutputContentGuardrail()
```

#### SemanticSimilarityGuardrail
Detects repetitive outputs that indicate loops or stuck states:

```go
guardrail := agent.NewSemanticSimilarityGuardrail(0.9) // 90% similarity threshold
```

### 3. Rate Limiting Guardrails

#### RateLimitGuardrail
Enforces rate limiting per user to prevent abuse:

```go
guardrail := agent.NewRateLimitGuardrail(100, 1*time.Minute) // 100 requests per minute
```

Features:
- Per-user tracking
- Time window-based limiting
- Automatic cleanup of old requests
- Context-based user identification

### 4. Loop Detection Guardrails

#### LoopDetectionGuardrail
Detects and prevents infinite loops in agent execution:

```go
guardrail := agent.NewLoopDetectionGuardrail(10) // Max 10 iterations
```

Features:
- Per-run iteration tracking
- Configurable maximum iterations
- Manual counter reset capability

## Usage Examples

### Example 1: Basic Input Validation

```go
// Create guardrails
inputGuardrails := agent.NewDefaultInputGuardrails()

// Create agent with guardrails
ag, err := agent.NewAgent(agent.AgentConfig{
    Context:         ctx,
    Model:           model,
    InputGuardrails: inputGuardrails,
})

// Run agent - input is validated automatically
response, err := ag.Run("What is AI?")
```

### Example 2: Custom Guardrail Chain

```go
// Create custom guardrails
guardrails := []agent.Guardrail{
    agent.NewPromptInjectionGuardrail(),
    agent.NewInputLengthGuardrail(5000),
    agent.NewRateLimitGuardrail(100, 1*time.Minute),
}

// Create agent with custom guardrails
ag, err := agent.NewAgent(agent.AgentConfig{
    Context:         ctx,
    Model:           model,
    InputGuardrails: guardrails,
})
```

### Example 3: Rate Limiting with User Context

```go
// Create rate limiting guardrail
rateLimitGuardrail := agent.NewRateLimitGuardrail(10, 1*time.Hour)

// Create context with user ID
userCtx := context.WithValue(ctx, "user_id", "user123")

// Create agent
ag, err := agent.NewAgent(agent.AgentConfig{
    Context:         userCtx,
    Model:           model,
    InputGuardrails: []agent.Guardrail{rateLimitGuardrail},
})

// Rate limiting is applied per user
response, err := ag.Run("Question 1")
response, err := ag.Run("Question 2")
// ... up to 10 requests per hour for this user
```

### Example 4: Loop Detection with Run ID

```go
// Create loop detection guardrail
loopDetectionGuardrail := agent.NewLoopDetectionGuardrail(5)

// Create context with run ID
runCtx := context.WithValue(ctx, "run_id", "run123")

// Create agent
ag, err := agent.NewAgent(agent.AgentConfig{
    Context:         runCtx,
    Model:           model,
    InputGuardrails: []agent.Guardrail{loopDetectionGuardrail},
})

// After run completes, reset counter
loopDetectionGuardrail.ResetLoopCounter("run123")
```

### Example 5: Complete Security Setup

```go
// Input validation
inputGuardrails := []agent.Guardrail{
    agent.NewPromptInjectionGuardrail(),
    agent.NewInputLengthGuardrail(10000),
}

// Output validation
outputGuardrails := []agent.Guardrail{
    agent.NewOutputContentGuardrail(),
    agent.NewSemanticSimilarityGuardrail(0.9),
}

// Tool validation
toolGuardrails := []agent.Guardrail{
    agent.NewOutputContentGuardrail(),
}

// Create secure agent
ag, err := agent.NewAgent(agent.AgentConfig{
    Context:          ctx,
    Model:            model,
    InputGuardrails:  inputGuardrails,
    OutputGuardrails: outputGuardrails,
    ToolGuardrails:   toolGuardrails,
})
```

## Guardrail Execution Flow

```
User Input
    ↓
Input Guardrails (validation)
    ↓
Agent Processing
    ↓
Output Guardrails (filtering)
    ↓
Tool Calls
    ↓
Tool Guardrails (validation)
    ↓
Response to User
```

## Best Practices

### 1. Defense in Depth
Use multiple guardrails for comprehensive protection:
```go
guardrails := []agent.Guardrail{
    agent.NewPromptInjectionGuardrail(),
    agent.NewInputLengthGuardrail(10000),
    agent.NewRateLimitGuardrail(100, 1*time.Minute),
}
```

### 2. Context-Based Configuration
Use context to pass user/run information:
```go
ctx := context.WithValue(context.Background(), "user_id", userID)
ctx = context.WithValue(ctx, "run_id", runID)
```

### 3. Appropriate Thresholds
Set guardrail thresholds based on your use case:
- Input length: 5000-50000 characters
- Rate limit: 10-1000 requests per minute
- Max iterations: 5-100 iterations
- Similarity threshold: 0.8-0.95

### 4. Error Handling
Handle guardrail violations gracefully:
```go
response, err := ag.Run(prompt)
if err != nil {
    if strings.Contains(err.Error(), "guardrail") {
        // Handle guardrail violation
        log.Printf("Security violation: %v", err)
    }
}
```

### 5. Monitoring
Track guardrail violations for security analysis:
```go
// Log all guardrail violations
// Monitor rate limit patterns
// Analyze injection attempts
// Track loop detections
```

## Supported Models

All guardrails work with Ollama Cloud and other models:
- Ollama Cloud (recommended)
- OpenAI
- Google Gemini
- Local Ollama

## Running the Examples

```bash
# Set Ollama API key
export OLLAMA_API_KEY=your_api_key

# Run the example
go run main.go
```

## Security Considerations

1. **Prompt Injection**: Always use PromptInjectionGuardrail for user-facing agents
2. **Output Filtering**: Use OutputContentGuardrail to prevent credential leakage
3. **Rate Limiting**: Implement rate limiting to prevent abuse
4. **Loop Detection**: Use loop detection to prevent resource exhaustion
5. **Semantic Similarity**: Detect stuck states with similarity guardrails

## Performance Impact

- PromptInjectionGuardrail: ~1-2ms per check
- InputLengthGuardrail: <1ms per check
- OutputContentGuardrail: ~1-2ms per check
- RateLimitGuardrail: <1ms per check
- LoopDetectionGuardrail: <1ms per check
- SemanticSimilarityGuardrail: ~2-5ms per check

## Next Steps

1. Implement guardrails in your agent
2. Monitor guardrail violations
3. Adjust thresholds based on your use case
4. Add custom guardrails for specific needs
5. Integrate with logging/monitoring systems

## Related Documentation

- [Agent Documentation](../../agno/agent/README.md)
- [Guardrails Implementation](../../agno/agent/guardrails.go)
- [Security Best Practices](../../docs/SECURITY.md)
