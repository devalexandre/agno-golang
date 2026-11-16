// Example: Redmine Assistant with the redmine toolkit.
//
// Requires REDMINE_SERVER_URL and REDMINE_TOKEN (or REDMINE_USERNAME/REDMINE_PASSWORD).
//
//	go run ./cookbook/tools/redmine
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/redmine"
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

	tools := redmine.New()

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Redmine Assistant",
		Model:         model,
		Instructions:  "You manage Redmine issues. Resolve names to ids with the list tools before creating or updating issues.",
		Tools:         []toolkit.Tool{tools},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("List the projects, then show the open bugs of the first project.", true, true)
}
