package toolkit

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// NewToolkit initializes a new empty Toolkit.
func NewToolkit() Toolkit {
	return Toolkit{
		methods: make(map[string]Method),
	}
}

// GetName returns the toolkit name.
func (tk *Toolkit) GetName() string {
	return tk.Name
}

// GetDescription returns the toolkit description.
func (tk *Toolkit) GetDescription() string {
	return tk.Description
}

// Register registers a method in the toolkit.
// methodName = Function name
// fn = Execution function
// paramExample = Example struct that represents the parameters for schema generation
func (tk *Toolkit) Register(methodName string, receiver interface{}, fn interface{}, paramExample interface{}) {
	if _, ok := tk.methods[methodName]; ok {
		panic(fmt.Sprintf("Register: method %s already registered", methodName))
	}

	if methodName == "" {
		panic("Register: methodName cannot be empty")
	}

	if receiver == nil {
		panic("Register: receiver cannot be nil")
	}

	if fn == nil {
		panic("Register: fn cannot be nil")
	}

	funcValue := reflect.ValueOf(fn)
	funcType := funcValue.Type()

	if funcType.Kind() != reflect.Func {
		panic("Register expects a function")
	}

	// Generate schema based on the provided struct
	paramType := reflect.TypeOf(paramExample)
	if paramType.Kind() == reflect.Ptr {
		paramType = paramType.Elem()
	}
	if paramType.Kind() != reflect.Struct {
		panic(fmt.Sprintf("Register: paramExample must be a struct, got %v", paramType.Kind()))
	}

	schema := GenerateSchemaFromType(paramType)

	fullMethodName := tk.Name + "_" + methodName

	tk.methods[fullMethodName] = Method{
		Receiver:  receiver,
		Function:  fn,
		Schema:    schema,
		ParamType: paramType,
	}
}

// GetMethods returns all methods registered in the toolkit.
func (tk *Toolkit) GetMethods() map[string]Method {
	return tk.methods
}

// GetFunction returns the execution function associated with a registered method.
func (tk *Toolkit) GetFunction(methodName string) interface{} {
	method, ok := tk.methods[tk.Name+"_"+methodName]
	if !ok {
		panic(fmt.Sprintf("GetFunction: method %s not found", methodName))
	}
	return method.Function
}

// Execute runs the function associated with a method, passing the JSON input.
func (tk *Toolkit) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	method, ok := tk.methods[methodName]
	if !ok {
		return nil, fmt.Errorf("Execute: method %s not found", methodName)
	}

	// Parse JSON to intermediate map
	var argsMap map[string]interface{}
	if err := json.Unmarshal(input, &argsMap); err != nil {
		return nil, fmt.Errorf("Execute: failed to parse input JSON: %w", err)
	}

	// Fix common types that come as strings
	for i := 0; i < method.ParamType.NumField(); i++ {
		field := method.ParamType.Field(i)
		jsonName := field.Tag.Get("json")
		if jsonName == "" {
			jsonName = strings.ToLower(field.Name)
		} else {
			jsonName = strings.Split(jsonName, ",")[0]
		}

		val, exists := argsMap[jsonName]
		if !exists {
			continue
		}

		switch field.Type.Kind() {
		case reflect.Float64:
			if strVal, ok := val.(string); ok {
				if f, err := strconv.ParseFloat(strVal, 64); err == nil {
					argsMap[jsonName] = f
				}
			}
		case reflect.Bool:
			if strVal, ok := val.(string); ok {
				if b, err := strconv.ParseBool(strVal); err == nil {
					argsMap[jsonName] = b
				}
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if strVal, ok := val.(string); ok {
				if i, err := strconv.ParseInt(strVal, 10, 64); err == nil {
					argsMap[jsonName] = i
				}
			}
		}
	}

	// Re-marshal corrected data
	cleanJSON, err := json.Marshal(argsMap)
	if err != nil {
		return nil, fmt.Errorf("Execute: failed to marshal corrected input: %w", err)
	}

	paramInstance := reflect.New(method.ParamType).Interface()
	if err := json.Unmarshal(cleanJSON, paramInstance); err != nil {
		return nil, fmt.Errorf("Execute: failed to unmarshal corrected input: %w", err)
	}

	args := []reflect.Value{
		reflect.ValueOf(paramInstance).Elem(),
	}
	resultValues := reflect.ValueOf(method.Function).Call(args)

	result := resultValues[0].Interface()
	var errResult error
	if !resultValues[1].IsNil() {
		errResult = resultValues[1].Interface().(error)
	}

	return result, errResult
}

// GetParameterStruct returns the JSON schema of parameters for the registered method.
func (tk *Toolkit) GetParameterStruct(methodName string) map[string]interface{} {
	method, ok := tk.methods[methodName]
	if !ok {
		panic(fmt.Sprintf("GetParameterStruct: method %s not found", methodName))
	}
	return method.Schema
}

// GenerateSchemaFromType generates a JSON Schema based on the provided type.
func GenerateSchemaFromType(paramType reflect.Type) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
		"required":   []string{},
	}

	properties := schema["properties"].(map[string]interface{})
	requiredFields := schema["required"].([]string)

	for i := 0; i < paramType.NumField(); i++ {
		field := paramType.Field(i)

		// Get field name from json tag
		fieldName := field.Tag.Get("json")
		if fieldName == "" || fieldName == "-" {
			fieldName = strings.ToLower(field.Name)
		} else {
			fieldName = strings.Split(fieldName, ",")[0] // remove ,omitempty
		}

		if fieldName == "" {
			continue
		}

		// Map to JSON Schema type
		typeStr := mapGoTypeToJSONType(field.Type.Kind())

		// Tag description
		description := field.Tag.Get("description")

		prop := map[string]interface{}{
			"type":        typeStr,
			"description": description,
		}
		// 🚀 If it's an array or slice, define items automatically!
		if field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array {
			elemType := field.Type.Elem().Kind()
			prop["items"] = map[string]interface{}{
				"type": mapGoTypeToJSONType(elemType),
			}
		}

		properties[fieldName] = prop

		// If the tag is required, add it
		if field.Tag.Get("required") == "true" {
			requiredFields = append(requiredFields, fieldName)
		}
	}

	// Update required fields
	schema["required"] = requiredFields

	return schema
}

// mapGoTypeToJSONType converts Go types to JSON Schema types.
func mapGoTypeToJSONType(kind reflect.Kind) string {
	switch kind {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Map, reflect.Struct:
		return "object"
	default:
		return "string"
	}
}
