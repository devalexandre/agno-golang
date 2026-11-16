package team

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// memberModel calls tools like the real clients when a call is planned;
// otherwise it answers with a fixed text (also used as the leader model).
type memberModel struct {
	planned *tools.FunctionCall
	answer  string
}

func (m *memberModel) Invoke(ctx context.Context, msgs []models.Message, opts ...models.Option) (*models.MessageResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	co := &models.CallOptions{}
	for _, o := range opts {
		o(co)
	}
	last := msgs[len(msgs)-1]
	if last.Role == models.TypeToolRole {
		return &models.MessageResponse{Role: "assistant", Content: m.answer + " (" + last.Content + ")"}, nil
	}
	if m.planned == nil {
		return &models.MessageResponse{Role: "assistant", Content: m.answer + ": " + last.Content}, nil
	}
	id := "call_1"
	content := ""
	for _, t := range co.ToolCall {
		if _, ok := t.GetMethods()[m.planned.Name]; ok {
			res, err := t.Execute(m.planned.Name, json.RawMessage(m.planned.Arguments))
			content = fmt.Sprint(res, err)
		}
	}
	next := append(append([]models.Message(nil), msgs...),
		models.Message{Role: models.TypeAssistantRole, ToolCalls: []tools.ToolCall{{ID: id, Type: "function", Function: *m.planned}}},
		models.Message{Role: models.TypeToolRole, Content: content, ToolCallID: &id})
	return m.Invoke(ctx, next, opts...)
}
func (m *memberModel) AInvoke(context.Context, []models.Message, ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return nil, nil
}
func (m *memberModel) InvokeStream(context.Context, []models.Message, ...models.Option) error {
	return nil
}
func (m *memberModel) AInvokeStream(context.Context, []models.Message, ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return nil, nil
}
func (m *memberModel) GetID() string { return "fake" }

type refundParams struct {
	Order string `json:"order" description:"Order id" required:"true"`
}

type refundTool struct {
	toolkit.Toolkit
	refunded []string
}

func (r *refundTool) Refund(p refundParams) (interface{}, error) {
	r.refunded = append(r.refunded, p.Order)
	return "refunded " + p.Order, nil
}

func TestTeamPausePropagatesAndContinues(t *testing.T) {
	rt := &refundTool{Toolkit: toolkit.NewToolkit()}
	rt.Name = "Billing"
	rt.Description = "billing"
	rt.RegisterWithOptions("Refund", "Refund an order", rt, rt.Refund, refundParams{}, toolkit.WithConfirmation())

	billing, err := agent.NewAgent(agent.AgentConfig{
		Name:  "billing",
		Model: &memberModel{planned: &tools.FunctionCall{Name: "Billing_Refund", Arguments: `{"order":"42"}`}, answer: "billing done"},
		Tools: []toolkit.Tool{rt},
	})
	if err != nil {
		t.Fatal(err)
	}
	support, err := agent.NewAgent(agent.AgentConfig{Name: "support", Model: &memberModel{answer: "support"}})
	if err != nil {
		t.Fatal(err)
	}

	tm := NewTeam(TeamConfig{
		Context: context.Background(),
		Name:    "desk",
		Model:   &memberModel{answer: "synthesis"},
		Members: []*agent.Agent{billing, support},
		Mode:    CoordinateMode,
	})

	resp, err := tm.Run("refund order 42")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != models.RunStatusPaused || len(resp.Requirements) != 1 {
		t.Fatalf("team should pause with the member requirement: %s %d", resp.Status, len(resp.Requirements))
	}
	req := resp.Requirements[0]
	if req.MemberAgentName != "billing" || req.MemberRunID == "" || len(rt.refunded) != 0 {
		t.Fatalf("bad member context: %+v refunded=%v", req, rt.refunded)
	}
	if err := req.Confirm(); err != nil {
		t.Fatal(err)
	}
	out, err := tm.ContinueRun(context.Background(), &resp)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != models.RunStatusCompleted || out.RunID != resp.RunID {
		t.Fatalf("expected completed team run: %s", out.Status)
	}
	if len(rt.refunded) != 1 || !strings.Contains(out.TextContent, "synthesis") {
		t.Fatalf("member tool should run and team should synthesize: %v %q", rt.refunded, out.TextContent)
	}
	if !strings.Contains(out.TextContent, "refunded 42") || !strings.Contains(out.TextContent, "support") {
		t.Fatalf("synthesis should include both member answers: %q", out.TextContent)
	}
}
