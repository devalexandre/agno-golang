// Human-in-the-loop: external execution.
//
// toolkit.WithExternalExecution() marks a method executed outside the agent
// (e.g. by the frontend or another service). The run pauses with the call's
// name and arguments; the caller runs it, hands the result back with
// SetExternalExecutionResult and ContinueRun feeds it to the model.
//
// Runs offline with a scripted stub model; no env vars or services needed.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/cookbook/internal/toolcallstub"
)

type ShellParams struct {
	Command string `json:"command" description:"Shell command" required:"true"`
}

type ShellTool struct{ toolkit.Toolkit }

// Run is never executed by the agent: the caller executes the command.
func (s *ShellTool) Run(p ShellParams) (interface{}, error) { return nil, nil }

func main() {
	tool := &ShellTool{Toolkit: toolkit.NewToolkit()}
	tool.Name, tool.Description = "Shell", "Shell access"
	tool.RegisterWithOptions("Run", "Run a shell command", tool, tool.Run, ShellParams{}, toolkit.WithExternalExecution())

	a, err := agent.NewAgent(agent.AgentConfig{
		Model: toolcallstub.New("Here is the listing.", "Shell_Run", `{"command":"ls /srv"}`),
		Tools: []toolkit.Tool{tool},
	})
	if err != nil {
		log.Fatal(err)
	}
	resp, err := a.Run("What is in /srv?")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("status=%s\n%s\n", resp.Status, resp.TextContent)
	for _, req := range resp.RequirementsNeedingExternalExecution() {
		fmt.Printf("executing %v externally\n", req.ToolExecution.ToolArgs["command"])
		_ = req.SetExternalExecutionResult("app\nbackups\nlogs")
	}
	out, err := a.ContinueRun(context.Background(), &resp)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("status=%s content=%q\n", out.Status, out.TextContent)
}
