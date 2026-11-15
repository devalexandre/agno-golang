package component

import (
	"encoding/json"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/models"
)

// Requirement wire format
//
// RunResult.Requirements and ContinueRequest.Requirements carry
// models.RunRequirement in its JSON form (Python RunRequirement.to_dict):
//
//	{"id": "...", "pause_type": "confirmation|user_input|user_feedback|external_execution",
//	 "tool_execution": {"tool_call_id": "...", "tool_name": "...", "tool_args": {...},
//	                    "requires_confirmation": true, "user_input_schema": [...], ...},
//	 "user_input_schema": [{"name": "...", "field_type": "...", "description": "...", "value": null}],
//	 "user_feedback_schema": [{"question": "...", "options": [...], "selected_options": null}]}
//
// A caller resolves a requirement by filling in, on the copy it got back:
//
//	confirmation (true/false) and optionally confirmation_note     -> Confirm / Reject
//	user_input_schema[].value                                     -> ProvideUserInput
//	user_feedback_schema[].selected_options                       -> ProvideUserFeedback
//	external_execution_result                                     -> SetExternalExecutionResult
//
// The same fields nested under tool_execution are accepted too. Requirements
// are matched to the paused run by id or tool_execution.tool_call_id.

// EncodeRequirements converts requirements to their wire form.
func EncodeRequirements(reqs []*models.RunRequirement) []map[string]any {
	if len(reqs) == 0 {
		return nil
	}
	return toMaps(reqs)
}

// DecodeRequirements parses wire requirements into models.RunRequirement.
func DecodeRequirements(maps []map[string]any) ([]*models.RunRequirement, error) {
	if len(maps) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(maps)
	if err != nil {
		return nil, err
	}
	var out []*models.RunRequirement
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode requirements: %w", err)
	}
	return out, nil
}
