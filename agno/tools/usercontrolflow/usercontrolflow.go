// Package usercontrolflow ports Agno's UserControlFlowTools: the agent calls
// GetUserInput to pause the run and ask the user to fill in fields.
//
// In Python the agent loop intercepts get_user_input and pauses the run
// (human-in-the-loop "user input"). The Go agent's HITL user-input support is
// being added separately; until it lands, the method is registered with
// toolkit.WithStopAfterCall so the tool loop stops after the call, and the
// requested fields are available from PendingFields for the application to
// collect and send back in the next message.
package usercontrolflow

import (
	"sync"

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

	mu      sync.Mutex
	pending []UserInputField
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
		u.Toolkit.RegisterWithOptions("GetUserInput", "Use this tool to get user input for the given fields. Provide all the fields that you require the user to fill in, as if they were filling in a form.", u, u.GetUserInput, GetUserInputParams{}, toolkit.WithStopAfterCall())
	}
	return u
}

// GetUserInput records the requested fields; the run is paused by the agent.
func (u *UserControlFlowTools) GetUserInput(p GetUserInputParams) (interface{}, error) {
	u.mu.Lock()
	u.pending = append([]UserInputField(nil), p.UserInputFields...)
	u.mu.Unlock()
	return "User input received", nil
}

// PendingFields returns the fields requested by the last GetUserInput call.
func (u *UserControlFlowTools) PendingFields() []UserInputField {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]UserInputField(nil), u.pending...)
}

// ClearPending forgets the pending request (after the user answered).
func (u *UserControlFlowTools) ClearPending() {
	u.mu.Lock()
	u.pending = nil
	u.mu.Unlock()
}
