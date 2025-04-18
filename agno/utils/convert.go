package utils

import (
	"fmt"
	"reflect"
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

		// Ignora campos do tipo func
		if field.Kind() == reflect.Func {
			continue
		}

		// Obtém o nome do campo (usando tag JSON, se disponível)
		tag := structField.Tag.Get("json")
		if tag == "" || tag == "-" {
			tag = structField.Name
		}

		result[tag] = field.Interface()
	}

	return result, nil
}
