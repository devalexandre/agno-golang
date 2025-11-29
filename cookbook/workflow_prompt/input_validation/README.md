# Input Schema Validation Example

This example demonstrates how to use **Input Schema Validation** in workflows to ensure type safety and required field validation.

## Concept

Input Schema Validation allows you to:
1. Define expected input structure using Go structs
2. Validate input types at runtime
3. Enforce required fields using struct tags
4. Get clear error messages for invalid inputs

## How It Works

```
Input → Validation → Workflow Execution
         ↓ (if invalid)
      Error with details
```

1. Define input schema as a struct
2. Configure workflow with `WithInputSchema()`
3. Workflow validates input before execution
4. Clear errors if validation fails

## Running the Example

```bash
go run main.go
```

## Configuration

```go
type WorkflowInput struct {
    Query    string `json:"query" validate:"required"`
    MaxSteps int    `json:"max_steps"`
    UserID   string `json:"user_id" validate:"required"`
}

workflow := v2.NewWorkflow(
    v2.WithInputSchema(&WorkflowInput{}),
    // ... other options
)
```

## Validation Rules

- **Type Matching**: Input must match schema type
- **Required Fields**: Fields with `validate:"required"` tag must be non-zero
- **Nil Check**: Nil inputs are rejected
- **Clear Errors**: Detailed error messages for debugging

## Test Cases

The example includes 4 test cases:

1. ✅ **Valid Input** - All required fields present
2. ❌ **Missing Required Field** - Validation fails
3. ❌ **Wrong Type** - Type mismatch error
4. ❌ **Nil Input** - Nil check fails

## Benefits

- **Type Safety**: Catch type errors before execution
- **Early Validation**: Fail fast with clear errors
- **Documentation**: Schema serves as input documentation
- **Maintainability**: Centralized input validation logic

## Use Cases

- API endpoints with strict input requirements
- Data processing pipelines
- Multi-step workflows with complex inputs
- Production systems requiring input validation
