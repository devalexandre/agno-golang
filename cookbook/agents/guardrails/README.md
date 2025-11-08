# Guardrails Example

This example demonstrates how to use **Guardrails** for reusable validation and policy enforcement in your agents.

## What are Guardrails?

Guardrails are reusable validation components that can be applied at different stages:
- **Input Guardrails**: Validate user input before processing
- **Output Guardrails**: Validate agent output before returning
- **Tool Guardrails**: Validate tool calls before execution

## Key Differences from Hooks

| Feature | Hooks | Guardrails |
|---------|-------|------------|
| Purpose | Custom logic, logging, side effects | Validation and policy enforcement |
| Reusability | Function-specific | Highly reusable across agents |
| Structure | Simple functions | Structured interface with name/description |
| Composition | Sequential execution | Can be chained and combined |

## Use Cases

- **Security**: Input sanitization, SQL injection prevention
- **Compliance**: Content filtering, PII detection
- **Quality**: Output length limits, format validation
- **Business Rules**: Domain-specific validation

## Running the Example

```bash
export OLLAMA_API_KEY=your_api_key_here
cd cookbook/agents/guardrails
go run main.go
```

## Creating Custom Guardrails

### Method 1: GuardrailFunc (Simple)

```go
profanityGuardrail := &agent.GuardrailFunc{
    Name:        "ProfanityFilter",
    Description: "Blocks inappropriate content",
    CheckFunc: func(ctx context.Context, data interface{}) error {
        // Your validation logic
        return nil // or error to block
    },
}
```

### Method 2: Custom Struct (Advanced)

```go
type EmailValidationGuardrail struct{}

func (g *EmailValidationGuardrail) GetName() string {
    return "EmailValidation"
}

func (g *EmailValidationGuardrail) GetDescription() string {
    return "Validates email format"
}

func (g *EmailValidationGuardrail) Check(ctx context.Context, data interface{}) error {
    // Validation logic
    return nil
}
```

## Integration

```go
agent.NewAgent(agent.AgentConfig{
    // ... other config ...
    
    InputGuardrails: []agent.Guardrail{
        &ProfanityGuardrail{},
        &LengthGuardrail{MaxLength: 500},
    },
    
    OutputGuardrails: []agent.Guardrail{
        &ContentFilterGuardrail{},
    },
    
    ToolGuardrails: []agent.Guardrail{
        &SQLInjectionGuardrail{},
        &EmailValidationGuardrail{},
    },
})
```

## Guardrail Chain

Combine multiple guardrails:

```go
chain := &agent.GuardrailChain{
    Name: "SecurityChain",
    Guardrails: []agent.Guardrail{
        &ProfanityGuardrail{},
        &SQLInjectionGuardrail{},
        &PIIDetectionGuardrail{},
    },
}
```

## Model Used

- **Ollama Cloud**: `qwen2.5:14b-instruct-cloud`
- Requires `OLLAMA_API_KEY` environment variable

## Related Features

- **Hooks**: Custom logic at different execution stages
- **Tool Hooks**: Middleware for tool execution
- **InputSchema/OutputSchema**: Structured data validation
