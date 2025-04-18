package utils

import (
	"fmt"
	"reflect"
	"strings"
)

func StructToMap(input interface{}) (map[string]interface{}, error) {
	val := reflect.ValueOf(input)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("input must be a struct or pointer to struct")
	}

	result := make(map[string]interface{})
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		structField := typ.Field(i)

		// Ignora campos não exportados
		if structField.PkgPath != "" {
			continue
		}

		// Ignora o campo ToolCall explicitamente
		if structField.Name == "ToolCall" {
			continue
		}

		// Ignora campos do tipo func
		if field.Kind() == reflect.Func {
			continue
		}

		// Ignora valores nulos, zero ou vazios
		if isEmptyValue(field) {
			continue
		}

		// Obtém o nome do campo (usando tag JSON, se disponível)
		tag := structField.Tag.Get("json")
		if tag == "" || tag == "-" {
			tag = structField.Name
		} else {
			tag = strings.Split(tag, ",")[0] // remove "omitempty"
		}

		result[tag] = field.Interface()
	}

	return result, nil
}

// isEmptyValue retorna true se o valor é nulo, zero ou vazio
func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Invalid:
		return true
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	}
	return false
}
