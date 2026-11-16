// Example: manage ClickUp tasks.
//
// Requires CLICKUP_API_KEY and MASTER_SPACE_ID (the workspace/team ID).
//
//	go run ./cookbook/tools/clickup
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/clickup"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	ctx := context.Background()

	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "ClickUp Assistant",
		Model:         model,
		Instructions:  "Use the ClickUp tools to list spaces, lists and tasks, and to create or update tasks.",
		Tools:         []toolkit.Tool{clickup.New()},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("List my spaces and the tasks in the first one.", true, true)
}
