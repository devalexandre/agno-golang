package models

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// This file holds the human-in-the-loop (HITL) types attached to a paused
// RunResponse. They mirror Python's agno.models.response.ToolExecution,
// agno.tools.function.UserInputField and agno.run.requirement.RunRequirement.

// PauseType identifies why a tool call paused the run.
type PauseType string

const (
	PauseTypeConfirmation      PauseType = "confirmation"
	PauseTypeUserInput         PauseType = "user_input"
	PauseTypeUserFeedback      PauseType = "user_feedback"
	PauseTypeExternalExecution PauseType = "external_execution"
)

// RunEventPaused is the Event value of a RunResponse returned for a paused run.
const RunEventPaused = "RunPaused"

// UserInputField is a tool argument that must be provided by the user before
// the tool runs.
type UserInputField struct {
	Name        string      `json:"name"`
	FieldType   string      `json:"field_type,omitempty"`
	Description string      `json:"description,omitempty"`
	Value       interface{} `json:"value,omitempty"`
}

// UserFeedbackOption is an option of a user-feedback question.
type UserFeedbackOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Selected    bool   `json:"selected,omitempty"`
}

// UserFeedbackQuestion is a structured question with predefined options
// (Python agno.tools.function.UserFeedbackQuestion). SelectedOptions is nil
// until the user answers; an empty slice is a valid "nothing selected" answer.
type UserFeedbackQuestion struct {
	Question        string                `json:"question"`
	Header          string                `json:"header,omitempty"`
	Options         []*UserFeedbackOption `json:"options,omitempty"`
	MultiSelect     bool                  `json:"multi_select,omitempty"`
	SelectedOptions []string              `json:"selected_options"`
}

// ToolExecution describes one tool call of a run, including its HITL state.
type ToolExecution struct {
	ToolCallID    string                 `json:"tool_call_id,omitempty"`
	ToolName      string                 `json:"tool_name,omitempty"`
	ToolArgs      map[string]interface{} `json:"tool_args,omitempty"`
	ToolCallError bool                   `json:"tool_call_error,omitempty"`
	Result        *string                `json:"result,omitempty"`
	// StopAfterToolCall reports that the run stopped after this call.
	StopAfterToolCall bool  `json:"stop_after_tool_call,omitempty"`
	CreatedAt         int64 `json:"created_at,omitempty"`

	RequiresConfirmation bool   `json:"requires_confirmation,omitempty"`
	Confirmed            *bool  `json:"confirmed,omitempty"`
	ConfirmationNote     string `json:"confirmation_note,omitempty"`

	RequiresUserInput bool              `json:"requires_user_input,omitempty"`
	UserInputSchema   []*UserInputField `json:"user_input_schema,omitempty"`
	// UserFeedbackSchema holds the questions of a user-feedback pause
	// (UserFeedbackTools.AskUser).
	UserFeedbackSchema []*UserFeedbackQuestion `json:"user_feedback_schema,omitempty"`
	Answered           bool                    `json:"answered,omitempty"`

	ExternalExecutionRequired bool `json:"external_execution_required,omitempty"`
	ExternalExecutionSilent   bool `json:"external_execution_silent,omitempty"`

	// ApprovalType is "required" (blocking) or "audit" (non-blocking audit trail).
	ApprovalType string `json:"approval_type,omitempty"`
	// ApprovalID is the approval record created when the run paused.
	ApprovalID string `json:"approval_id,omitempty"`
}

// IsPaused reports whether the call needs a human before it can run.
func (t *ToolExecution) IsPaused() bool {
	return t != nil && (t.RequiresConfirmation || t.RequiresUserInput || t.ExternalExecutionRequired)
}

// PauseType returns the most specific reason the call paused
// (user feedback > external > user input > confirmation, as in Python).
func (t *ToolExecution) PauseType() PauseType {
	switch {
	case t == nil:
		return PauseTypeConfirmation
	case len(t.UserFeedbackSchema) > 0:
		return PauseTypeUserFeedback
	case t.ExternalExecutionRequired:
		return PauseTypeExternalExecution
	case t.RequiresUserInput:
		return PauseTypeUserInput
	default:
		return PauseTypeConfirmation
	}
}

// ResultString returns the result or "" when there is none.
func (t *ToolExecution) ResultString() string {
	if t == nil || t.Result == nil {
		return ""
	}
	return *t.Result
}

// RunRequirement is something a human must resolve before a paused run can
// continue: confirm/reject a call, provide user input, or execute it externally.
type RunRequirement struct {
	ID            string         `json:"id"`
	ToolExecution *ToolExecution `json:"tool_execution,omitempty"`
	CreatedAt     int64          `json:"created_at,omitempty"`

	Confirmation     *bool  `json:"confirmation,omitempty"`
	ConfirmationNote string `json:"confirmation_note,omitempty"`

	ExternalExecutionResult *string `json:"external_execution_result,omitempty"`

	// Member context, set when the requirement comes from a team member.
	MemberAgentID   string `json:"member_agent_id,omitempty"`
	MemberAgentName string `json:"member_agent_name,omitempty"`
	MemberRunID     string `json:"member_run_id,omitempty"`
}

// NewRunRequirement creates a requirement for a paused tool call.
func NewRunRequirement(te *ToolExecution) *RunRequirement {
	return &RunRequirement{ID: uuid.New().String(), ToolExecution: te, CreatedAt: time.Now().Unix()}
}

// ErrRequirementMismatch is returned when a resolution does not apply to a requirement.
var ErrRequirementMismatch = errors.New("requirement does not need this resolution")

// UserInputSchema returns the fields the user must fill in (nil when none).
func (r *RunRequirement) UserInputSchema() []*UserInputField {
	if r == nil || r.ToolExecution == nil {
		return nil
	}
	return r.ToolExecution.UserInputSchema
}

// NeedsConfirmation reports whether the requirement still waits for a confirmation.
func (r *RunRequirement) NeedsConfirmation() bool {
	if r == nil || r.Confirmation != nil || r.ToolExecution == nil || r.ToolExecution.Confirmed != nil {
		return false
	}
	return r.ToolExecution.RequiresConfirmation
}

// NeedsUserInput reports whether user input is still missing.
func (r *RunRequirement) NeedsUserInput() bool {
	if r == nil || r.ToolExecution == nil || r.ToolExecution.Answered {
		return false
	}
	if !r.ToolExecution.RequiresUserInput || len(r.ToolExecution.UserFeedbackSchema) > 0 {
		// Feedback questions are answered with ProvideUserFeedback.
		return false
	}
	for _, f := range r.ToolExecution.UserInputSchema {
		if f.Value == nil {
			return true
		}
	}
	// No schema (or all values present) but not marked answered yet.
	return len(r.ToolExecution.UserInputSchema) == 0
}

// UserFeedbackSchema returns the questions the user must answer (nil when none).
func (r *RunRequirement) UserFeedbackSchema() []*UserFeedbackQuestion {
	if r == nil || r.ToolExecution == nil {
		return nil
	}
	return r.ToolExecution.UserFeedbackSchema
}

// NeedsUserFeedback reports whether feedback questions are still unanswered.
func (r *RunRequirement) NeedsUserFeedback() bool {
	if r == nil || r.ToolExecution == nil || r.ToolExecution.Answered {
		return false
	}
	for _, q := range r.ToolExecution.UserFeedbackSchema {
		if q.SelectedOptions == nil {
			return true
		}
	}
	return false
}

// NeedsExternalExecution reports whether the external result is still missing.
func (r *RunRequirement) NeedsExternalExecution() bool {
	if r == nil || r.ToolExecution == nil || r.ExternalExecutionResult != nil {
		return false
	}
	return r.ToolExecution.ExternalExecutionRequired && r.ToolExecution.Result == nil
}

// Confirm approves the tool call.
func (r *RunRequirement) Confirm() error {
	if !r.NeedsConfirmation() {
		return ErrRequirementMismatch
	}
	yes := true
	r.Confirmation = &yes
	r.ToolExecution.Confirmed = &yes
	return nil
}

// Reject denies the tool call; note is sent to the model as the call's error.
func (r *RunRequirement) Reject(note string) error {
	if !r.NeedsConfirmation() {
		return ErrRequirementMismatch
	}
	no := false
	r.Confirmation = &no
	r.ConfirmationNote = note
	r.ToolExecution.Confirmed = &no
	r.ToolExecution.ConfirmationNote = note
	return nil
}

// ProvideUserInput fills user-input fields by name. The requirement is
// answered once every field has a value.
func (r *RunRequirement) ProvideUserInput(values map[string]interface{}) error {
	if !r.NeedsUserInput() {
		return ErrRequirementMismatch
	}
	te := r.ToolExecution
	for _, f := range te.UserInputSchema {
		if v, ok := values[f.Name]; ok {
			f.Value = v
		}
	}
	if len(te.UserInputSchema) == 0 {
		// Free-form input: merge everything into the arguments.
		if te.ToolArgs == nil {
			te.ToolArgs = map[string]interface{}{}
		}
		for k, v := range values {
			te.ToolArgs[k] = v
		}
		te.Answered = true
		return nil
	}
	for _, f := range te.UserInputSchema {
		if f.Value == nil {
			return nil
		}
	}
	te.Answered = true
	return nil
}

// ProvideUserFeedback answers feedback questions: selections maps the
// question text to the selected option labels. The requirement is answered
// once every question has a selection (an empty list means "none").
func (r *RunRequirement) ProvideUserFeedback(selections map[string][]string) error {
	if !r.NeedsUserFeedback() {
		return ErrRequirementMismatch
	}
	te := r.ToolExecution
	for _, q := range te.UserFeedbackSchema {
		sel, ok := selections[q.Question]
		if !ok {
			continue
		}
		if sel == nil {
			sel = []string{}
		}
		q.SelectedOptions = append([]string{}, sel...)
		for _, opt := range q.Options {
			opt.Selected = false
			for _, l := range sel {
				if opt.Label == l {
					opt.Selected = true
				}
			}
		}
	}
	for _, q := range te.UserFeedbackSchema {
		if q.SelectedOptions == nil {
			return nil
		}
	}
	te.Answered = true
	return nil
}

// SetExternalExecutionResult records the result of a call executed outside the agent.
func (r *RunRequirement) SetExternalExecutionResult(result string) error {
	if !r.NeedsExternalExecution() {
		return ErrRequirementMismatch
	}
	r.ExternalExecutionResult = &result
	r.ToolExecution.Result = &result
	return nil
}

// IsResolved reports whether nothing is left to do for this requirement.
func (r *RunRequirement) IsResolved() bool {
	return !r.NeedsConfirmation() && !r.NeedsUserInput() && !r.NeedsUserFeedback() && !r.NeedsExternalExecution()
}

// PauseType returns why the requirement paused the run.
func (r *RunRequirement) PauseType() PauseType {
	if r == nil {
		return PauseTypeConfirmation
	}
	return r.ToolExecution.PauseType()
}

// requirementJSON is the wire form of RunRequirement. Like Python's
// RunRequirement.to_dict it also carries the user input / feedback schemas at
// the top level, so clients can fill them in without digging into
// tool_execution.
type requirementJSON struct {
	*requirementAlias
	PauseType          PauseType               `json:"pause_type,omitempty"`
	UserInputSchema    []*UserInputField       `json:"user_input_schema,omitempty"`
	UserFeedbackSchema []*UserFeedbackQuestion `json:"user_feedback_schema,omitempty"`
}

type requirementAlias RunRequirement

// MarshalJSON adds pause_type and the top-level schemas.
func (r RunRequirement) MarshalJSON() ([]byte, error) {
	out := requirementJSON{requirementAlias: (*requirementAlias)(&r)}
	if r.ToolExecution != nil {
		out.PauseType = r.ToolExecution.PauseType()
		out.UserInputSchema = r.ToolExecution.UserInputSchema
		out.UserFeedbackSchema = r.ToolExecution.UserFeedbackSchema
	}
	return json.Marshal(out)
}

// UnmarshalJSON accepts resolutions given at the top level (confirmation,
// external_execution_result, user_input_schema values, user_feedback_schema
// selections) and pushes them onto tool_execution, as Python's
// RunRequirement.from_dict does. Values already set on tool_execution win.
func (r *RunRequirement) UnmarshalJSON(b []byte) error {
	in := requirementJSON{requirementAlias: (*requirementAlias)(r)}
	if err := json.Unmarshal(b, &in); err != nil {
		return err
	}
	te := r.ToolExecution
	if te == nil {
		return nil
	}
	if r.Confirmation != nil && te.Confirmed == nil {
		c := *r.Confirmation
		te.Confirmed = &c
		if !c && te.ConfirmationNote == "" {
			te.ConfirmationNote = r.ConfirmationNote
		}
	}
	if r.ExternalExecutionResult != nil && te.Result == nil {
		res := *r.ExternalExecutionResult
		te.Result = &res
	}
	for _, top := range in.UserInputSchema {
		if top == nil || top.Value == nil {
			continue
		}
		for _, f := range te.UserInputSchema {
			if f.Name == top.Name && f.Value == nil {
				f.Value = top.Value
			}
		}
	}
	for _, top := range in.UserFeedbackSchema {
		if top == nil || top.SelectedOptions == nil {
			continue
		}
		for _, q := range te.UserFeedbackSchema {
			if q.Question == top.Question && q.SelectedOptions == nil {
				q.SelectedOptions = top.SelectedOptions
				for _, opt := range q.Options {
					for _, l := range top.SelectedOptions {
						if opt.Label == l {
							opt.Selected = true
						}
					}
				}
			}
		}
	}
	return nil
}

// IsPaused reports whether the run is paused waiting on requirements.
func (r *RunResponse) IsPaused() bool { return r.Status == RunStatusPaused }

// ActiveRequirements returns the requirements that are not resolved yet.
func (r *RunResponse) ActiveRequirements() []*RunRequirement {
	var out []*RunRequirement
	for _, req := range r.Requirements {
		if !req.IsResolved() {
			out = append(out, req)
		}
	}
	return out
}

// RequirementsNeedingConfirmation returns unresolved confirmation requirements.
func (r *RunResponse) RequirementsNeedingConfirmation() []*RunRequirement {
	var out []*RunRequirement
	for _, req := range r.Requirements {
		if req.NeedsConfirmation() {
			out = append(out, req)
		}
	}
	return out
}

// RequirementsNeedingUserInput returns unresolved user-input requirements.
func (r *RunResponse) RequirementsNeedingUserInput() []*RunRequirement {
	var out []*RunRequirement
	for _, req := range r.Requirements {
		if req.NeedsUserInput() {
			out = append(out, req)
		}
	}
	return out
}

// RequirementsNeedingUserFeedback returns unresolved user-feedback requirements.
func (r *RunResponse) RequirementsNeedingUserFeedback() []*RunRequirement {
	var out []*RunRequirement
	for _, req := range r.Requirements {
		if req.NeedsUserFeedback() {
			out = append(out, req)
		}
	}
	return out
}

// RequirementsNeedingExternalExecution returns unresolved external-execution requirements.
func (r *RunResponse) RequirementsNeedingExternalExecution() []*RunRequirement {
	var out []*RunRequirement
	for _, req := range r.Requirements {
		if req.NeedsExternalExecution() {
			out = append(out, req)
		}
	}
	return out
}
