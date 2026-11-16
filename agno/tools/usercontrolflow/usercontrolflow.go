// Package usercontrolflow ports Agno's UserControlFlowTools: the agent calls
// GetUserInput to pause the run and ask the user to fill in fields.
//
// As in Python, the call does not run the tool: it pauses the agent run
// (human-in-the-loop "user input"). The paused RunResponse carries one
// RunRequirement whose UserInputSchema lists the fields the model asked for;
// fill them with RunRequirement.ProvideUserInput and resume with
// Agent.ContinueRun. The model then receives the values as the tool result
// ("User inputs retrieved: [...]").
package usercontrolflow

import (
	"fmt"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// UserInputField describes one field the agent wants the user to fill in.
type UserInputField struct {
	FieldName        string      `json:"field_name"`
	FieldType        string      `json:"field_type"`
	FieldDescription string      `json:"field_description"`
	Value            interface{} `json:"value,omitempty"`
}

// UserControlFlowTools provides GetUserInput.
type UserControlFlowTools struct {
	toolkit.Toolkit
	Instructions    string
	AddInstructions bool

	enGetUserInput bool
	custom         *string
}

// Option configures UserControlFlowTools.
type Option func(*UserControlFlowTools)

// WithGetUserInput enables or disables GetUserInput (default true).
func WithGetUserInput(b bool) Option { return func(u *UserControlFlowTools) { u.enGetUserInput = b } }

// WithAll enables every tool.
func WithAll() Option { return func(u *UserControlFlowTools) { u.enGetUserInput = true } }

// WithInstructions replaces the default instructions.
func WithInstructions(s string) Option { return func(u *UserControlFlowTools) { u.custom = &s } }

// WithAddInstructions controls whether instructions are added to the prompt (default true).
func WithAddInstructions(b bool) Option {
	return func(u *UserControlFlowTools) { u.AddInstructions = b }
}

// GetUserInputParams are the parameters of GetUserInput.
type GetUserInputParams struct {
	UserInputFields []UserInputField `json:"user_input_fields" description:"A list of objects, each with keys: field_name (the name of the field to get input for), field_type (the type of the field: str, int, float, bool, list, dict, ...), field_description (a description of the field)." required:"true"`
}

// NewUserControlFlowTools creates the toolkit.
func NewUserControlFlowTools(opts ...Option) *UserControlFlowTools {
	u := &UserControlFlowTools{enGetUserInput: true, AddInstructions: true}
	for _, o := range opts {
		o(u)
	}
	u.Instructions = DefaultInstructions
	if u.custom != nil {
		u.Instructions = *u.custom
	}
	u.Toolkit = toolkit.NewToolkit()
	u.Toolkit.Name = "user_control_flow_tools"
	u.Toolkit.Description = "Interrupt the run to ask the user for input."
	if u.enGetUserInput {
		u.Toolkit.RegisterWithOptions("GetUserInput", "Use this tool to get user input for the given fields. Provide all the fields that you require the user to fill in, as if they were filling in a form.", u, u.GetUserInput, GetUserInputParams{}, toolkit.WithUserInputFromArgs(FieldsFromArgs))
	}
	return u
}

// FieldsFromArgs turns GetUserInput's arguments into the user-input fields
// of the paused run (Python's get_user_input handling in Model).
func FieldsFromArgs(args map[string]interface{}) []toolkit.UserInputFieldSpec {
	raw, _ := args["user_input_fields"].([]interface{})
	var out []toolkit.UserInputFieldSpec
	for _, item := range raw {
		f, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name := str(f["field_name"])
		if name == "" {
			continue
		}
		ft := str(f["field_type"])
		if ft == "" {
			ft = "str"
		}
		out = append(out, toolkit.UserInputFieldSpec{Name: name, FieldType: ft, Description: str(f["field_description"])})
	}
	return out
}

func str(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

// GetUserInput is only executed when the call asks for no fields (or outside
// an agent run); otherwise the agent pauses the run instead of calling it.
func (u *UserControlFlowTools) GetUserInput(p GetUserInputParams) (interface{}, error) {
	return "User input received", nil
}
