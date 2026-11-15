package usercontrolflow

import "testing"

func TestGetUserInput(t *testing.T) {
	u := NewUserControlFlowTools()
	m, ok := u.GetMethods()["user_control_flow_tools_GetUserInput"]
	if !ok || !m.StopAfterCall {
		t.Fatal("GetUserInput must be registered with StopAfterCall")
	}
	out, err := u.Execute("user_control_flow_tools_GetUserInput", []byte(`{"user_input_fields":[{"field_name":"to","field_type":"str","field_description":"recipient"}]}`))
	if err != nil || out != "User input received" {
		t.Fatalf("%v %v", out, err)
	}
	if f := u.PendingFields(); len(f) != 1 || f[0].FieldName != "to" {
		t.Fatalf("pending %+v", f)
	}
	if len(NewUserControlFlowTools(WithGetUserInput(false)).GetMethods()) != 0 {
		t.Fatal("tool should be disabled")
	}
}
