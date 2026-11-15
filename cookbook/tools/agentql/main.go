// Example: AgentQL Scraper agent using the agentql toolkit.
//
// Requires AGENTQL_API_KEY.
//
//	go run ./cookbook/tools/agentql
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/agentql"
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

	tools := agentql.New()

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "AgentQL Scraper",
		Model:         model,
		Instructions:  "Scrape websites with AgentQL and summarize the content.",
		Tools:         []toolkit.Tool{tools},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("Scrape https://docs.agno.com and list the main sections.", true, true)
}
