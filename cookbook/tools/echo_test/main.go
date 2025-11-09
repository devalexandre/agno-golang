package main

import (
	"encoding/json"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/tools"
)

func main() {
	// Test EchoTool
	echoTool := tools.NewEchoTool()

	// Test GetMethods
	methods := echoTool.GetMethods()
	fmt.Printf("Methods: %+v\n", methods)

	// Test GetName
	fmt.Printf("Tool Name: %s\n", echoTool.GetName())

	// Test GetDescription
	fmt.Printf("Tool Description: %s\n", echoTool.GetDescription())

	// Test Execute
	params := map[string]interface{}{
		"message": "Hello, World!",
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := echoTool.Execute("Echo", paramsJSON)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result: %v\n", result)
	}

	// Test GetParameterStruct
	schema := echoTool.GetParameterStruct("Echo")
	fmt.Printf("Schema: %+v\n", schema)
}
