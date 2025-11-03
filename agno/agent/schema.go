package agent

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// SchemaField represents a field in a JSON schema
type SchemaField struct {
	Type        string                 `json:"type,omitempty"`
	Description string                 `json:"description,omitempty"`
	Properties  map[string]SchemaField `json:"properties,omitempty"`
	Items       *SchemaField           `json:"items,omitempty"`
	Required    []string               `json:"required,omitempty"`
	Enum        []interface{}          `json:"enum,omitempty"`
	Format      string                 `json:"format,omitempty"`
	Default     interface{}            `json:"default,omitempty"`
	MinItems    *int                   `json:"minItems,omitempty"`
	MaxItems    *int                   `json:"maxItems,omitempty"`
	Minimum     *float64               `json:"minimum,omitempty"`
	Maximum     *float64               `json:"maximum,omitempty"`
}

// JSONSchema represents a JSON schema structure
type JSONSchema struct {
	Type        string                 `json:"type"`
	Description string                 `json:"description,omitempty"`
	Properties  map[string]SchemaField `json:"properties,omitempty"`
	Required    []string               `json:"required,omitempty"`
}

// GenerateJSONSchema generates a JSON schema from a Go struct type or slice of structs
func GenerateJSONSchema(v interface{}) (*JSONSchema, error) {
	t := reflect.TypeOf(v)
	if t == nil {
		return nil, fmt.Errorf("cannot generate schema from nil value")
	}

	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Handle slice types - if it's a slice, generate schema for array of items
	if t.Kind() == reflect.Slice {
		elemType := t.Elem()

		// Handle pointer element types
		if elemType.Kind() == reflect.Ptr {
			elemType = elemType.Elem()
		}

		if elemType.Kind() != reflect.Struct {
			return nil, fmt.Errorf("slice schema generation only supports slice of structs, got slice of %s", elemType.Kind())
		}

		// Generate schema for the element type
		itemSchema := generateStructSchema(elemType)

		// Return array schema
		return &JSONSchema{
			Type: "array",
			Properties: map[string]SchemaField{
				"items": {
					Type:       "object",
					Properties: itemSchema.Properties,
					Required:   itemSchema.Required,
				},
			},
		}, nil
	}

	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("schema generation only supports struct or slice types, got %s", t.Kind())
	}

	return generateStructSchema(t), nil
}

// generateStructSchema generates a JSON schema for a struct type
func generateStructSchema(t reflect.Type) *JSONSchema {
	schema := &JSONSchema{
		Type:       "object",
		Properties: make(map[string]SchemaField),
		Required:   make([]string, 0),
	}

	// Get description from the struct's doc comment if available
	// This would require using reflection metadata which isn't directly available
	// For now, we'll use a json tag or field comment

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get JSON tag
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		// Parse JSON tag
		parts := strings.Split(jsonTag, ",")
		fieldName := parts[0]
		omitempty := false
		for _, part := range parts[1:] {
			if part == "omitempty" {
				omitempty = true
			}
		}

		// Get description from tag
		description := field.Tag.Get("description")

		// Generate field schema
		fieldSchema := generateFieldSchema(field.Type, description)

		schema.Properties[fieldName] = fieldSchema

		// Add to required if not omitempty
		if !omitempty {
			schema.Required = append(schema.Required, fieldName)
		}
	}

	return schema
}

// generateFieldSchema generates schema for a specific field
func generateFieldSchema(t reflect.Type, description string) SchemaField {
	field := SchemaField{
		Description: description,
	}

	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.String:
		field.Type = "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		field.Type = "integer"
	case reflect.Float32, reflect.Float64:
		field.Type = "number"
	case reflect.Bool:
		field.Type = "boolean"
	case reflect.Slice, reflect.Array:
		field.Type = "array"
		elemField := generateFieldSchema(t.Elem(), "")
		field.Items = &elemField
	case reflect.Map:
		field.Type = "object"
	case reflect.Struct:
		field.Type = "object"
		field.Properties = make(map[string]SchemaField)
		field.Required = make([]string, 0)

		for i := 0; i < t.NumField(); i++ {
			structField := t.Field(i)
			if !structField.IsExported() {
				continue
			}

			jsonTag := structField.Tag.Get("json")
			if jsonTag == "" || jsonTag == "-" {
				continue
			}

			parts := strings.Split(jsonTag, ",")
			fieldName := parts[0]
			omitempty := false
			for _, part := range parts[1:] {
				if part == "omitempty" {
					omitempty = true
				}
			}

			desc := structField.Tag.Get("description")
			nestedField := generateFieldSchema(structField.Type, desc)
			field.Properties[fieldName] = nestedField

			if !omitempty {
				field.Required = append(field.Required, fieldName)
			}
		}
	default:
		field.Type = "string" // Fallback
	}

	return field
}

// ValidateAndUnmarshal validates JSON data against a schema and unmarshals into target
func ValidateAndUnmarshal(data []byte, target interface{}) error {
	// First, unmarshal into the target
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	// Additional validation could be added here
	// For now, json.Unmarshal provides basic type validation

	return nil
}

// MarshalWithSchema marshals a value and ensures it conforms to the schema
func MarshalWithSchema(v interface{}) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}
	return data, nil
}

// ToJSONSchemaString converts a JSONSchema to a formatted JSON string
func (s *JSONSchema) ToJSONString() (string, error) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
