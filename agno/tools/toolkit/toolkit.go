package toolkit

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// NewToolkit inicializa um novo Toolkit vazio.
func NewToolkit() Toolkit {
	return Toolkit{
		methods: make(map[string]Method),
	}
}

// GetName retorna o nome da toolkit.
func (tk *Toolkit) GetName() string {
	return tk.Name
}

// GetDescription retorna a descrição da toolkit.
func (tk *Toolkit) GetDescription() string {
	return tk.Description
}

// Register registra um método na toolkit.
// methodName = Nome da função
// fn = Função de execução
// paramExample = Exemplo da struct que representa os parâmetros para geração do schema
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

	// Gera o schema baseado na struct informada
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

// GetMethods retorna todos os métodos registrados na toolkit.
func (tk *Toolkit) GetMethods() map[string]Method {
	return tk.methods
}

// GetFunction retorna a função de execução associada a um método registrado.
func (tk *Toolkit) GetFunction(methodName string) interface{} {
	method, ok := tk.methods[tk.Name+"_"+methodName]
	if !ok {
		panic(fmt.Sprintf("GetFunction: method %s not found", methodName))
	}
	return method.Function
}

// Execute executa a função associada a um método, passando o input JSON.
// Execute executa a função associada a um método, passando o input JSON.
func (tk *Toolkit) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	method, ok := tk.methods[methodName]
	if !ok {
		return nil, fmt.Errorf("Execute: method %s not found", methodName)
	}

	// Cria uma nova instância do tipo de parâmetro
	paramInstance := reflect.New(method.ParamType).Interface()

	// Faz o Unmarshal do input JSON para a struct
	if err := json.Unmarshal(input, paramInstance); err != nil {
		return nil, fmt.Errorf("Execute: failed to unmarshal input: %w", err)
	}

	// Prepara argumentos: apenas os parâmetros da função
	args := []reflect.Value{
		reflect.ValueOf(paramInstance).Elem(),
	}

	// Chama a função dinamicamente usando reflection
	resultValues := reflect.ValueOf(method.Function).Call(args)

	// Extrai resultados
	result := resultValues[0].Interface()
	var err error
	if !resultValues[1].IsNil() {
		err = resultValues[1].Interface().(error)
	}

	return result, err
}

// GetParameterStruct retorna o schema JSON de parâmetros para o método registrado.
func (tk *Toolkit) GetParameterStruct(methodName string) map[string]interface{} {
	method, ok := tk.methods[methodName]
	if !ok {
		panic(fmt.Sprintf("GetParameterStruct: method %s not found", methodName))
	}
	return method.Schema
}

// GenerateSchemaFromType gera um JSON Schema baseado no tipo informado.
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

		// Pega o nome do campo pela tag json
		fieldName := field.Tag.Get("json")
		if fieldName == "" || fieldName == "-" {
			fieldName = strings.ToLower(field.Name)
		} else {
			fieldName = strings.Split(fieldName, ",")[0] // remove ,omitempty
		}

		if fieldName == "" {
			continue
		}

		// Mapeia tipo JSON Schema
		typeStr := mapGoTypeToJSONType(field.Type.Kind())

		// Descrição da tag
		description := field.Tag.Get("description")

		prop := map[string]interface{}{
			"type":        typeStr,
			"description": description,
		}
		// 🚀 Se for array ou slice, define o items automaticamente!
		if field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array {
			elemType := field.Type.Elem().Kind()
			prop["items"] = map[string]interface{}{
				"type": mapGoTypeToJSONType(elemType),
			}
		}

		properties[fieldName] = prop

		// Se a tag for required, adiciona
		if field.Tag.Get("required") == "true" {
			requiredFields = append(requiredFields, fieldName)
		}
	}

	// Atualiza os required fields
	schema["required"] = requiredFields

	return schema
}

// mapGoTypeToJSONType converte tipos Go para tipos JSON Schema.
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
