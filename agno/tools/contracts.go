package tools

// ToolType is the type of a tool call
type ToolType string

// ToolCall represents a tool call made by the model
type ToolCall struct {
	ID       string       `json:"id,omitempty"`
	Type     ToolType     `json:"type"`
	Function FunctionCall `json:"function,omitempty"`
}

// FunctionCall represents a function call within a ToolCall
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Tools represents the tool definition for OpenAI-compatible models
type Tools struct {
	Type     string              `json:"type"`
	Function *FunctionDefinition `json:"function,omitempty"`
}

// Parameters represents function parameters
type Parameters struct {
	Type                 string                 `json:"type"`
	Properties           map[string]interface{} `json:"properties,omitempty"`
	Required             []string               `json:"required,omitempty"`
	AdditionalProperties *bool                  `json:"additionalProperties,omitempty"`
}

// FunctionDefinition defines a callable function
type FunctionDefinition struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Parameters  Parameters `json:"parameters,omitempty"`
	Strict      bool       `json:"strict,omitempty"`
	Required    []string   `json:"required,omitempty"`
}
