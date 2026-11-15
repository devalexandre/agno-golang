package toolkit

import (
	"log"
	"sort"
)

// Approval types for WithApproval (mirror Python agno.approval.ApprovalType).
const (
	// ApprovalRequired creates a blocking approval record when the run pauses;
	// the run cannot continue until the record is resolved.
	ApprovalRequired = "required"
	// ApprovalAudit creates a resolved audit record after the HITL interaction.
	ApprovalAudit = "audit"
)

// UserInputFieldSpec describes a parameter the user must provide.
type UserInputFieldSpec struct {
	Name        string
	FieldType   string
	Description string
}

// WithUserInput pauses the run until the user provides the given parameters
// (all parameters when none are given). Those parameters are hidden from the
// model. Mirrors Python's requires_user_input / user_input_fields.
func WithUserInput(fields ...string) MethodOption {
	return func(m *Method) {
		m.RequiresUserInput = true
		m.UserInputFields = append(m.UserInputFields, fields...)
	}
}

// UserFeedbackOptionSpec is an option of a user-feedback question.
type UserFeedbackOptionSpec struct {
	Label       string
	Description string
}

// UserFeedbackQuestionSpec is a structured question with predefined options.
type UserFeedbackQuestionSpec struct {
	Question    string
	Header      string
	Options     []UserFeedbackOptionSpec
	MultiSelect bool
}

// UserInputBuilder derives the user-input fields from the call arguments.
type UserInputBuilder func(args map[string]interface{}) []UserInputFieldSpec

// UserFeedbackBuilder derives the feedback questions from the call arguments.
type UserFeedbackBuilder func(args map[string]interface{}) []UserFeedbackQuestionSpec

// WithUserInputFromArgs pauses the run for user input whose fields are
// chosen by the model at call time (Python's get_user_input). build turns the
// call arguments into the fields to collect; when it returns none the call
// runs normally. The model-facing schema is left untouched. On ContinueRun
// the tool is not executed: the model receives the collected values.
func WithUserInputFromArgs(build UserInputBuilder) MethodOption {
	return func(m *Method) {
		m.RequiresUserInput = true
		m.UserInputBuilder = build
	}
}

// WithUserFeedback pauses the run with structured questions built from the
// call arguments (Python's ask_user). When build returns none the call runs
// normally. On ContinueRun the tool is not executed: the model receives the
// selected options.
func WithUserFeedback(build UserFeedbackBuilder) MethodOption {
	return func(m *Method) {
		m.RequiresUserInput = true
		m.UserFeedbackBuilder = build
	}
}

// IsDynamicUserInput reports whether the method's pause is built from the
// call arguments (WithUserInputFromArgs or WithUserFeedback).
func (m Method) IsDynamicUserInput() bool {
	return m.UserInputBuilder != nil || m.UserFeedbackBuilder != nil
}

// WithExternalExecution marks the method as executed outside the agent: the
// run pauses and the caller supplies the result through ContinueRun.
func WithExternalExecution() MethodOption {
	return func(m *Method) {
		m.ExternalExecution = true
	}
}

// WithExternalExecutionSilent is WithExternalExecution without the verbose
// paused message.
func WithExternalExecutionSilent() MethodOption {
	return func(m *Method) {
		m.ExternalExecution = true
		m.ExternalExecutionSilent = true
	}
}

// WithShowResult includes the method result in the run content.
func WithShowResult() MethodOption {
	return func(m *Method) {
		m.ShowResult = true
	}
}

// WithApproval attaches an approval type (ApprovalRequired or ApprovalAudit).
// "required" implies confirmation when no other HITL flag is set; "audit"
// needs a HITL flag (confirmation, user input or external execution) and is
// ignored otherwise.
func WithApproval(approvalType string) MethodOption {
	return func(m *Method) {
		m.ApprovalType = approvalType
	}
}

// RequiresHITL reports whether calling the method pauses the run.
func (m Method) RequiresHITL() bool {
	return m.RequiresConfirmation || m.RequiresUserInput || m.ExternalExecution
}

// finalizeMethod validates flag combinations and derives the user input
// schema after all options were applied.
func finalizeMethod(name string, m *Method) {
	if m.StopAfterCall {
		// Python: stop_after_tool_call implies show_result.
		m.ShowResult = true
	}
	switch m.ApprovalType {
	case "":
	case ApprovalRequired:
		if !m.RequiresHITL() {
			m.RequiresConfirmation = true
		}
	case ApprovalAudit:
		if !m.RequiresHITL() {
			log.Printf("toolkit: %s: approval type 'audit' needs a HITL flag (confirmation, user input or external execution); ignoring", name)
			m.ApprovalType = ""
		}
	default:
		log.Printf("toolkit: %s: invalid approval type %q; ignoring", name, m.ApprovalType)
		m.ApprovalType = ""
	}
	if m.RequiresUserInput && !m.IsDynamicUserInput() {
		m.UserInputSchema, m.Schema = splitUserInputSchema(m.Schema, m.UserInputFields)
	}
}

// splitUserInputSchema extracts the user-provided fields from the schema and
// returns the model-facing schema without them.
func splitUserInputSchema(schema map[string]interface{}, fields []string) ([]UserInputFieldSpec, map[string]interface{}) {
	props, _ := schema["properties"].(map[string]interface{})
	if len(fields) == 0 {
		for name := range props {
			fields = append(fields, name)
		}
		sort.Strings(fields)
	}
	userSet := make(map[string]bool, len(fields))
	specs := make([]UserInputFieldSpec, 0, len(fields))
	for _, name := range fields {
		userSet[name] = true
		spec := UserInputFieldSpec{Name: name}
		if p, ok := props[name].(map[string]interface{}); ok {
			spec.FieldType, _ = p["type"].(string)
			spec.Description, _ = p["description"].(string)
		}
		specs = append(specs, spec)
	}

	out := make(map[string]interface{}, len(schema))
	for k, v := range schema {
		out[k] = v
	}
	newProps := make(map[string]interface{}, len(props))
	for k, v := range props {
		if !userSet[k] {
			newProps[k] = v
		}
	}
	out["properties"] = newProps
	switch req := schema["required"].(type) {
	case []string:
		kept := []string{}
		for _, r := range req {
			if !userSet[r] {
				kept = append(kept, r)
			}
		}
		out["required"] = kept
	case []interface{}:
		kept := []interface{}{}
		for _, r := range req {
			if s, ok := r.(string); !ok || !userSet[s] {
				kept = append(kept, r)
			}
		}
		out["required"] = kept
	}
	return specs, out
}
