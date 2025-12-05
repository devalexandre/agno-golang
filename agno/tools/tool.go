package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// toCamelCase converts a string like "validates input data" to "validatesInputData"
// Useful for tool names to be compatible with Ollama
func toCamelCase(s string) string {
	// Split by spaces and non-alphanumeric characters
	words := strings.FieldsFunc(s, func(r rune) bool {
		return !isAlphaNumeric(r)
	})

	if len(words) == 0 {
		return ""
	}

	// First word stays lowercase
	result := strings.ToLower(words[0])

	// Remaining words are capitalized
	for i := 1; i < len(words); i++ {
		if len(words[i]) > 0 {
			result += strings.ToUpper(words[i][:1]) + strings.ToLower(words[i][1:])
		}
	}

	return result
}

// isAlphaNumeric checks if a rune is alphanumeric
func isAlphaNumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// Tool represents a simple function-based tool that can be called by an agent.
// This matches Python's simple tool pattern - just wrap a regular function.
type Tool struct {
	// Name of the tool
	Name string

	// Description of what the tool does
	Description string

	// JSON Schema describing the parameters
	Parameters map[string]interface{}

	// The actual function to execute
	// Signature: func(ctx context.Context, args map[string]interface{}) (interface{}, error)
	Entrypoint func(ctx context.Context, args map[string]interface{}) (interface{}, error)

	// Original function (for reflection-based calls)
	fn      reflect.Value
	fnType  reflect.Type
	methods map[string]toolkit.Method // For toolkit.Tool compatibility
}

// NewToolFromFunction creates a Tool from a simple Go function.
// Works just like Python's @tool decorator!
//
// Example:
//
//	func add(a int, b int) (int, error) {
//	    return a + b, nil
//	}
//	tool := NewToolFromFunction(add, "Add two numbers")
//	// Now use tool with Agent!
func NewToolFromFunction(fn interface{}, description string) *Tool {
	fnValue := reflect.ValueOf(fn)
	fnType := fnValue.Type()

	if fnType.Kind() != reflect.Func {
		panic(fmt.Sprintf("expected function, got %v", fnType.Kind()))
	}

	// Use description as name: convert to camelCase for Ollama compatibility
	name := toCamelCase(description)

	// Generate schema from function signature
	schema := generateSchemaFromFunction(fnType)

	// Create wrapper
	wrapper := createFunctionWrapper(fnValue, fnType)

	// Create the tool
	tool := &Tool{
		Name:        name,
		Description: description,
		Parameters:  schema,
		Entrypoint:  wrapper,
		fn:          fnValue,
		fnType:      fnType,
		methods:     make(map[string]toolkit.Method),
	}

	// Register the default method for this tool
	tool.methods[name] = toolkit.Method{
		Receiver:    tool,
		Description: description,
		Function:    wrapper,
		Schema:      schema,
	}

	return tool
}

// extractFunctionName gets the function name from a function value
func extractFunctionName(fn interface{}) string {
	// Try to get name from reflect
	_ = reflect.ValueOf(fn)

	// For lambda functions and closures, just use a generic name
	// In production, you might use runtime.FuncForPC to get the real name
	return "function"
}

// generateSchemaFromFunction creates a JSON Schema from function parameters
func generateSchemaFromFunction(fnType reflect.Type) map[string]interface{} {
	properties := make(map[string]interface{})
	var required []string
	var paramIndex int

	// Check if first parameter is context.Context
	startIndex := 0
	if fnType.NumIn() > 0 && fnType.In(0) == reflect.TypeOf((*context.Context)(nil)).Elem() {
		startIndex = 1
	}

	// Process remaining parameters
	for i := startIndex; i < fnType.NumIn(); i++ {
		paramType := fnType.In(i)
		paramName := fmt.Sprintf("arg%d", paramIndex)
		paramIndex++

		properties[paramName] = typeToJSONSchema(paramType)
		required = append(required, paramName)
	}

	// Build schema
	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}

// typeToJSONSchema converts a Go type to JSON Schema
func typeToJSONSchema(t reflect.Type) map[string]interface{} {
	switch t.Kind() {
	case reflect.String:
		return map[string]interface{}{"type": "string"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return map[string]interface{}{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]interface{}{"type": "number"}
	case reflect.Bool:
		return map[string]interface{}{"type": "boolean"}
	case reflect.Slice:
		return map[string]interface{}{
			"type":  "array",
			"items": typeToJSONSchema(t.Elem()),
		}
	case reflect.Map:
		return map[string]interface{}{
			"type": "object",
		}
	default:
		return map[string]interface{}{"type": "string"}
	}
}

// createFunctionWrapper creates a wrapper that accepts map[string]interface{}
func createFunctionWrapper(fnValue reflect.Value, fnType reflect.Type) func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		var callArgs []reflect.Value
		paramIndex := 0

		// Check if function expects context.Context
		startIndex := 0
		if fnType.NumIn() > 0 && fnType.In(0) == reflect.TypeOf((*context.Context)(nil)).Elem() {
			callArgs = append(callArgs, reflect.ValueOf(ctx))
			startIndex = 1
		}

		// Convert arguments from map to function parameters
		for i := startIndex; i < fnType.NumIn(); i++ {
			paramType := fnType.In(i)
			paramName := fmt.Sprintf("arg%d", paramIndex)
			paramIndex++

			argValue, ok := args[paramName]
			if !ok {
				return nil, fmt.Errorf("missing required parameter: %s", paramName)
			}

			// Convert argument to the correct type
			converted, err := convertValue(argValue, paramType)
			if err != nil {
				return nil, fmt.Errorf("error converting parameter %s: %v", paramName, err)
			}

			callArgs = append(callArgs, converted)
		}

		// Call the function
		results := fnValue.Call(callArgs)

		// Handle return values
		if len(results) == 0 {
			return nil, nil
		}

		// Check if last return value is error
		var lastErr error
		lastResultIdx := len(results) - 1

		if lastResultIdx >= 0 {
			lastResult := results[lastResultIdx]
			if lastResult.Type().Implements(reflect.TypeOf((*error)(nil)).Elem()) {
				if !lastResult.IsNil() {
					lastErr = lastResult.Interface().(error)
				}
				lastResultIdx-- // Exclude error from result
			}
		}

		if lastErr != nil {
			return nil, lastErr
		}

		// Return the first result
		if lastResultIdx >= 0 {
			return results[0].Interface(), nil
		}

		return nil, nil
	}
}

// convertValue converts a value to the target type
func convertValue(value interface{}, targetType reflect.Type) (reflect.Value, error) {
	if value == nil {
		return reflect.New(targetType).Elem(), nil
	}

	sourceValue := reflect.ValueOf(value)
	sourceType := sourceValue.Type()

	// If types match, return as-is
	if sourceType == targetType {
		return sourceValue, nil
	}

	// Try conversion
	switch targetType.Kind() {
	case reflect.String:
		return reflect.ValueOf(fmt.Sprintf("%v", value)), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if sourceType.Kind() == reflect.Float64 {
			return reflect.ValueOf(int64(value.(float64))).Convert(targetType), nil
		}
		return reflect.ValueOf(value).Convert(targetType), nil
	case reflect.Float32, reflect.Float64:
		if sourceType.Kind() == reflect.Float64 {
			return reflect.ValueOf(value).Convert(targetType), nil
		}
	}

	if sourceValue.CanConvert(targetType) {
		return sourceValue.Convert(targetType), nil
	}

	return reflect.Value{}, fmt.Errorf("cannot convert %v to %v", sourceType, targetType)
}

// --- toolkit.Tool Interface Implementation (for compatibility with Agent) ---

// GetName returns the tool name
func (t *Tool) GetName() string {
	return t.Name
}

// GetDescription returns the tool description
func (t *Tool) GetDescription() string {
	return t.Description
}

// GetParameterStruct returns the parameter schema for a method
// For simple tools, there's only one method (the tool itself)
func (t *Tool) GetParameterStruct(methodName string) map[string]interface{} {
	if methodName == t.Name || methodName == "" {
		return t.Parameters
	}
	return make(map[string]interface{})
}

// GetMethods returns the methods available
// For simple tools created from functions, there's one implicit method
func (t *Tool) GetMethods() map[string]toolkit.Method {
	return t.methods
}

// GetFunction returns the function for a method
func (t *Tool) GetFunction(methodName string) interface{} {
	if methodName == t.Name || methodName == "" {
		return t.Entrypoint
	}
	return nil
}

// GetDescriptionOfMethod returns the description of a method
func (t *Tool) GetDescriptionOfMethod(methodName string) string {
	if methodName == t.Name || methodName == "" {
		return t.Description
	}
	return ""
}

// Execute executes the tool with the given arguments
func (t *Tool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	// Parse input
	var args map[string]interface{}
	if err := json.Unmarshal(input, &args); err != nil {
		return nil, fmt.Errorf("failed to parse arguments: %v", err)
	}

	// Execute the tool
	ctx := context.Background()
	return t.Entrypoint(ctx, args)
}
