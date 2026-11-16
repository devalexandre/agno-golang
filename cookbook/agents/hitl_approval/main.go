// Human-in-the-loop: approval records (required vs audit).
//
//   - toolkit.WithApproval(toolkit.ApprovalRequired): when the run pauses a
//     "pending" approval record is written to the approval store. ContinueRun
//     fails with agent.ErrApprovalPending until an approver resolves it with
//     approval.Resolve; the decision is then applied to the paused call.
//   - toolkit.WithApproval(toolkit.ApprovalAudit) (combined with a HITL flag):
//     an "approved"/"rejected" audit record is written after resolution.
//
// Runs offline with a scripted stub model and approval.NewInMemoryStore();
// no env vars or services needed.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/approval"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/cookbook/internal/toolcallstub"
)

type PayParams struct {
	Invoice string  `json:"invoice" description:"Invoice id" required:"true"`
	Amount  float64 `json:"amount" description:"Amount" required:"true"`
}

type PaymentsTool struct{ toolkit.Toolkit }

func (p *PaymentsTool) Pay(in PayParams) (interface{}, error) {
	return fmt.Sprintf("paid invoice %s (%.2f)", in.Invoice, in.Amount), nil
}

func (p *PaymentsTool) Refund(in PayParams) (interface{}, error) {
	return fmt.Sprintf("refunded invoice %s", in.Invoice), nil
}

func main() {
	ctx := context.Background()
	store := approval.NewInMemoryStore()

	tool := &PaymentsTool{Toolkit: toolkit.NewToolkit()}
	tool.Name, tool.Description = "Payments", "Payments"
	tool.RegisterWithOptions("Pay", "Pay an invoice", tool, tool.Pay, PayParams{}, toolkit.WithApproval(toolkit.ApprovalRequired))
	tool.RegisterWithOptions("Refund", "Refund an invoice", tool, tool.Refund, PayParams{}, toolkit.WithConfirmation(), toolkit.WithApproval(toolkit.ApprovalAudit))

	// Required approval.
	a, err := agent.NewAgent(agent.AgentConfig{
		Name:          "payments-agent",
		Model:         toolcallstub.New("Done.", "Payments_Pay", `{"invoice":"INV-7","amount":1200}`),
		Tools:         []toolkit.Tool{tool},
		ApprovalStore: store,
	})
	if err != nil {
		log.Fatal(err)
	}
	resp, err := a.Run("Pay invoice INV-7")
	if err != nil {
		log.Fatal(err)
	}
	approvalID := resp.Requirements[0].ToolExecution.ApprovalID
	rec, _ := store.GetApproval(ctx, approvalID)
	fmt.Printf("run %s, approval %s status=%s\n", resp.Status, rec.ID, rec.Status)

	if _, err := a.ContinueRun(ctx, &resp); errors.Is(err, agent.ErrApprovalPending) {
		fmt.Println("continue refused:", err)
	}
	if _, err := approval.Resolve(ctx, store, approvalID, approval.StatusApproved, "finance-lead", nil); err != nil {
		log.Fatal(err)
	}
	out, err := a.ContinueRun(ctx, &resp)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("status=%s content=%q\n\n", out.Status, out.TextContent)

	// Audit approval.
	b, err := agent.NewAgent(agent.AgentConfig{
		Name:          "refunds-agent",
		Model:         toolcallstub.New("Done.", "Payments_Refund", `{"invoice":"INV-8","amount":10}`),
		Tools:         []toolkit.Tool{tool},
		ApprovalStore: store,
	})
	if err != nil {
		log.Fatal(err)
	}
	resp, _ = b.Run("Refund INV-8")
	_ = resp.Requirements[0].Confirm()
	if _, err := b.ContinueRun(ctx, &resp); err != nil {
		log.Fatal(err)
	}
	audits, _ := store.ListApprovals(ctx, approval.Filter{ApprovalType: approval.TypeAudit})
	for _, r := range audits {
		fmt.Printf("audit record: tool=%s status=%s\n", r.ToolName, r.Status)
	}
}
