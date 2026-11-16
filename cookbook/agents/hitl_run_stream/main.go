// Human-in-the-loop with streaming: RunStream pauses too.
//
// When the model calls a tool that needs confirmation, user input or
// external execution, the stream stops and:
//
//   - RunStream returns a *agent.RunPausedError (errors.Is(err, agent.ErrRunPaused));
//   - RunStreamWithResponse returns the PAUSED RunResponse and accepts run
//     options (WithSessionID, WithUserID, WithRunID, ...).
//
// No session is needed: the paused state is kept in memory (and in the session
// DB when the agent has one). Resolve the requirements and call ContinueRun.
//
// Runs offline with a scripted stub model; no env vars or services needed.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/cookbook/internal/toolcallstub"
)

type DeployParams struct {
	Service string `json:"service" description:"Service to deploy" required:"true"`
}

type DeployTool struct{ toolkit.Toolkit }

func (d *DeployTool) Deploy(p DeployParams) (interface{}, error) {
	return "deployed " + p.Service, nil
}

func main() {
	tool := &DeployTool{Toolkit: toolkit.NewToolkit()}
	tool.Name, tool.Description = "Ops", "Deployment operations"
	tool.RegisterWithOptions("Deploy", "Deploy a service", tool, tool.Deploy, DeployParams{}, toolkit.WithConfirmation())

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context: context.Background(),
		Name:    "deployer",
		Model:   toolcallstub.New("Deployment finished.", "Ops_Deploy", `{"service":"billing"}`),
		Tools:   []toolkit.Tool{tool},
	})
	if err != nil {
		log.Fatal(err)
	}

	print := func(chunk []byte) error { fmt.Print(string(chunk)); return nil }

	// 1. RunStream: the pause comes back as a typed error.
	err = ag.RunStream("deploy billing", print)
	var paused *agent.RunPausedError
	if !errors.As(err, &paused) {
		log.Fatalf("expected a paused run, got %v", err)
	}
	req := paused.Response.ActiveRequirements()[0]
	fmt.Printf("paused (errors.Is ErrRunPaused=%v): confirm %s(%v)?\n",
		errors.Is(err, agent.ErrRunPaused), req.ToolExecution.ToolName, req.ToolExecution.ToolArgs)

	// The user approves; the run resumes where it stopped.
	if err := req.Confirm(); err != nil {
		log.Fatal(err)
	}
	out, err := ag.ContinueRun(context.Background(), &paused.Response)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("continued: status=%s content=%q\n\n", out.Status, out.TextContent)

	// 2. RunStreamWithResponse: same flow, with run options and the response.
	resp, err := ag.RunStreamWithResponse("deploy billing", print, agent.WithRunID("deploy-42"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("run %s status=%s requirements=%d\n", resp.RunID, resp.Status, len(resp.ActiveRequirements()))
	_ = resp.ActiveRequirements()[0].Reject("not during business hours")
	out, err = ag.ContinueRun(context.Background(), &resp)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("rejected: status=%s content=%q\n", out.Status, out.TextContent)
}
