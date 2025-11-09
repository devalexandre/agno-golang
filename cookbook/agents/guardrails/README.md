# Guardrails - Comprehensive Security Examples

## Overview

This directory contains comprehensive examples demonstrating guardrails for agent security and safety. Guardrails are validation rules that protect agents from malicious inputs, dangerous outputs, and resource abuse.

## Examples Structure

Each example is organized in its own directory following the pattern: `cookbook/agents/guardrails/{functionality}/{example_name}/main.go`

### 1. Input Validation (`input_validation/`)

Demonstrates input validation guardrails that protect against malicious inputs.

**Features:**
- Prompt injection detection
- Input length validation
- SQL injection detection
- Command injection detection

**Run:**
```bash
go run cookbook/agents/guardrails/input_validation/main.go
```

**Tests:**
- Normal input (passes)
- Prompt injection attempt (blocked)
- Very long input (blocked)
- SQL injection attempt (blocked)
- Command injection attempt (blocked)

### 2. Output Validation (`output_validation/`)

Demonstrates output validation guardrails that filter dangerous content from agent responses.

**Features:**
- Output content filtering
- Dangerous pattern detection
- Safe response validation

**Run:**
```bash
go run cookbook/agents/guardrails/output_validation/main.go
```

**Tests:**
- Normal query (passes)
- Technical question (passes)
- Data science query (passes)

### 3. Rate Limiting (`rate_limiting/`)

Demonstrates rate limiting guardrails that prevent abuse by limiting requests per user.

**Features:**
- Per-user rate limiting
- Time window-based limiting
- Automatic cleanup of old requests
- Thread-safe implementation

**Run:**
```bash
go run cookbook/agents/guardrails/rate_limiting/main.go
```

**Configuration:**
- Max Requests: 3
- Time Window: 10 seconds
- User ID: anonymous (default)

**Tests:**
- Requests 1-3 (allowed)
- Requests 4-5 (blocked - rate limit exceeded)

### 4. Loop Detection (`loop_detection/`)

Demonstrates loop detection guardrails that prevent infinite loops in agent execution.

**Features:**
- Per-run iteration tracking
- Configurable maximum iterations
- Manual counter reset capability

**Run:**
```bash
go run cookbook/agents/guardrails/loop_detection/main.go
```

**Configuration:**
- Max Iterations: 5
- Run ID: run123
- Tracking: Per-run iteration count

**Tests:**
- Iterations 1-5 (allowed)
- Iterations 6-7 (blocked - maximum iterations exceeded)

### 5. Complete Example (`complete_example/`)

Demonstrates a complete agent with all guardrails working together.

**Features:**
- Input validation (prompt injection, length)
- Output validation (content filtering, similarity)
- Tool guardrails (content filtering)

**Run:**
```bash
go run cookbook/agents/guardrails/complete_example/main.go
```

**Tests:**
- Safe query (passes)
- Injection attempt (blocked)
- Another safe query (passes)

## Guardrail Types

### Input Validation Guardrails

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

### Output Validation Guardrails

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

### Rate Limiting Guardrails

#### RateLimitGuardrail
Enforces rate limiting per user to prevent abuse:

```go
guardrail := agent.NewRateLimitGuardrail(100, 1*time.Minute) // 100 requests per minute
```

### Loop Detection Guardrails

#### LoopDetectionGuardrail
Detects and prevents infinite loops in agent execution:

```go
guardrail := agent.NewLoopDetectionGuardrail(10) // Max 10 iterations
```

## Usage Pattern

```go
// Create guardrails
inputGuardrails := []agent.Guardrail{
    agent.NewPromptInjectionGuardrail(),
    agent.NewInputLengthGuardrail(10000),
}

outputGuardrails := []agent.Guardrail{
    agent.NewOutputContentGuardrail(),
    agent.NewSemanticSimilarityGuardrail(0.9),
}

// Create agent with guardrails
ag, err := agent.NewAgent(agent.AgentConfig{
    Context:          ctx,
    Model:            model,
    InputGuardrails:  inputGuardrails,
    OutputGuardrails: outputGuardrails,
})

// Guardrails are automatically applied
response, err := ag.Run("What is AI?")
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

## Performance Impact

All guardrails execute efficiently:
- PromptInjectionGuardrail: ~1-2ms per check
- InputLengthGuardrail: <1ms per check
- OutputContentGuardrail: ~1-2ms per check
- RateLimitGuardrail: <1ms per check
- LoopDetectionGuardrail: <1ms per check
- SemanticSimilarityGuardrail: ~2-5ms per check

## Security Considerations

1. **Prompt Injection**: Always use PromptInjectionGuardrail for user-facing agents
2. **Output Filtering**: Use OutputContentGuardrail to prevent credential leakage
3. **Rate Limiting**: Implement rate limiting to prevent abuse
4. **Loop Detection**: Use loop detection to prevent resource exhaustion
5. **Semantic Similarity**: Detect stuck states with similarity guardrails

## Running All Examples

```bash
# Input Validation
go run cookbook/agents/guardrails/input_validation/main.go

# Output Validation
go run cookbook/agents/guardrails/output_validation/main.go

# Rate Limiting
go run cookbook/agents/guardrails/rate_limiting/main.go

# Loop Detection
go run cookbook/agents/guardrails/loop_detection/main.go

# Complete Example
go run cookbook/agents/guardrails/complete_example/main.go
```

## Environment Setup

All examples require the Ollama API key:

```bash
export OLLAMA_API_KEY=your_api_key
```

## Related Documentation

- [Agent Documentation](../../agno/agent/README.md)
- [Guardrails Implementation](../../agno/agent/guardrails.go)
- [Security Best Practices](../../docs/SECURITY.md)
