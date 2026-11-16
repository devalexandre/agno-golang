// Team human-in-the-loop: a member's pause propagates to the team.
//
// When a member agent calls a tool needing confirmation, Team.Run returns
// Status=PAUSED with the member's requirements (tagged with MemberAgentName
// and MemberRunID). After resolving them, Team.ContinueRun continues the
// member's run, runs the remaining members and synthesizes as usual.
//
// Runs offline with scripted stub models; no env vars or services needed.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/team"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/cookbook/internal/toolcallstub"
)

type RefundParams struct {
	Order string `json:"order" description:"Order id" required:"true"`
}

type BillingTool struct{ toolkit.Toolkit }

func (b *BillingTool) Refund(p RefundParams) (interface{}, error) {
	return "refunded order " + p.Order, nil
}

func main() {
	tool := &BillingTool{Toolkit: toolkit.NewToolkit()}
	tool.Name, tool.Description = "Billing", "Billing"
	tool.RegisterWithOptions("Refund", "Refund an order", tool, tool.Refund, RefundParams{}, toolkit.WithConfirmation())

	billing, err := agent.NewAgent(agent.AgentConfig{
		Name:  "billing",
		Model: toolcallstub.New("Refund processed.", "Billing_Refund", `{"order":"42"}`),
		Tools: []toolkit.Tool{tool},
	})
	if err != nil {
		log.Fatal(err)
	}
	support, err := agent.NewAgent(agent.AgentConfig{Name: "support", Model: toolcallstub.New("Apology sent for:")})
	if err != nil {
		log.Fatal(err)
	}

	desk := team.NewTeam(team.TeamConfig{
		Context: context.Background(),
		Name:    "support-desk",
		Model:   toolcallstub.New("Team summary:"),
		Members: []*agent.Agent{billing, support},
		Mode:    team.CoordinateMode,
	})

	resp, err := desk.Run("Refund order 42 and apologise to the customer")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("team status=%s\n", resp.Status)
	for _, req := range resp.ActiveRequirements() {
		fmt.Printf("member %s wants %s %v -> confirming\n", req.MemberAgentName, req.ToolExecution.ToolName, req.ToolExecution.ToolArgs)
		_ = req.Confirm()
	}
	out, err := desk.ContinueRun(context.Background(), &resp)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("team status=%s\n%s\n", out.Status, out.TextContent)
}
