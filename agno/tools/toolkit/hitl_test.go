package toolkit

import "testing"

type hitlParams struct {
	To      string `json:"to" description:"Recipient" required:"true"`
	Subject string `json:"subject" description:"Subject" required:"true"`
}

type hitlRecv struct{}

func (hitlRecv) Do(p hitlParams) (interface{}, error) { return p.To, nil }

func newHITLKit(opts ...MethodOption) Method {
	tk := NewToolkit()
	tk.Name = "K"
	tk.RegisterWithOptions("Do", "do", hitlRecv{}, hitlRecv{}.Do, hitlParams{}, opts...)
	return tk.GetMethods()["K_Do"]
}

func TestWithUserInputHidesFields(t *testing.T) {
	m := newHITLKit(WithUserInput("to"))
	if !m.RequiresUserInput || !m.RequiresHITL() {
		t.Fatal("expected user input flag")
	}
	props := m.Schema["properties"].(map[string]interface{})
	if _, ok := props["to"]; ok {
		t.Fatal("user field must be hidden from the model schema")
	}
	if _, ok := props["subject"]; !ok {
		t.Fatal("other fields must remain")
	}
	for _, r := range m.Schema["required"].([]string) {
		if r == "to" {
			t.Fatal("user field must not be required from the model")
		}
	}
	if len(m.UserInputSchema) != 1 || m.UserInputSchema[0].Name != "to" || m.UserInputSchema[0].FieldType != "string" || m.UserInputSchema[0].Description != "Recipient" {
		t.Fatalf("bad user input schema: %+v", m.UserInputSchema)
	}

	all := newHITLKit(WithUserInput())
	if len(all.UserInputSchema) != 2 || len(all.Schema["properties"].(map[string]interface{})) != 0 {
		t.Fatalf("no fields means every field comes from the user: %+v", all.UserInputSchema)
	}
}

func TestApprovalAndStopFlags(t *testing.T) {
	if m := newHITLKit(WithApproval(ApprovalRequired)); !m.RequiresConfirmation || m.ApprovalType != ApprovalRequired {
		t.Fatal("required approval implies confirmation")
	}
	if m := newHITLKit(WithApproval(ApprovalAudit)); m.ApprovalType != "" {
		t.Fatal("audit approval without HITL flag is ignored")
	}
	if m := newHITLKit(WithExternalExecution(), WithApproval(ApprovalAudit)); m.ApprovalType != ApprovalAudit || m.RequiresConfirmation {
		t.Fatal("audit approval kept with external execution")
	}
	if m := newHITLKit(WithStopAfterCall()); !m.ShowResult {
		t.Fatal("stop after call implies show result")
	}
	if m := newHITLKit(WithExternalExecutionSilent()); !m.ExternalExecution || !m.ExternalExecutionSilent {
		t.Fatal("silent external execution")
	}
}
