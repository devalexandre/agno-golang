// Example: structured questions to the user with UserFeedbackTools.
//
// The model calls AskUser with questions and predefined options. The run
// pauses (as Python's ask_user does): the requirement's UserFeedbackSchema
// holds the questions, ProvideUserFeedback records the selected labels and
// ContinueRun hands the selections back to the model as the tool result.
//
// Runs offline with a scripted stub model; no env vars or services needed.
// Replace toolcallstub with a real model (e.g. ollama or openai) to let an
// LLM write the questions.
//
//	go run ./cookbook/tools/userfeedback
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/tools/userfeedback"
	"github.com/devalexandre/agno-golang/cookbook/internal/toolcallstub"
)

func main() {
	ctx := context.Background()
	uf := userfeedback.NewUserFeedbackTools()

	// The stub plays the model: it asks where to go and which extras to book.
	model := toolcallstub.New("Planned the trip.", "user_feedback_tools_AskUser",
		`{"questions":[
			{"question":"Which city do you prefer?","header":"Destination","options":[
				{"label":"Lisbon","description":"Sunny, by the sea"},{"label":"Prague","description":"Historic old town"}]},
			{"question":"What should I book?","header":"Extras","multi_select":true,"options":[
				{"label":"Hotel"},{"label":"Car"},{"label":"Museum pass"}]}]}`)

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Name:         "Trip Planner",
		Model:        model,
		Instructions: "Help plan trips. Ask the user to choose before planning.\n" + uf.Instructions,
		Tools:        []toolkit.Tool{uf},
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// 1. Run: the model calls AskUser and the run pauses.
	resp, err := ag.Run("Plan a weekend trip for me in Europe.")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("status=%s\n", resp.Status)

	// 2. Answer: show the questions and record the choices (read them from
	// the user in a real app).
	choices := map[string][]string{
		"Which city do you prefer?": {"Lisbon"},
		"What should I book?":       {"Hotel", "Museum pass"},
	}
	for _, req := range resp.RequirementsNeedingUserFeedback() {
		for _, q := range req.UserFeedbackSchema() {
			fmt.Printf("\n[%s] %s (multi_select=%v)\n", q.Header, q.Question, q.MultiSelect)
			for i, o := range q.Options {
				fmt.Printf("  %d) %s %s\n", i+1, o.Label, o.Description)
			}
			fmt.Printf("  -> %v\n", choices[q.Question])
		}
		if err := req.ProvideUserFeedback(choices); err != nil {
			log.Fatal(err)
		}
	}

	// 3. Continue: the model receives the selections and finishes the run.
	out, err := ag.ContinueRun(ctx, &resp)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nstatus=%s\n%s\n", out.Status, out.TextContent)
}
