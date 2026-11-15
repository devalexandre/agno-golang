package userfeedback_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/tools/userfeedback"
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

const method = "user_feedback_tools_AskUser"

func TestAskUserPausesAndContinues(t *testing.T) {
	u := userfeedback.NewUserFeedbackTools()
	if m := u.GetMethods()[method]; m.UserFeedbackBuilder == nil || m.StopAfterCall {
		t.Fatalf("AskUser must pause for user feedback: %+v", m)
	}
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context: context.Background(),
		Model: &stubModel{call: tools.FunctionCall{Name: method,
			Arguments: `{"questions":[{"question":"Where to?","header":"Dest","options":[{"label":"Paris","description":"France"},{"label":"Rome"}]},{"question":"Extras?","header":"Extras","multi_select":true,"options":[{"label":"Hotel"},{"label":"Car"}]}]}`}},
		Tools: []toolkit.Tool{u},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := ag.Run("plan a trip")
	if err != nil {
		t.Fatal(err)
	}
	if !resp.IsPaused() || len(resp.Requirements) != 1 {
		t.Fatalf("expected one paused requirement, got %s", resp.Status)
	}
	req := resp.Requirements[0]
	if req.PauseType() != models.PauseTypeUserFeedback || !req.NeedsUserFeedback() || req.NeedsUserInput() {
		t.Fatalf("bad pause type %s", req.PauseType())
	}
	qs := req.UserFeedbackSchema()
	if len(qs) != 2 || qs[0].Header != "Dest" || len(qs[0].Options) != 2 || qs[0].Options[0].Description != "France" || !qs[1].MultiSelect {
		t.Fatalf("bad questions: %+v", qs)
	}
	// The wire form carries the questions at the top level, like Python.
	b, _ := json.Marshal(req)
	var wire map[string]interface{}
	_ = json.Unmarshal(b, &wire)
	if wire["pause_type"] != "user_feedback" || wire["user_feedback_schema"] == nil {
		t.Fatalf("wire form: %s", b)
	}

	if err := req.ProvideUserFeedback(map[string][]string{"Where to?": {"Rome"}}); err != nil {
		t.Fatal(err)
	}
	if req.IsResolved() {
		t.Fatal("one question is still unanswered")
	}
	if err := req.ProvideUserFeedback(map[string][]string{"Extras?": {"Hotel", "Car"}}); err != nil {
		t.Fatal(err)
	}
	if !req.IsResolved() || !qs[0].Options[1].Selected || qs[0].Options[0].Selected {
		t.Fatalf("selection not applied: %+v", qs[0].Options)
	}
	out, err := ag.ContinueRun(context.Background(), &resp)
	if err != nil {
		t.Fatal(err)
	}
	want := `done: User feedback received: [{"question":"Where to?","selected":["Rome"]},{"question":"Extras?","selected":["Hotel","Car"]}]`
	if out.Status != models.RunStatusCompleted || out.TextContent != want {
		t.Fatalf("got %s %q", out.Status, out.TextContent)
	}
}
