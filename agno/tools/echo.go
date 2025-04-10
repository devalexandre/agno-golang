package tools

import (
	"encoding/json"
	"fmt"
)

// EchoTool implements the Tool interface for echoing messages
type EchoTool struct{}

// Description returns a short description of the tool
func (et EchoTool) Description() string {
	return "Echoes back the message provided"
}

// Name returns the name of the tool
func (et EchoTool) Name() string {
	return "EchoTool"
}

// GetParameterStruct returns the parameter structure for the echo tool
func (et EchoTool) GetParameterStruct() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"message": map[string]interface{}{
				"type":        "string",
				"description": "A message to echo back",
			},
		},
		"required": []string{"message"},
	}
}

// Execute echoes back the message
func (et EchoTool) Execute(input json.RawMessage) (interface{}, error) {
	var params map[string]interface{}
	err := json.Unmarshal(input, &params)
	if err != nil {
		return nil, err
	}

	// Verificar se temos o parâmetro message
	message, ok := params["message"].(string)
	if !ok {
		// Verificar se temos o parâmetro properties (formato alternativo usado pelo Gemini)
		message, ok = params["properties"].(string)
		if !ok {
			return nil, fmt.Errorf("message parameter is required")
		}
	}

	return "Echo: " + message, nil
}
