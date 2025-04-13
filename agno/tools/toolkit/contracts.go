package toolkit

import (
	"encoding/json"
	"reflect"
)

// Toolkit armazena as informações da ferramenta e seus métodos registrados.
type Toolkit struct {
	Name        string
	Description string
	methods     map[string]Method
}

// Method armazena a função de execução e seu schema de parâmetros.
type Method struct {
	Receiver  interface{}
	Function  interface{}
	Schema    map[string]interface{}
	ParamType reflect.Type
}

// Tool é a interface que define as operações básicas para qualquer ferramenta.
type Tool interface {
	GetName() string                                                       // Retorna o nome da ferramenta
	GetDescription() string                                                // Retorna a descrição da ferramenta
	GetParameterStruct(methodName string) map[string]interface{}           // Retorna o schema JSON baseado no método registrado
	GetMethods() map[string]Method                                         // Retorna os métodos registrados
	GetFunction(methodName string) interface{}                             // Retorna a função de execução
	Execute(methodName string, input json.RawMessage) (interface{}, error) // Executa a função
}
