package models

import (
	"encoding/json"
	"testing"
)

// Resolutions given at the top level of the wire form reach tool_execution,
// as in Python's RunRequirement.from_dict.
func TestRunRequirementWireResolution(t *testing.T) {
	input := NewRunRequirement(&ToolExecution{ToolCallID: "c1", ToolName: "send", RequiresUserInput: true,
		UserInputSchema: []*UserInputField{{Name: "to", FieldType: "string"}}})
	feedback := NewRunRequirement(&ToolExecution{ToolCallID: "c2", ToolName: "ask", RequiresUserInput: true,
		UserFeedbackSchema: []*UserFeedbackQuestion{{Question: "Where?", Options: []*UserFeedbackOption{{Label: "Rome"}, {Label: "Paris"}}}}})
	confirm := NewRunRequirement(&ToolExecution{ToolCallID: "c3", ToolName: "rm", RequiresConfirmation: true})

	var wire []map[string]interface{}
	b, _ := json.Marshal([]*RunRequirement{input, feedback, confirm})
	if err := json.Unmarshal(b, &wire); err != nil {
		t.Fatal(err)
	}
	if wire[0]["pause_type"] != "user_input" || wire[1]["pause_type"] != "user_feedback" || wire[2]["pause_type"] != "confirmation" {
		t.Fatalf("pause types: %v", wire)
	}
	wire[0]["user_input_schema"].([]interface{})[0].(map[string]interface{})["value"] = "ana"
	wire[1]["user_feedback_schema"].([]interface{})[0].(map[string]interface{})["selected_options"] = []string{"Rome"}
	wire[2]["confirmation"] = false
	wire[2]["confirmation_note"] = "no"

	b, _ = json.Marshal(wire)
	var got []*RunRequirement
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	for i, r := range got {
		if !r.IsResolved() {
			t.Fatalf("requirement %d not resolved: %+v", i, r.ToolExecution)
		}
	}
	if got[0].ToolExecution.UserInputSchema[0].Value != "ana" {
		t.Fatal("user input value lost")
	}
	if q := got[1].ToolExecution.UserFeedbackSchema[0]; q.SelectedOptions[0] != "Rome" || !q.Options[0].Selected || q.Options[1].Selected {
		t.Fatalf("feedback: %+v", q)
	}
	if te := got[2].ToolExecution; te.Confirmed == nil || *te.Confirmed || te.ConfirmationNote != "no" {
		t.Fatalf("confirmation: %+v", te)
	}
}
