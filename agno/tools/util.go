package tools

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

func mapToParameters(m map[string]interface{}) Parameters {
	b, err := json.Marshal(m)
	if err != nil {
		panic(fmt.Errorf("failed to marshal schema: %w", err))
	}
	var p Parameters
	err = json.Unmarshal(b, &p)
	if err != nil {
		panic(fmt.Errorf("failed to unmarshal schema into Parameters: %w", err))
	}
	return p
}

func ConvertToTools(tool Tool) Tools {
	// Generates the JSONSchema for the parameters.
	paramsSchema, err := GenerateJSONSchema(tool.GetParameterStruct())
	if err != nil {
		panic(fmt.Errorf("failed to generate JSONSchema: %w", err))
	}

	return Tools{
		Type: "function",
		Function: &FunctionDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  mapToParameters(paramsSchema),
		},
	}
}

// ConvertToolsToToolChoice converts a Tools struct to ToolChoice.
func ConvertToolsToToolChoice(tools Tools) (ToolCall, error) {
	var toolChoice ToolCall

	// Define the type.
	toolChoice.Type = ToolType(tools.Type)

	// Check if the Function field is defined.
	if tools.Function != nil {
		// Create an instance of ToolFunction.
		toolChoice.Function = FunctionCall{
			Name: tools.Function.Name,
		}

		// Serialize the parameters to a JSON string.

		parametersJSON, err := json.Marshal(tools.Function.Parameters)
		if err != nil {
			return ToolCall{}, fmt.Errorf("failed to serialize parameters: %w", err)
		}
		toolChoice.Function.Arguments = string(parametersJSON)

	}

	return toolChoice, nil
}

func ConvertToTool(tool Tool) (map[string]interface{}, error) {
	// Generate the JSONSchema for the parameters.
	paramsSchema, err := GenerateJSONSchema(tool.GetParameterStruct())
	if err != nil {
		return nil, fmt.Errorf("failed to generate JSONSchema: %w", err)
	}

	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        tool.Name(),
			"description": tool.Description(),
			"parameters":  paramsSchema,
		},
	}, nil
}

// GenerateJSONSchema generates a JSONSchema from a Go structure.
func GenerateJSONSchema(paramStruct interface{}) (map[string]interface{}, error) {
	t := reflect.TypeOf(paramStruct)
	if t.Kind() == reflect.Map {
		if t.Key().Kind() == reflect.String {
			m, ok := paramStruct.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("failed to convert map to map[string]interface{}")
			}
			props := make(map[string]interface{})
			for k, v := range m {
				props[k] = v
			}
			return map[string]interface{}{
				"type":       "object",
				"properties": props,
			}, nil
		}
		return nil, fmt.Errorf("unsupported map key type: %v", t.Key().Kind())
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected a struct, got %v", t.Kind())
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": make(map[string]interface{}),
		"required":   []string{},
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" && !field.Anonymous { // Ignore unexported fields.
			continue
		}

		tag := field.Tag.Get("json")
		if tag == "-" || tag == "" {
			continue
		}

		name := tag
		if idx := strings.Index(tag, ","); idx != -1 {
			name = tag[:idx]
		}
		if name == "" {
			name = field.Name
		}

		fieldType := field.Type
		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}

		switch fieldType.Kind() {
		case reflect.String:
			schema["properties"].(map[string]interface{})[name] = map[string]string{"type": "string"}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			schema["properties"].(map[string]interface{})[name] = map[string]string{"type": "integer"}
		case reflect.Float32, reflect.Float64:
			schema["properties"].(map[string]interface{})[name] = map[string]string{"type": "number"}
		case reflect.Bool:
			schema["properties"].(map[string]interface{})[name] = map[string]string{"type": "boolean"}
		case reflect.Struct:
			nestedSchema, err := GenerateJSONSchema(fieldType)
			if err != nil {
				return nil, fmt.Errorf("failed to generate nested schema for field %s: %w", name, err)
			}
			schema["properties"].(map[string]interface{})[name] = nestedSchema
		case reflect.Slice, reflect.Array:
			itemType := fieldType.Elem()
			if itemType.Kind() == reflect.Ptr {
				itemType = itemType.Elem()
			}
			switch itemType.Kind() {
			case reflect.String:
				schema["properties"].(map[string]interface{})[name] = map[string]interface{}{
					"type":  "array",
					"items": map[string]string{"type": "string"},
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				schema["properties"].(map[string]interface{})[name] = map[string]interface{}{
					"type":  "array",
					"items": map[string]string{"type": "integer"},
				}
			case reflect.Float32, reflect.Float64:
				schema["properties"].(map[string]interface{})[name] = map[string]interface{}{
					"type":  "array",
					"items": map[string]string{"type": "number"},
				}
			default:
				return nil, fmt.Errorf("unsupported array item type: %v", itemType.Kind())
			}
		default:
			return nil, fmt.Errorf("unsupported type: %v", fieldType.Kind())
		}

		// Add the field as required if it doesn't have the `omitempty` tag.
		if !strings.Contains(tag, "omitempty") {
			schema["required"] = append(schema["required"].([]string), name)
		}
	}

	return schema, nil
}
