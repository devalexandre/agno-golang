// Example: query Salesforce CRM data with the Salesforce toolkit.
//
// Requires SALESFORCE_USERNAME, SALESFORCE_PASSWORD and
// SALESFORCE_SECURITY_TOKEN (optionally SALESFORCE_DOMAIN=test for
// sandboxes), or salesforce.WithInstanceURL + salesforce.WithSessionID.
//
//	go run ./cookbook/tools/salesforce
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/salesforce"
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

	// Read-only tools by default; use salesforce.WithAll() to allow writes.
	sf := salesforce.New()

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Salesforce Analyst",
		Model:         model,
		Instructions:  salesforce.Instructions,
		Tools:         []toolkit.Tool{sf},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("List the 5 most recently created Accounts with their industry.", true, true)
}
