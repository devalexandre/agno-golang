# Tool Hooks Example

This example demonstrates how to use **ToolBeforeHooks** and **ToolAfterHooks** to add middleware functionality to tool execution.

## Features Demonstrated

### ToolBeforeHooks
Executed **before** a tool is called:
- **Logging**: Track which tools are being called
- **Validation**: Validate input parameters (guardrails)
- **Rate Limiting**: Control how many times tools can be called
- **Authorization**: Check permissions before tool execution
- **Auditing**: Record tool usage for compliance

### ToolAfterHooks  
Executed **after** a tool completes:
- **Result Logging**: Track tool outputs
- **Audit Trail**: Record complete tool execution details
- **Result Validation**: Verify tool outputs are reasonable
- **Caching**: Store results for future use
- **Notifications**: Alert on specific conditions

## Use Cases

- **Security**: Validate inputs to prevent malicious tool usage
- **Compliance**: Audit trail for regulatory requirements  
- **Debugging**: Detailed logging of tool calls and results
- **Performance**: Rate limiting and caching
- **Monitoring**: Track tool usage patterns

## Running the Example

```bash
export OLLAMA_API_KEY=your_api_key_here
cd cookbook/agents/tool_hooks
go run main.go
```

## Code Structure

The example creates a calculator agent with multiple hooks:

**Before Hooks:**
1. Logging hook - tracks call count
2. Validation hook - checks numeric ranges
3. Rate limiting hook - prevents excessive calls

**After Hooks:**
1. Result logging hook - displays results
2. Audit trail hook - records full execution details
3. Result validation hook - checks for reasonable outputs

## Expected Output

The example runs two tests:
1. Valid calculation: `(5 + 3) * 2` - should succeed with all hooks executing
2. Invalid calculation: `2000000 * 3` - should fail validation hook due to out-of-range input

## Implementation Details

Tool hooks are configured in `AgentConfig`:

```go
agent.NewAgent(agent.AgentConfig{
    // ... other config ...
    ToolBeforeHooks: []func(ctx context.Context, toolName string, args map[string]interface{}) error{
        func(ctx context.Context, toolName string, args map[string]interface{}) error {
            // Your validation/logging logic here
            return nil // or error to abort
        },
    },
    ToolAfterHooks: []func(ctx context.Context, toolName string, args map[string]interface{}, result interface{}) error{
        func(ctx context.Context, toolName string, args map[string]interface{}, result interface{}) error {
            // Your result processing logic here
            return nil
        },
    },
})
```

## Key Points

- Hooks are executed **in order** as defined in the arrays
- If a **before hook** returns an error, tool execution is aborted
- If an **after hook** returns an error, the error is propagated but result is still returned
- Hooks have access to full context, tool name, arguments, and results
- Multiple hooks can be chained for complex workflows

## Model Used

- **Ollama Cloud**: `qwen2.5:14b-instruct-cloud`
- Requires `OLLAMA_API_KEY` environment variable

## Related Features

- **PreHooks**: Execute before agent.Run() starts
- **PostHooks**: Execute after agent.Run() completes
- **Guardrails**: More sophisticated validation (see guardrails example)
