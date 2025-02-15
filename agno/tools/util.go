package tools

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared"
)

// ConvertToOpenAITool converte uma ferramenta Tool para o formato esperado pelo OpenAI.
func ConvertToToos(tool Tool) Tools {
	// Gera o esquema JSONSchema dos parâmetros.
	paramsSchema, err := GenerateJSONSchema(tool.GetParameterStruct())
	if err != nil {
		panic(fmt.Errorf("failed to generate JSONSchema: %w", err))
	}

	functionParameters := shared.FunctionParameters(paramsSchema)

	return Tools{
		Type: "function",
		Function: &FunctionDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  functionParameters,
		},
	}
}

func ConvertToTool(tool Tool) (map[string]interface{}, error) {
	// Gera o esquema JSONSchema dos parâmetros.
	paramsSchema, err := GenerateJSONSchema(tool.GetParameterStruct())
	if err != nil {
		return nil, fmt.Errorf("failed to generate JSONSchema: %w", err)
	}

	return map[string]interface{}{
		"type": openai.ChatCompletionToolTypeFunction,
		"function": map[string]interface{}{
			"name":        tool.Name(),
			"description": tool.Description(),
			"parameters":  paramsSchema,
		},
	}, nil
}

// GenerateJSONSchema gera um esquema JSONSchema a partir de uma estrutura Go.
func GenerateJSONSchema(paramStruct interface{}) (map[string]interface{}, error) {
	t := reflect.TypeOf(paramStruct)
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
		if field.PkgPath != "" && !field.Anonymous { // Ignora campos não exportados.
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

		// Adiciona o campo como obrigatório se ele não tiver a tag `omitempty`.
		if !strings.Contains(tag, "omitempty") {
			schema["required"] = append(schema["required"].([]string), name)
		}
	}

	return schema, nil
}
