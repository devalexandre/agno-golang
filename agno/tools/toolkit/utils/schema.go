package utils

import (
	"reflect"
	"strings"
)

func GenerateJSONSchema(inputStruct interface{}) map[string]interface{} {
	t := reflect.TypeOf(inputStruct)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	properties := make(map[string]interface{})
	var requiredFields []string

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if !field.IsExported() {
			continue
		}

		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		jsonName := extractJSONName(jsonTag)

		fieldType := mapGoTypeToJSONType(field.Type)

		property := map[string]interface{}{
			"type": fieldType,
		}

		if desc := field.Tag.Get("description"); desc != "" {
			property["description"] = desc
		}

		if field.Tag.Get("required") == "true" {
			requiredFields = append(requiredFields, jsonName)
		}

		// Ensure arrays always have items defined
		if field.Type.Kind() == reflect.Slice {
			itemType := mapGoTypeToJSONType(field.Type.Elem())
			property["items"] = map[string]interface{}{
				"type": itemType,
			}
		}

		properties[jsonName] = property
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}

	if len(requiredFields) > 0 {
		schema["required"] = requiredFields
	}

	return schema
}

func mapGoTypeToJSONType(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Slice:
		return "array"
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int32, reflect.Int64, reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Struct:
		return "object"
	case reflect.Ptr:
		return mapGoTypeToJSONType(t.Elem())
	default:
		return "string"
	}
}

func extractJSONName(tag string) string {
	if idx := strings.Index(tag, ","); idx != -1 {
		return tag[:idx]
	}
	return tag
}
