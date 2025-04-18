package tools

type ToolType string

type ToolCall struct {
	ID       string       `json:"id,omitempty"`
	Type     ToolType     `json:"type"`
	Function FunctionCall `json:"function,omitempty"`
}

// ToolFunction is a function to be called in a tool choice.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Tool is a tool that can be used by the model.
type Tools struct {
	// Type is the type of the tool.
	Type string `json:"type"`
	// Function is the function to call.
	Function *FunctionDefinition `json:"function,omitempty"`
}

type Parameters struct {
	// Type is the type of the parameters.
	Type string `json:"type"`
	// Properties is a map of properties for the parameters.
	Properties Properties `json:"properties,omitempty"`
}
type Properties map[string]interface{}

// FunctionDefinition is a definition of a function that can be called by the model.
type FunctionDefinition struct {
	// Name is the name of the function.
	Name string `json:"name"`
	// Description is a description of the function.
	Description string `json:"description"`
	// Parameters is a list of parameters for the function.
	Parameters Parameters `json:"parameters,omitempty"`
	// Strict is a flag to indicate if the function should be called strictly. Only used for openai llm structured output.
	Strict   bool     `json:"strict,omitempty"`
	Required []string `json:"required,omitempty"`
}

// FunctionReference is a reference to a function.
type FunctionReference struct {
	// Name is the name of the function.
	Name string `json:"name"`
}

// FunctionCallBehavior is the behavior to use when calling functions.
type FunctionCallBehavior string

const (
	// FunctionCallBehaviorNone will not call any functions.
	FunctionCallBehaviorNone FunctionCallBehavior = "none"
	// FunctionCallBehaviorAuto will call functions automatically.
	FunctionCallBehaviorAuto FunctionCallBehavior = "auto"
)
