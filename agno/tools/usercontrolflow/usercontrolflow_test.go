package usercontrolflow_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/tools/usercontrolflow"
)

// stubModel calls one tool on the user turn and echoes tool results after.
type stubModel struct{ call tools.FunctionCall }

func (m *stubModel) Invoke(ctx context.Context, msgs []models.Message, opts ...models.Option) (*models.MessageResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if last := msgs[len(msgs)-1]; last.Role == models.TypeToolRole {
		return &models.MessageResponse{Role: "assistant", Content: "done: " + last.Content}, nil
	}
	co := &models.CallOptions{}
	for _, o := range opts {
		o(co)
	}
	id := "call_1"
	content := "tool not found"
	for _, t := range co.ToolCall {
		if _, ok := t.GetMethods()[m.call.Name]; ok {
			res, err := t.Execute(m.call.Name, json.RawMessage(m.call.Arguments))
			content = "Error: "
			if err == nil {
				content, _ = res.(string)
			}
		}
	}
	next := append(append([]models.Message(nil), msgs...),
		models.Message{Role: models.TypeAssistantRole, ToolCalls: []tools.ToolCall{{ID: id, Type: "function", Function: m.call}}},
		models.Message{Role: models.TypeToolRole, Content: content, ToolCallID: &id})
	return m.Invoke(ctx, next, opts...)
}
func (m *stubModel) AInvoke(context.Context, []models.Message, ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return nil, nil
}
func (m *stubModel) InvokeStream(context.Context, []models.Message, ...models.Option) error {
	return nil
}
func (m *stubModel) AInvokeStream(context.Context, []models.Message, ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return nil, nil
}
func (m *stubModel) GetID() string { return "stub" }

const method = "user_control_flow_tools_GetUserInput"

func TestGetUserInputRegistration(t *testing.T) {
	u := usercontrolflow.NewUserControlFlowTools()
	m, ok := u.GetMethods()[method]
	if !ok || !m.RequiresUserInput || m.UserInputBuilder == nil || m.StopAfterCall {
		t.Fatalf("GetUserInput must pause for dynamic user input: %+v", m)
	}
	// The model still sees user_input_fields.
	props, _ := u.GetParameterStruct(method)["properties"].(map[string]interface{})
	if _, ok := props["user_input_fields"]; !ok {
		t.Fatalf("schema lost user_input_fields: %v", props)
	}
	if len(usercontrolflow.NewUserControlFlowTools(usercontrolflow.WithGetUserInput(false)).GetMethods()) != 0 {
		t.Fatal("tool should be disabled")
	}
}

func TestGetUserInputPausesAndContinues(t *testing.T) {
	u := usercontrolflow.NewUserControlFlowTools()
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context: context.Background(),
		Model: &stubModel{call: tools.FunctionCall{Name: method,
			Arguments: `{"user_input_fields":[{"field_name":"to","field_type":"str","field_description":"recipient"},{"field_name":"day","field_description":"meeting day"}]}`}},
		Tools: []toolkit.Tool{u},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := ag.Run("invite someone")
	if err != nil {
		t.Fatal(err)
	}
	if !resp.IsPaused() || len(resp.Requirements) != 1 {
		t.Fatalf("expected one paused requirement, got %s %+v", resp.Status, resp.Requirements)
	}
	req := resp.Requirements[0]
	if req.PauseType() != models.PauseTypeUserInput || !req.NeedsUserInput() {
		t.Fatalf("bad pause: %s", req.PauseType())
	}
	schema := req.UserInputSchema()
	if len(schema) != 2 || schema[0].Name != "to" || schema[0].FieldType != "str" || schema[0].Description != "recipient" || schema[1].FieldType != "str" {
		t.Fatalf("bad schema: %+v %+v", schema[0], schema[1])
	}
	if err := req.ProvideUserInput(map[string]interface{}{"to": "ana@example.com", "day": "monday"}); err != nil {
		t.Fatal(err)
	}
	out, err := ag.ContinueRun(context.Background(), &resp)
	if err != nil {
		t.Fatal(err)
	}
	want := `done: User inputs retrieved: [{"name":"to","value":"ana@example.com"},{"name":"day","value":"monday"}]`
	if out.Status != models.RunStatusCompleted || out.TextContent != want {
		t.Fatalf("got %s %q", out.Status, out.TextContent)
	}
}

func TestGetUserInputWithoutFieldsRuns(t *testing.T) {
	u := usercontrolflow.NewUserControlFlowTools()
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context: context.Background(),
		Model:   &stubModel{call: tools.FunctionCall{Name: method, Arguments: `{"user_input_fields":[]}`}},
		Tools:   []toolkit.Tool{u},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := ag.Run("hi")
	if err != nil || resp.IsPaused() || !strings.Contains(resp.TextContent, "User input received") {
		t.Fatalf("no fields should not pause: %v %s %q", err, resp.Status, resp.TextContent)
	}
}
