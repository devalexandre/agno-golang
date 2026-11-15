// Example: let the agent ask the user for missing information with
// UserControlFlowTools.
//
// The model calls GetUserInput with the fields it needs. The run pauses (as
// Python's get_user_input does): the requirement's UserInputSchema lists the
// fields the model asked for, ProvideUserInput fills them and ContinueRun
// hands the values back to the model as the tool result.
//
// Runs offline with a scripted stub model; no env vars or services needed.
// Replace toolcallstub with a real model (e.g. ollama or openai) to let an
// LLM choose the fields.
//
//	go run ./cookbook/tools/usercontrolflow
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/tools/usercontrolflow"
	"github.com/devalexandre/agno-golang/cookbook/internal/toolcallstub"
)

func main() {
	ctx := context.Background()
	uc := usercontrolflow.NewUserControlFlowTools()

	// The stub plays the model: it asks for the recipient and the date.
	model := toolcallstub.New("Drafted the invitation.", "user_control_flow_tools_GetUserInput",
		`{"user_input_fields":[
			{"field_name":"recipient","field_type":"str","field_description":"Who should be invited"},
			{"field_name":"date","field_type":"str","field_description":"Meeting date"}]}`)

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Name:         "Email Drafter",
		Model:        model,
		Instructions: "You draft emails.\n" + uc.Instructions,
		Tools:        []toolkit.Tool{uc},
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// 1. Run: the model calls GetUserInput and the run pauses.
	resp, err := ag.Run("Draft an email inviting someone to a meeting.")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("status=%s\n%s\n", resp.Status, resp.TextContent)

	// 2. Answer: fill in the fields the model asked for (read them from the
	// user in a real app).
	answers := map[string]interface{}{"recipient": "ana@example.com", "date": "next Monday"}
	for _, req := range resp.RequirementsNeedingUserInput() {
		values := map[string]interface{}{}
		for _, f := range req.UserInputSchema() {
			fmt.Printf("field %s (%s): %s -> %v\n", f.Name, f.FieldType, f.Description, answers[f.Name])
			values[f.Name] = answers[f.Name]
		}
		if err := req.ProvideUserInput(values); err != nil {
			log.Fatal(err)
		}
	}

	// 3. Continue: the model receives the values and finishes the run.
	out, err := ag.ContinueRun(ctx, &resp)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("status=%s\n%s\n", out.Status, out.TextContent)
}
