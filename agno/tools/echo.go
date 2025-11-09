package tools

import (
	"encoding/json"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// EchoTool implements the Tool interface for echoing messages
type EchoTool struct {
	toolkit.Toolkit
}

// EchoParams represents parameters for the echo tool
type EchoParams struct {
	Message string `json:"message" description:"A message to echo back" required:"true"`
}

// NewEchoTool creates a new EchoTool instance
func NewEchoTool() *EchoTool {
	et := &EchoTool{}
	et.Toolkit = toolkit.NewToolkit()
	et.Toolkit.Name = "EchoTool"
	et.Toolkit.Description = "Echoes back the message provided"

	// Register methods
	et.Toolkit.Register("Echo", "Echoes back the message provided", et, et.Echo, EchoParams{})

	return et
}

// Echo echoes back the message
func (et *EchoTool) Echo(params EchoParams) (interface{}, error) {
	if params.Message == "" {
		return nil, fmt.Errorf("message parameter is required")
	}

	return "Echo: " + params.Message, nil
}

// Implement toolkit.Tool interface methods (delegated to Toolkit)

func (et *EchoTool) GetName() string {
	return et.Toolkit.GetName()
}

func (et *EchoTool) GetDescription() string {
	return et.Toolkit.GetDescription()
}

func (et *EchoTool) GetParameterStruct(methodName string) map[string]interface{} {
	return et.Toolkit.GetParameterStruct(et.Toolkit.GetName() + "_" + methodName)
}

func (et *EchoTool) GetMethods() map[string]toolkit.Method {
	return et.Toolkit.GetMethods()
}

func (et *EchoTool) GetFunction(methodName string) interface{} {
	return et.Toolkit.GetFunction(et.Toolkit.GetName() + "_" + methodName)
}

func (et *EchoTool) GetDescriptionOfMethod(methodName string) string {
	return et.Toolkit.GetDescriptionOfMethod(et.Toolkit.GetName() + "_" + methodName)
}

func (et *EchoTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	fullMethodName := et.Toolkit.GetName() + "_" + methodName
	return et.Toolkit.Execute(fullMethodName, input)
}
