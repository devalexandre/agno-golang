package toolkit

import (
	"encoding/json"
	"reflect"
)

// Toolkit stores the tool information and its registered methods.
type Toolkit struct {
	Name        string
	Description string
	methods     map[string]Method
}

// Method stores the execution function and its parameter schema.
type Method struct {
	Receiver    interface{}
	Description string
	Function    interface{}
	Schema      map[string]interface{}
	ParamType   reflect.Type
}

// Tool is the interface that defines the basic operations for any tool.
type Tool interface {
	GetName() string                                                       // Returns the tool name
	GetDescription() string                                                // Returns the tool description
	GetParameterStruct(methodName string) map[string]interface{}           // Returns the JSON schema based on the registered method
	GetMethods() map[string]Method                                         // Returns the registered methods
	GetFunction(methodName string) interface{}                             // Returns the execution function
	GetDescriptionOfMethod(methodName string) string                       // Returns the description of a specific method
	Execute(methodName string, input json.RawMessage) (interface{}, error) // Executes the function
}
