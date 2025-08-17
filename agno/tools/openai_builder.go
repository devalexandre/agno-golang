package tools

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// OpenAITool represents the OpenAI API tool format
type OpenAITool struct {
	Type        string           `json:"type"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Parameters  OpenAIParameters `json:"parameters"`
	Strict      bool             `json:"strict"`
}

// OpenAIParameters represents the parameters structure for OpenAI tools
type OpenAIParameters struct {
	Type                 string                    `json:"type"`
	Properties           map[string]OpenAIProperty `json:"properties"`
	Required             []string                  `json:"required"`
	AdditionalProperties bool                      `json:"additionalProperties"`
}

// OpenAIProperty represents a parameter property
type OpenAIProperty struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

// BuildOpenAITools converts toolkit tools to OpenAI API format
func BuildOpenAITools(tools []toolkit.Tool) ([]OpenAITool, error) {
	var openaiTools []OpenAITool

	for _, tool := range tools {
		for methodName := range tool.GetMethods() {
			openaiTool, err := convertToOpenAITool(tool, methodName)
			if err != nil {
				return nil, fmt.Errorf("failed to convert tool %s.%s: %w", tool.GetName(), methodName, err)
			}
			openaiTools = append(openaiTools, openaiTool)
		}
	}

	return openaiTools, nil
}

// convertToOpenAITool converts a single toolkit tool method to OpenAI format
func convertToOpenAITool(tool toolkit.Tool, methodName string) (OpenAITool, error) {
	// Get parameter structure from the tool
	paramStruct := tool.GetParameterStruct(methodName)

	// Generate parameters schema
	parameters, err := generateOpenAIParameters(paramStruct)
	if err != nil {
		return OpenAITool{}, fmt.Errorf("failed to generate parameters for %s: %w", methodName, err)
	}

	return OpenAITool{
		Type:        "function",
		Name:        fmt.Sprintf("%s_%s", tool.GetName(), methodName),
		Description: tool.GetDescription(),
		Parameters:  parameters,
		Strict:      true,
	}, nil
}

// generateOpenAIParameters generates OpenAI-compatible parameters from Go struct
func generateOpenAIParameters(paramStruct interface{}) (OpenAIParameters, error) {
	t := reflect.TypeOf(paramStruct)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return OpenAIParameters{}, fmt.Errorf("expected struct, got %v", t.Kind())
	}

	properties := make(map[string]OpenAIProperty)
	var required []string

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if field.PkgPath != "" && !field.Anonymous {
			continue
		}

		// Get JSON tag
		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}

		// Parse field name from JSON tag
		fieldName := parseJSONFieldName(jsonTag, field.Name)
		if fieldName == "" {
			continue
		}

		// Get field type
		fieldType := field.Type
		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}

		// Convert Go type to OpenAI type
		openaiType, err := goTypeToOpenAIType(fieldType)
		if err != nil {
			return OpenAIParameters{}, fmt.Errorf("unsupported type for field %s: %w", fieldName, err)
		}

		// Get description from comment or tag
		description := field.Tag.Get("description")
		if description == "" {
			description = fmt.Sprintf("%s parameter", fieldName)
		}

		properties[fieldName] = OpenAIProperty{
			Type:        openaiType,
			Description: description,
		}

		// Check if field is required (no omitempty tag)
		if !strings.Contains(jsonTag, "omitempty") {
			required = append(required, fieldName)
		}
	}

	return OpenAIParameters{
		Type:                 "object",
		Properties:           properties,
		Required:             required,
		AdditionalProperties: false,
	}, nil
}

// parseJSONFieldName extracts field name from JSON tag
func parseJSONFieldName(jsonTag, defaultName string) string {
	if jsonTag == "" {
		return strings.ToLower(defaultName)
	}

	parts := strings.Split(jsonTag, ",")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}

	return strings.ToLower(defaultName)
}

// goTypeToOpenAIType converts Go types to OpenAI parameter types
func goTypeToOpenAIType(t reflect.Type) (string, error) {
	switch t.Kind() {
	case reflect.String:
		return "string", nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "integer", nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer", nil
	case reflect.Float32, reflect.Float64:
		return "number", nil
	case reflect.Bool:
		return "boolean", nil
	case reflect.Slice, reflect.Array:
		return "array", nil
	case reflect.Struct:
		return "object", nil
	default:
		return "", fmt.Errorf("unsupported type: %v", t.Kind())
	}
}

// ConvertToOpenAIToolsJSON converts tools to JSON format expected by OpenAI API
func ConvertToOpenAIToolsJSON(tools []toolkit.Tool) ([]byte, error) {
	openaiTools, err := BuildOpenAITools(tools)
	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(openaiTools, "", "  ")
}
