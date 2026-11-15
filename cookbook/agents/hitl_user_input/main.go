// Human-in-the-loop: user input.
//
// toolkit.WithUserInput("to") hides the "to" parameter from the model; when
// the model calls the method the run pauses and the requirement lists the
// fields the user must fill in (name, type, description). ProvideUserInput
// sets them and ContinueRun executes the call with the merged arguments.
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

type EmailParams struct {
	To      string `json:"to" description:"Recipient address" required:"true"`
	Subject string `json:"subject" description:"Subject" required:"true"`
}

type EmailTool struct{ toolkit.Toolkit }

func (e *EmailTool) Send(p EmailParams) (interface{}, error) {
	return fmt.Sprintf("email %q sent to %s", p.Subject, p.To), nil
}

func main() {
	tool := &EmailTool{Toolkit: toolkit.NewToolkit()}
	tool.Name, tool.Description = "Email", "Email tools"
	tool.RegisterWithOptions("Send", "Send an email", tool, tool.Send, EmailParams{}, toolkit.WithUserInput("to"))

	a, err := agent.NewAgent(agent.AgentConfig{
		Model: toolcallstub.New("Done.", "Email_Send", `{"subject":"Quarterly report"}`),
		Tools: []toolkit.Tool{tool},
	})
	if err != nil {
		log.Fatal(err)
	}

	resp, err := a.Run("Send the quarterly report")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("status=%s\n", resp.Status)
	for _, req := range resp.RequirementsNeedingUserInput() {
		values := map[string]interface{}{}
		for _, f := range req.UserInputSchema() {
			fmt.Printf("field %s (%s): %s\n", f.Name, f.FieldType, f.Description)
			values[f.Name] = "cfo@example.com" // read from the user in a real app
		}
		_ = req.ProvideUserInput(values)
	}
	out, err := a.ContinueRun(context.Background(), &resp)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("status=%s content=%q\n", out.Status, out.TextContent)
}
